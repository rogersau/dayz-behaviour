package observations

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

const (
	BuilderVersion      = "observation-builder-v2"
	IndependenceVersion = "episode-window-v1"
)

type Config struct {
	DecisionWindowMS    int64
	RefractoryMS        int64
	EpisodeGapMS        int64
	EncounterGapMS      int64
	MaxQueueDelayMS     int64
	TimingPolicyVersion string
}

func DefaultConfig() Config {
	return Config{
		DecisionWindowMS:    5_000,
		RefractoryMS:        5_000,
		EpisodeGapMS:        30_000,
		EncounterGapMS:      120_000,
		MaxQueueDelayMS:     250,
		TimingPolicyVersion: "server-receive-window-v1",
	}
}

type Observation struct {
	ObservationID              string    `json:"observation_id"`
	OpportunityID              string    `json:"opportunity_id"`
	DecisionWindowID           string    `json:"decision_window_id"`
	RefractoryWindowID         string    `json:"refractory_window_id"`
	EncounterID                string    `json:"encounter_id"`
	ObserverTargetEpisodeID    string    `json:"observer_target_episode_id"`
	ServerSessionID            string    `json:"server_session_id"`
	ServerID                   string    `json:"server_id"`
	MapID                      string    `json:"map_id"`
	AreaCell                   string    `json:"area_cell"`
	ObserverPlayerSessionID    string    `json:"observer_player_session_id"`
	TargetPlayerSessionID      string    `json:"target_player_session_id"`
	TargetIdentityKey          string    `json:"-"`
	StartedMS                  int64     `json:"started_ms"`
	ClosedMS                   int64     `json:"closed_ms"`
	SamplingStream             string    `json:"sampling_stream"`
	SamplingPolicyVersion      string    `json:"sampling_policy_version"`
	VisibilityClass            string    `json:"visibility_class"`
	VisibilityAuthority        string    `json:"visibility_authority"`
	ObserverOriginMode         string    `json:"observer_origin_mode"`
	OutcomeType                string    `json:"outcome_type"`
	OutcomeObserved            bool      `json:"outcome_observed"`
	OutcomeObservedMS          int64     `json:"outcome_observed_ms"`
	OutcomeAuthority           string    `json:"outcome_authority,omitempty"`
	CueClass                   string    `json:"cue_class"`
	Independent                bool      `json:"independent"`
	StrongHiddenEligible       bool      `json:"strong_hidden_eligible"`
	ControlEligible            bool      `json:"control_eligible"`
	TimingEligible             bool      `json:"timing_eligible"`
	TimingPolicyVersion        string    `json:"timing_policy_version"`
	OcclusionDurationMS        int64     `json:"occlusion_duration_ms"`
	VisibilityValidationID     string    `json:"visibility_validation_id"`
	FirstExposureMS            int64     `json:"first_exposure_ms"`
	ExposureCause              string    `json:"exposure_cause"`
	ExposureWindowCensored     bool      `json:"exposure_window_censored"`
	TargetInclusionProbability float64   `json:"target_inclusion_probability"`
	QueueAdmissionProbability  float64   `json:"queue_admission_probability"`
	QueueDelayMS               int64     `json:"queue_delay_ms"`
	DistanceBand               string    `json:"distance_band"`
	ObserverMovementBand       string    `json:"observer_movement_band"`
	ObserverStanceID           int       `json:"observer_stance_id"`
	BaselineWeaponState        string    `json:"baseline_weapon_state"`
	CameraMode                 string    `json:"camera_mode"`
	ServerPopulationBand       string    `json:"server_population_band"`
	SourceEventIDs             []string  `json:"source_event_ids"`
	CueFacts                   []CueFact `json:"cue_facts"`
}

type CueFact struct {
	CueType         string `json:"cue_type"`
	SourceEventID   string `json:"source_event_id"`
	OccurredMS      int64  `json:"occurred_ms"`
	Details         string `json:"details"`
	SourceAuthority string `json:"source_authority"`
}

type wirePayload struct {
	TargetPlayerID             string  `json:"target_player_id"`
	TargetPlayerSessionID      string  `json:"target_player_session_id"`
	Classification             string  `json:"classification"`
	ObserverOriginMode         string  `json:"observer_origin_mode"`
	SamplingStream             string  `json:"sampling_stream"`
	SamplingPolicyVersion      string  `json:"sampling_policy_version"`
	SamplingReason             string  `json:"sampling_reason"`
	TargetInclusionProbability float64 `json:"target_inclusion_probability"`
	QueueAdmissionProbability  float64 `json:"queue_admission_probability"`
	QueueDelayMS               int64   `json:"queue_delay_ms"`
	ProbeStartedMS             int64   `json:"probe_started_ms"`
	SourcePlayerID             string  `json:"source_player_id"`
	MapID                      string  `json:"map_id"`
	AreaCell                   string  `json:"area_cell"`
	TargetDistanceMetres       float64 `json:"target_distance_metres"`
	ObserverSpeedMPS           float64 `json:"observer_speed_mps"`
	ObserverStanceID           int     `json:"observer_stance_id"`
	BaselineWeaponState        string  `json:"baseline_weapon_state"`
	CameraMode                 string  `json:"camera_mode"`
	ServerPopulationCount      int     `json:"server_population_count"`
	OcclusionDurationMS        int64   `json:"occlusion_duration_ms"`
	VisibilityValidationID     string  `json:"visibility_validation_id"`
}

type flatEvent struct {
	batch schema.Batch
	event schema.Event
	id    string
	data  wirePayload
}

func Build(batches []schema.Batch, config Config) ([]Observation, error) {
	if config.DecisionWindowMS <= 0 || config.RefractoryMS <= 0 || config.EpisodeGapMS <= 0 || config.EncounterGapMS <= 0 || config.MaxQueueDelayMS <= 0 || config.TimingPolicyVersion == "" {
		return nil, fmt.Errorf("observation durations must be positive")
	}
	events, err := flatten(batches)
	if err != nil {
		return nil, err
	}

	var result []Observation
	for index, current := range events {
		if current.event.EventType != "VISIBILITY_OBSERVATION" || current.data.SamplingStream != "random_opportunity" {
			continue
		}
		started := current.data.ProbeStartedMS
		if started <= 0 {
			started = current.event.ServerTimeMS
		}
		targetSession := current.data.TargetPlayerSessionID
		if targetSession == "" {
			targetSession = current.batch.ServerSessionID + ":" + current.data.TargetPlayerID
		}
		cueClassification, cueFact := cueClass(events, index, current, current.event.PlayerSessionID, current.data.TargetPlayerID, started)
		observation := Observation{
			OpportunityID:              current.id,
			ServerSessionID:            current.batch.ServerSessionID,
			ServerID:                   current.batch.ServerID,
			MapID:                      current.data.MapID,
			AreaCell:                   current.data.AreaCell,
			ObserverPlayerSessionID:    current.event.PlayerSessionID,
			TargetPlayerSessionID:      targetSession,
			TargetIdentityKey:          targetIdentityKey(current.data.TargetPlayerID, targetSession),
			StartedMS:                  started,
			ClosedMS:                   started + config.DecisionWindowMS,
			SamplingStream:             current.data.SamplingStream,
			SamplingPolicyVersion:      current.data.SamplingPolicyVersion,
			VisibilityClass:            current.data.Classification,
			VisibilityAuthority:        string(current.event.SourceAuthority),
			ObserverOriginMode:         current.data.ObserverOriginMode,
			OutcomeType:                "READINESS",
			CueClass:                   cueClassification,
			Independent:                true,
			TargetInclusionProbability: current.data.TargetInclusionProbability,
			QueueAdmissionProbability:  current.data.QueueAdmissionProbability,
			QueueDelayMS:               current.data.QueueDelayMS,
			DistanceBand:               distanceBand(current.data.TargetDistanceMetres),
			ObserverMovementBand:       movementBand(current.data.ObserverSpeedMPS),
			ObserverStanceID:           current.data.ObserverStanceID,
			BaselineWeaponState:        current.data.BaselineWeaponState,
			CameraMode:                 current.data.CameraMode,
			ServerPopulationBand:       populationBand(current.data.ServerPopulationCount),
			OcclusionDurationMS:        current.data.OcclusionDurationMS,
			VisibilityValidationID:     current.data.VisibilityValidationID,
			SourceEventIDs:             []string{current.id},
		}
		if cueFact != nil {
			observation.CueFacts = append(observation.CueFacts, *cueFact)
			observation.SourceEventIDs = append(observation.SourceEventIDs, cueFact.SourceEventID)
		}
		observation.OutcomeObserved, observation.OutcomeObservedMS, observation.OutcomeAuthority, observation.SourceEventIDs = readinessOutcome(events, index+1, current, observation, observation.SourceEventIDs)
		observation.FirstExposureMS, observation.ExposureCause, observation.ExposureWindowCensored, observation.SourceEventIDs = firstExposure(events, index+1, current, observation, current.data.TargetPlayerID, observation.SourceEventIDs)
		observation.TimingEligible = observation.QueueDelayMS >= 0 && observation.QueueDelayMS <= config.MaxQueueDelayMS
		observation.TimingPolicyVersion = fmt.Sprintf("%s:max-queue-%dms", config.TimingPolicyVersion, config.MaxQueueDelayMS)
		observation.StrongHiddenEligible = observation.VisibilityClass == "ROBUSTLY_OCCLUDED" &&
			observation.VisibilityAuthority == "A" &&
			validatedFirstPersonOrigin(observation.ObserverOriginMode) &&
			observation.VisibilityValidationID != "" && observation.OcclusionDurationMS > 0 &&
			observation.TimingEligible &&
			observation.TargetInclusionProbability > 0 &&
			observation.QueueAdmissionProbability > 0
		observation.ControlEligible = (observation.VisibilityClass == "EXPOSED" || observation.VisibilityClass == "PARTIALLY_EXPOSED") &&
			observation.VisibilityAuthority == "A" &&
			observation.TimingEligible &&
			observation.TargetInclusionProbability > 0 &&
			observation.QueueAdmissionProbability > 0
		observation.DecisionWindowID = stableID("window", current.id)
		observation.RefractoryWindowID = stableID("refractory", observation.ObserverPlayerSessionID, fmt.Sprint(started/config.RefractoryMS))
		observation.ObservationID = stableID("observation", current.id, BuilderVersion)
		result = append(result, observation)
	}
	assignIndependence(result, config)
	return result, nil
}

func firstExposure(events []flatEvent, startIndex int, origin flatEvent, observation Observation, targetID string, sourceIDs []string) (int64, string, bool, []string) {
	const exposureHorizonMS = int64(30_000)
	for _, candidate := range events[startIndex:] {
		if !sameServerSession(origin, candidate) {
			break
		}
		if candidate.event.ServerTimeMS > observation.StartedMS+exposureHorizonMS {
			break
		}
		if candidate.event.PlayerSessionID != observation.ObserverPlayerSessionID || candidate.event.EventType != "VISIBILITY_OBSERVATION" || candidate.data.TargetPlayerID != targetID {
			continue
		}
		if candidate.data.Classification == "EXPOSED" || candidate.data.Classification == "PARTIALLY_EXPOSED" {
			return candidate.event.ServerTimeMS, "UNKNOWN", false, append(sourceIDs, candidate.id)
		}
	}
	return 0, "UNKNOWN", true, sourceIDs
}

func distanceBand(value float64) string {
	switch {
	case value < 25:
		return "0-25"
	case value < 50:
		return "25-50"
	case value < 100:
		return "50-100"
	case value < 250:
		return "100-250"
	default:
		return "250+"
	}
}

func movementBand(value float64) string {
	switch {
	case value < 0.25:
		return "STATIONARY"
	case value < 2.0:
		return "SLOW"
	default:
		return "FAST"
	}
}

func populationBand(value int) string {
	switch {
	case value < 10:
		return "0-9"
	case value < 30:
		return "10-29"
	case value < 60:
		return "30-59"
	default:
		return "60+"
	}
}

func flatten(batches []schema.Batch) ([]flatEvent, error) {
	var events []flatEvent
	for _, batch := range batches {
		for _, event := range batch.Events {
			var payload wirePayload
			if len(event.Payload) > 0 {
				if err := json.Unmarshal(event.Payload, &payload); err != nil {
					return nil, fmt.Errorf("decode payload for %s/%d: %w", batch.ServerSessionID, event.ServerSequence, err)
				}
			}
			id := event.SourceEventID
			if id == "" {
				id = fmt.Sprintf("%s:%d", batch.ServerSessionID, event.ServerSequence)
			}
			events = append(events, flatEvent{batch: batch, event: event, id: id, data: payload})
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].batch.ServerID != events[j].batch.ServerID {
			return events[i].batch.ServerID < events[j].batch.ServerID
		}
		if events[i].batch.ServerSessionID != events[j].batch.ServerSessionID {
			return events[i].batch.ServerSessionID < events[j].batch.ServerSessionID
		}
		if events[i].event.ServerTimeMS == events[j].event.ServerTimeMS {
			return events[i].event.ServerSequence < events[j].event.ServerSequence
		}
		return events[i].event.ServerTimeMS < events[j].event.ServerTimeMS
	})
	return events, nil
}

func readinessOutcome(events []flatEvent, startIndex int, origin flatEvent, observation Observation, sourceIDs []string) (bool, int64, string, []string) {
	for _, candidate := range events[startIndex:] {
		if !sameServerSession(origin, candidate) {
			break
		}
		if candidate.event.ServerTimeMS > observation.ClosedMS {
			break
		}
		if candidate.event.PlayerSessionID != observation.ObserverPlayerSessionID || candidate.event.EventType != "DECISION_EDGE" {
			continue
		}
		switch candidate.data.SamplingReason {
		case "WEAPON_RAISED", "ADS_ENTERED", "OPTICS_ENTERED":
			return true, candidate.event.ServerTimeMS, string(candidate.event.SourceAuthority), append(sourceIDs, candidate.id)
		}
	}
	return false, 0, "", sourceIDs
}

func cueClass(events []flatEvent, currentIndex int, origin flatEvent, observerSessionID, targetID string, atMS int64) (string, *CueFact) {
	const cueLookbackMS = int64(30_000)
	for index := currentIndex - 1; index >= 0; index-- {
		event := events[index]
		if !sameServerSession(origin, event) {
			break
		}
		if atMS-event.event.ServerTimeMS > cueLookbackMS {
			break
		}
		if event.event.PlayerSessionID != observerSessionID {
			continue
		}
		if event.event.EventType == "VISIBILITY_OBSERVATION" && event.data.TargetPlayerID == targetID &&
			(event.data.Classification == "EXPOSED" || event.data.Classification == "PARTIALLY_EXPOSED") {
			return "KNOWN", &CueFact{CueType: "RECENT_VISUAL_EXPOSURE", SourceEventID: event.id, OccurredMS: event.event.ServerTimeMS, Details: "captured exposed or partially exposed visibility observation", SourceAuthority: string(event.event.SourceAuthority)}
		}
		if (event.event.EventType == "PLAYER_HIT" || event.event.EventType == "PLAYER_KILLED") && event.data.SourcePlayerID == targetID {
			return "PLAUSIBLE", &CueFact{CueType: "RECENT_COMBAT_CONTACT", SourceEventID: event.id, OccurredMS: event.event.ServerTimeMS, Details: "captured hit or kill attributed to the target", SourceAuthority: string(event.event.SourceAuthority)}
		}
	}
	return "UNEXPLAINED_IN_CAPTURED_DATA", nil
}

func assignIndependence(observations []Observation, config Config) {
	lastFeatureByObserver := map[string]int64{}
	type groupState struct {
		lastMS         int64
		episodeIndex   int
		encounterIndex int
	}
	groups := map[string]groupState{}
	for index := range observations {
		observation := &observations[index]
		pair := observation.ObserverPlayerSessionID + "|" + observation.TargetPlayerSessionID
		state := groups[pair]
		if state.lastMS == 0 || observation.StartedMS-state.lastMS > config.EpisodeGapMS {
			state.episodeIndex++
		}
		if state.lastMS == 0 || observation.StartedMS-state.lastMS > config.EncounterGapMS {
			state.encounterIndex++
		}
		state.lastMS = observation.StartedMS
		groups[pair] = state
		observation.ObserverTargetEpisodeID = stableID("episode", pair, fmt.Sprint(state.episodeIndex), IndependenceVersion)
		observation.EncounterID = stableID("encounter", pair, fmt.Sprint(state.encounterIndex), IndependenceVersion)

		if previous, ok := lastFeatureByObserver[observation.ObserverPlayerSessionID]; ok && observation.StartedMS-previous < config.RefractoryMS {
			observation.Independent = false
			observation.StrongHiddenEligible = false
			observation.ControlEligible = false
		} else {
			lastFeatureByObserver[observation.ObserverPlayerSessionID] = observation.StartedMS
		}
	}
}

func sameServerSession(left, right flatEvent) bool {
	return left.batch.ServerID == right.batch.ServerID && left.batch.ServerSessionID == right.batch.ServerSessionID
}

func validatedFirstPersonOrigin(value string) bool {
	return value == "VALIDATED_FIRST_PERSON_HEAD" || value == "FIRST_PERSON_EYE"
}

func targetIdentityKey(targetID, targetSessionID string) string {
	if targetID != "" {
		return stableID("target-identity", targetID)
	}
	return stableID("target-session", targetSessionID)
}

func stableID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:16])
}
