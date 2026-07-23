package cues

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/rogersau/dayz-behaviour/internal/audio"
	"github.com/rogersau/dayz-behaviour/internal/observations"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

const LedgerVersion = "audio-cue-ledger-v1"

type Config struct {
	GunshotLookbackMS int64
	FootstepLookbackMS int64
	SnapshotToleranceMS int64
}

func DefaultConfig() Config {
	return Config{GunshotLookbackMS: 30_000, FootstepLookbackMS: 5_000, SnapshotToleranceMS: 2_000}
}

type payload struct {
	SourcePlayerID string    `json:"source_player_id"`
	WeaponType     string    `json:"weapon_type"`
	Ammo           string    `json:"ammo"`
	IsSuppressed   bool      `json:"is_suppressed"`
	SuppressorType string    `json:"suppressor_type"`
	Position       []float64 `json:"position"`
	MovementSpeed  float64   `json:"movement_speed_mps"`
	MovementState  string    `json:"movement_state"`
	Stance         string    `json:"stance_name"`
	SurfaceType    string    `json:"surface_type"`
	FootwearType   string    `json:"footwear_type"`
}

type event struct {
	serverID        string
	serverSessionID string
	playerSessionID string
	eventType       string
	eventID         string
	timeMS          int64
	authority       string
	data            payload
}

type positionSample struct {
	timeMS   int64
	position [3]float64
}

type candidateCue struct {
	fact  observations.CueFact
	class string
	rank  int
}

func Enrich(items []observations.Observation, batches []schema.Batch, config Config) error {
	if config.GunshotLookbackMS <= 0 || config.FootstepLookbackMS <= 0 || config.SnapshotToleranceMS <= 0 {
		return fmt.Errorf("audio cue lookback and snapshot tolerance must be positive")
	}
	events, snapshots, err := indexEvents(batches)
	if err != nil {
		return err
	}

	for index := range items {
		observation := &items[index]
		targetID := durableID(observation.TargetPlayerSessionID)
		if targetID == "" {
			continue
		}
		key := sessionKey(observation.ServerID, observation.ServerSessionID)
		var bestGunshot, bestFootstep *candidateCue
		for _, current := range events[key] {
			if current.timeMS > observation.StartedMS {
				break
			}
			ageMS := observation.StartedMS - current.timeMS
			switch current.eventType {
			case "SHOT_FIRED_SERVER":
				if ageMS > config.GunshotLookbackMS || current.data.SourcePlayerID != targetID {
					continue
				}
				cue, ok := gunshotCue(current, observation, snapshots, config)
				if ok && betterCue(cue, bestGunshot) {
					copy := cue
					bestGunshot = &copy
				}
			case "MOVEMENT_AUDIO_OPPORTUNITY":
				if ageMS > config.FootstepLookbackMS || (current.playerSessionID != observation.TargetPlayerSessionID && current.data.SourcePlayerID != targetID) {
					continue
				}
				cue, ok := footstepCue(current, observation, snapshots, config)
				if ok && betterCue(cue, bestFootstep) {
					copy := cue
					bestFootstep = &copy
				}
			}
		}
		for _, selected := range []*candidateCue{bestGunshot, bestFootstep} {
			if selected == nil {
				continue
			}
			observation.CueFacts = append(observation.CueFacts, selected.fact)
			observation.SourceEventIDs = appendUnique(observation.SourceEventIDs, selected.fact.SourceEventID)
			if cueClassRank(selected.class) > cueClassRank(observation.CueClass) {
				observation.CueClass = selected.class
			}
		}
	}
	return nil
}

func indexEvents(batches []schema.Batch) (map[string][]event, map[string][]positionSample, error) {
	events := map[string][]event{}
	snapshots := map[string][]positionSample{}
	for _, batch := range batches {
		key := sessionKey(batch.ServerID, batch.ServerSessionID)
		for _, source := range batch.Events {
			var data payload
			if len(source.Payload) > 0 {
				if err := json.Unmarshal(source.Payload, &data); err != nil {
					return nil, nil, fmt.Errorf("decode cue payload %s/%d: %w", batch.ServerSessionID, source.ServerSequence, err)
				}
			}
			id := source.SourceEventID
			if id == "" {
				id = fmt.Sprintf("%s:%d", batch.ServerSessionID, source.ServerSequence)
			}
			events[key] = append(events[key], event{
				serverID: batch.ServerID, serverSessionID: batch.ServerSessionID,
				playerSessionID: source.PlayerSessionID, eventType: source.EventType,
				eventID: id, timeMS: source.ServerTimeMS, authority: string(source.SourceAuthority), data: data,
			})
			if source.EventType == "PLAYER_SNAPSHOT" {
				if position, ok := vector3(data.Position); ok {
					snapshotKey := playerKey(batch.ServerID, batch.ServerSessionID, source.PlayerSessionID)
					snapshots[snapshotKey] = append(snapshots[snapshotKey], positionSample{timeMS: source.ServerTimeMS, position: position})
				}
			}
		}
	}
	for key := range events {
		sort.SliceStable(events[key], func(i, j int) bool { return events[key][i].timeMS < events[key][j].timeMS })
	}
	for key := range snapshots {
		sort.SliceStable(snapshots[key], func(i, j int) bool { return snapshots[key][i].timeMS < snapshots[key][j].timeMS })
	}
	return events, snapshots, nil
}

func gunshotCue(source event, observation *observations.Observation, snapshots map[string][]positionSample, config Config) (candidateCue, bool) {
	observerPosition, ok := nearestPosition(snapshots[playerKey(observation.ServerID, observation.ServerSessionID, observation.ObserverPlayerSessionID)], source.timeMS, config.SnapshotToleranceMS)
	if !ok {
		return candidateCue{}, false
	}
	sourcePosition, ok := vector3(source.data.Position)
	if !ok {
		return candidateCue{}, false
	}
	distance := distance3(observerPosition, sourcePosition)
	result := audio.ClassifyGunshot(audio.GunshotInput{DistanceM: distance, Suppressed: source.data.IsSuppressed})
	if result.Audibility == audio.NotAudible {
		return candidateCue{}, false
	}
	class, rank := cueClass(result.Audibility, true)
	details := cueDetails(map[string]any{
		"ledger_version": LedgerVersion, "audio_model_version": result.ModelVersion,
		"audibility": result.Audibility, "distance_metres": round1(distance),
		"direction_degrees": round1(directionDegrees(observerPosition, sourcePosition)),
		"weapon_type": source.data.WeaponType, "ammo": source.data.Ammo,
		"suppressed": source.data.IsSuppressed, "suppressor_type": source.data.SuppressorType,
		"likely_range_metres": result.LikelyRangeM, "maximum_range_metres": result.MaximumRangeM,
	})
	return candidateCue{rank: rank, class: class, fact: observations.CueFact{
		CueType: "GUNSHOT_AUDIO_OPPORTUNITY", SourceEventID: source.eventID,
		OccurredMS: source.timeMS, Details: details, SourceAuthority: authorityOrServer(source.authority),
	}}, true
}

func footstepCue(source event, observation *observations.Observation, snapshots map[string][]positionSample, config Config) (candidateCue, bool) {
	observerPosition, ok := nearestPosition(snapshots[playerKey(observation.ServerID, observation.ServerSessionID, observation.ObserverPlayerSessionID)], source.timeMS, config.SnapshotToleranceMS)
	if !ok {
		return candidateCue{}, false
	}
	sourcePosition, ok := vector3(source.data.Position)
	if !ok {
		return candidateCue{}, false
	}
	distance := distance3(observerPosition, sourcePosition)
	result := audio.ClassifyFootstep(audio.FootstepInput{
		DistanceM: distance, SpeedMPS: source.data.MovementSpeed, Stance: source.data.Stance,
		SurfaceType: source.data.SurfaceType, Footwear: source.data.FootwearType,
	})
	if result.Audibility == audio.NotAudible {
		return candidateCue{}, false
	}
	class, rank := cueClass(result.Audibility, false)
	details := cueDetails(map[string]any{
		"ledger_version": LedgerVersion, "audio_model_version": result.ModelVersion,
		"audibility": result.Audibility, "distance_metres": round1(distance),
		"direction_degrees": round1(directionDegrees(observerPosition, sourcePosition)),
		"speed_metres_per_second": round1(source.data.MovementSpeed),
		"gait": source.data.MovementState, "stance": source.data.Stance,
		"surface_type": source.data.SurfaceType, "footwear_type": source.data.FootwearType,
		"likely_range_metres": round1(result.LikelyRangeM), "maximum_range_metres": round1(result.MaximumRangeM),
		"estimated_not_raw_audio": true,
	})
	return candidateCue{rank: rank, class: class, fact: observations.CueFact{
		CueType: "FOOTSTEP_AUDIO_OPPORTUNITY", SourceEventID: source.eventID,
		OccurredMS: source.timeMS, Details: details, SourceAuthority: authorityOrServer(source.authority),
	}}, true
}

func cueClass(value audio.Audibility, gunshot bool) (string, int) {
	switch value {
	case audio.Strong:
		if gunshot {
			return "KNOWN", 3
		}
		return "PLAUSIBLE", 2
	case audio.Likely:
		return "PLAUSIBLE", 2
	case audio.Possible:
		// Possible cues are preserved for reviewers but do not suppress the primary analysis.
		return "UNEXPLAINED_IN_CAPTURED_DATA", 1
	default:
		return "UNEXPLAINED_IN_CAPTURED_DATA", 0
	}
}

func betterCue(value candidateCue, current *candidateCue) bool {
	return current == nil || value.rank > current.rank || (value.rank == current.rank && value.fact.OccurredMS > current.fact.OccurredMS)
}

func nearestPosition(samples []positionSample, atMS, toleranceMS int64) ([3]float64, bool) {
	var best [3]float64
	bestDelta := toleranceMS + 1
	for _, sample := range samples {
		delta := sample.timeMS - atMS
		if delta < 0 {
			delta = -delta
		}
		if delta <= toleranceMS && delta < bestDelta {
			best, bestDelta = sample.position, delta
		}
	}
	return best, bestDelta <= toleranceMS
}

func vector3(value []float64) ([3]float64, bool) {
	if len(value) != 3 {
		return [3]float64{}, false
	}
	return [3]float64{value[0], value[1], value[2]}, true
}

func distance3(left, right [3]float64) float64 {
	dx, dy, dz := right[0]-left[0], right[1]-left[1], right[2]-left[2]
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func directionDegrees(observer, source [3]float64) float64 {
	degrees := math.Atan2(source[0]-observer[0], source[2]-observer[2]) * 180 / math.Pi
	if degrees < 0 {
		degrees += 360
	}
	return degrees
}

func cueDetails(value map[string]any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func durableID(sessionID string) string {
	if index := strings.LastIndex(sessionID, ":"); index >= 0 {
		return sessionID[index+1:]
	}
	return sessionID
}

func sessionKey(serverID, serverSessionID string) string { return serverID + "\x00" + serverSessionID }
func playerKey(serverID, serverSessionID, playerSessionID string) string {
	return sessionKey(serverID, serverSessionID) + "\x00" + playerSessionID
}

func cueClassRank(value string) int {
	switch value {
	case "KNOWN":
		return 3
	case "PLAUSIBLE":
		return 2
	default:
		return 0
	}
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func authorityOrServer(value string) string {
	if value == "" {
		return "A"
	}
	return value
}

func round1(value float64) float64 { return math.Round(value*10) / 10 }
