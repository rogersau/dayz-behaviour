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
	BuilderVersion      = "observation-builder-v4-temporal"
	IndependenceVersion = "episode-window-v1"
)

type Config struct {
	DecisionWindowMS      int64
	RefractoryMS          int64
	EpisodeGapMS          int64
	EncounterGapMS        int64
	MaxQueueDelayMS       int64
	MaxEventUncertaintyMS int64
	TimingPolicyVersion   string
}

func DefaultConfig() Config {
	return Config{
		DecisionWindowMS:      5_000,
		RefractoryMS:          5_000,
		EpisodeGapMS:          30_000,
		EncounterGapMS:        120_000,
		MaxQueueDelayMS:       250,
		MaxEventUncertaintyMS: 500,
		TimingPolicyVersion:   "interval-timing-v1",
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
	OutcomeLowerMS             int64     `json:"outcome_lower_ms"`
	OutcomeUpperMS             int64     `json:"outcome_upper_ms"`
	OutcomeUncertaintyMS       int64     `json:"outcome_uncertainty_ms"`
	OutcomeTimingSource        string    `json:"outcome_timing_source"`
	OutcomeAuthority           string    `json:"outcome_authority,omitempty"`
	CueClass                   string    `json:"cue_class"`
	Independent                bool      `json:"independent"`
	StrongHiddenEligible       bool      `json:"strong_hidden_eligible"`
	ControlEligible            bool      `json:"control_eligible"`
	PositiveControlEligible    bool      `json:"positive_control_eligible"`
	ControlKind                string    `json:"control_kind"`
	TimingEligible             bool      `json:"timing_eligible"`
	TimingPolicyVersion        string    `json:"timing_policy_version"`
	OcclusionDurationMS        int64     `json:"occlusion_duration_ms"`
	VisibilityValidationID     string    `json:"visibility_validation_id"`
	FirstExposureMS            int64     `json:"first_exposure_ms"`
	FirstExposureLowerMS       int64     `json:"first_exposure_lower_ms"`
	FirstExposureUpperMS       int64     `json:"first_exposure_upper_ms"`
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
	ProbeCompletedMS           int64   `json:"probe_completed_ms"`
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
	ControlType                string  `json:"control_type"`
	EventTimeLowerMS           int64   `json:"event_time_lower_ms"`
	EventTimeUpperMS           int64   `json:"event_time_upper_ms"`
	EventTimeEstimateMS        int64   `json:"event_time_estimate_ms"`
	EventTimeUncertaintyMS     int64   `json:"event_time_uncertainty_ms"`
	EventTimingSource          string  `json:"event_timing_source"`
}

type flatEvent struct {
	batch schema.Batch
	event schema.Event
	id    string
	data  wirePayload
}

type outcomeResult struct {
	observed    bool
	estimateMS  int64
	lowerMS     int64
	upperMS     int64
	uncertainty int64
	timing      string
	authority   string
	sourceIDs   []string
}

type exposureResult struct {
	estimateMS int64
	lowerMS    int64
	upperMS    int64
	cause      string
	censored   bool
	sourceIDs  []string
}

func Build(batches []schema.Batch, config Config) ([]Observation, error) {
	if config.DecisionWindowMS <= 0 || config.RefractoryMS <= 0 || config.EpisodeGapMS <= 0 || config.EncounterGapMS <= 0 || config.MaxQueueDelayMS <= 0 || config.TimingPolicyVersion == "" {
		return nil, fmt.Errorf("observation durations must be positive")
	}
	if config.MaxEventUncertaintyMS <= 0 {
		config.MaxEventUncertaintyMS = 500
	}
	events, err := flatten(batches)
	if err != nil {
		return nil, err
	}

	var result []Observation
	for index, current := range events {
		if !isRandomOpportunityEvent(current) {
			continue
		}
		started := current.data.ProbeStartedMS
		if started <= 0 {
			started = current.event.ServerTimeMS
		}
		targetSession := current.data.TargetPlayerSessionID
		if targetSession == "" && current.data.TargetPlayerID != "" {
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
			ControlKind:                controlKind(current.data.Classification, current.data.ControlType),
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
		outcome := readinessOutcome(events, index+1, current, observation, observation.SourceEventIDs)
		observation.OutcomeObserved = outcome.observed
		observation.OutcomeObservedMS = outcome.estimateMS
		observation.OutcomeLowerMS = outcome.lowerMS
		observation.OutcomeUpperMS = outcome.upperMS
		observation.OutcomeUncertaintyMS = outcome.uncertainty
		observation.OutcomeTimingSource = outcome.timing
		observation.OutcomeAuthority = outcome.authority
		observation.SourceEventIDs = outcome.sourceIDs
		if current.data.TargetPlayerID != "" {
			exposure := firstExposure(events, index+1, current, observation, current.data.TargetPlayerID, observation.SourceEventIDs)
			observation.FirstExposureMS = exposure.estimateMS
			observation.FirstExposureLowerMS = exposure.lowerMS
			observation.FirstExposureUpperMS = exposure.upperMS
			observation.ExposureCause = exposure.cause
			observation.ExposureWindowCensored = exposure.censored
			observation.SourceEventIDs = exposure.sourceIDs
		} else {
			observation.ExposureCause = "NOT_APPLICABLE"
			observation.ExposureWindowCensored = true
		}
		observation.TimingEligible = observation.QueueDelayMS >= 0 && observation.QueueDelayMS <= config.MaxQueueDelayMS
		if observation.OutcomeObserved && observation.OutcomeUncertaintyMS > config.MaxEventUncertaintyMS {
			observation.TimingEligible = false
		}
		observation.TimingPolicyVersion = fmt.Sprintf("%s:max-queue-%dms:max-event-uncertainty-%dms", config.TimingPolicyVersion, config.MaxQueueDelayMS, config.MaxEventUncertaintyMS)
		observation.StrongHiddenEligible = observation.VisibilityClass == "ROBUSTLY_OCCLUDED" &&
			observation.VisibilityAuthority == "A" &&
			validatedFirstPersonOrigin(observation.ObserverOriginMode) &&
			observation.VisibilityValidationID != "" && observation.OcclusionDurationMS > 0 &&
			observation.TimingEligible &&
			observation.TargetInclusionProbability > 0 &&
			observation.QueueAdmissionProbability > 0
		observation.ControlEligible = observation.VisibilityClass == "NO_RELEVANT_TARGET" &&
			observation.VisibilityAuthority == "A" &&
			observation.TimingEligible &&
			observation.QueueAdmissionProbability > 0
		observation.PositiveControlEligible = (observation.VisibilityClass == "EXPOSED" || observation.VisibilityClass == "PARTIALLY_EXPOSED") &&
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

func firstExposure(events []flatEvent, startIndex int, origin flatEvent, observation Observation, targetID string, sourceIDs []string) exposureResult {
	const exposureHorizonMS = int64(30_000)
	result := exposureResult{cause: "UNKNOWN", censored: true, sourceIDs: sourceIDs}
	for _, candidate := range events[startIndex:] {
		if !sameServerSession(origin, candidate) {
			break
		}
		if candidate.event.ServerTimeMS > observation.StartedMS+exposureHorizonMS {
			break
		}
		if candidate.event.PlayerSessionID != observation.ObserverPlayerSessionID || candidate.event.EventType != "VISIBILITY_OBSERVATION" || candidate.data.TargetPlayerID != targetID || candidate.data.SamplingStream != "event_enrichment" {
			continue
		}
		if candidate.data.Classification != "EXPOSED" && candidate.data.Classification != "PARTIALLY_EXPOSED" {
			continue
		}
		lower := candidate.data.ProbeStartedMS
		if lower <= 0 {
			lower = candidate.event.ServerTimeMS
		}
		upper := candidate.data.ProbeCompletedMS
		if upper < lower {
			upper = lower
		}
		result.lowerMS = lower
		result.upperMS = upper
		result.estimateMS = lower + ((upper - lower) / 2)
		result.censored = false
		result.sourceIDs = append(sourceIDs, candidate.id)
		return result
	}
	return result
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

func readinessOutcome(events []flatEvent, startIndex int, origin flatEvent, observation Observation, sourceIDs []string) outcomeResult {
	result := outcomeResult{sourceIDs: sourceIDs}
	for _, candidate := range events[startIndex:] {
		if !sameServerSession(origin, candidate) {
			break
		}
		if candidate.event.ServerTimeMS > observation.ClosedMS+5_000 {
			break
		}
		if candidate.event.PlayerSessionID != observation.ObserverPlayerSessionID || candidate.event.EventType != "DECISION_EDGE" {
			continue
		}
		switch candidate.data.SamplingReason {
		case "WEAPON_RAISED", "ADS_ENTERED", "OPTICS_ENTERED":
		default:
			continue
		}
		lower, upper, estimate, uncertainty, source := eventInterval(candidate)
		if upper < observation.StartedMS || lower > observation.ClosedMS {
			continue
		}
		result.observed = true
		result.estimateMS = estimate
		result.lowerMS = lower
		result.upperMS = upper
		result.uncertainty = uncertainty
		result.timing = source
		result.authority = string(candidate.event.SourceAuthority)
		result.sourceIDs = append(sourceIDs, candidate.id)
		return result
	}
	return result
}

func eventInterval(event flatEvent) (int64, int64, int64, int64, string) {
	lower := event.data.EventTimeLowerMS
	upper := event.data.EventTimeUpperMS
	if lower <= 0 || upper < lower {
		lower = event.event.ServerTimeMS
		upper = lower
	}
	estimate := event.data.EventTimeEstimateMS
	if estimate < lower || estimate > upper {
		estimate = lower + ((upper - lower) / 2)
	}
	uncertainty := event.data.EventTimeUncertaintyMS
	if uncertainty <= 0 {
		uncertainty = (upper - lower) / 2
	}
	source := event.data.EventTimingSource
	if source == "" {
		source = "SERVER_EVENT_POINT"
	}
	return lower, upper, estimate, uncertainty, source
}

func cueClass(events []flatEvent, currentIndex int, origin flatEvent, observerSessionID, targetID string, atMS int64) (string, *CueFact) {
	if targetID == "" {
		return "UNEXPLAINED_IN_CAPTURED_DATA", nil
	}
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
			observation.PositiveControlEligible = false
		} else {
			lastFeatureByObserver[observation.ObserverPlayerSessionID] = observation.StartedMS
		}
	}
}

func sameServerSession(left, right flatEvent) bool {
	return left.batch.ServerID == right.batch.ServerID && left.batch.ServerSessionID == right.batch.ServerSessionID
}

func isRandomOpportunityEvent(event flatEvent) bool {
	if event.data.SamplingStream != "random_opportunity" {
		return false
	}
	return event.event.EventType == "VISIBILITY_OBSERVATION" || event.event.EventType == "SAMPLING_OPPORTUNITY"
}

func controlKind(classification, explicit string) string {
	if explicit != "" {
		return explicit
	}
	switch classification {
	case "ROBUSTLY_OCCLUDED", "HEAD_ORIGIN_OCCLUDED":
		return "HIDDEN_TARGET"
	case "EXPOSED", "PARTIALLY_EXPOSED":
		return "VISIBLE_TARGET"
	case "NO_RELEVANT_TARGET":
		return "NEUTRAL_NO_RELEVANT_TARGET"
	default:
		return "OTHER"
	}
}

func validatedFirstPersonOrigin(value string) bool {
	return value == "VALIDATED_FIRST_PERSON_HEAD" || value == "FIRST_PERSON_EYE"
}

func targetIdentityKey(targetID, targetSessionID string) string {
	if targetID != "" {
		return stableID("target-identity", targetID)
	}
	if targetSessionID != "" {
		return stableID("target-session", targetSessionID)
	}
	return ""
}

func stableID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:16])
}
