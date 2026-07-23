package features

import (
	"math"

	"github.com/rogersau/dayz-behaviour/internal/observations"
)

const ReadinessAlgorithmVersion = "beta-binomial-v3-neutral-controls"

type ReadinessResult struct {
	PlayerSessionID           string  `json:"player_session_id"`
	HiddenSuccesses           int     `json:"hidden_successes"`
	HiddenTrials              int     `json:"hidden_trials"`
	ControlSuccesses          int     `json:"control_successes"`
	ControlTrials             int     `json:"control_trials"`
	HiddenPosteriorMean       float64 `json:"hidden_posterior_mean"`
	HiddenLowerBound          float64 `json:"hidden_lower_bound"`
	ControlPosteriorMean      float64 `json:"control_posterior_mean"`
	ReadinessLift             float64 `json:"readiness_lift"`
	ReadinessLiftLowerBound   float64 `json:"readiness_lift_lower_bound"`
	IndependentSessionCount   int     `json:"independent_session_count"`
	IndependentEncounterCount int     `json:"independent_encounter_count"`
	IndependentTargetCount    int     `json:"independent_target_count"`
	AlgorithmVersion          string  `json:"algorithm_version"`
}

func EstimateReadiness(playerSessionID string, input []observations.Observation, priorAlpha, priorBeta float64) ReadinessResult {
	return EstimateReadinessForSessions(playerSessionID, []string{playerSessionID}, input, priorAlpha, priorBeta)
}

func EstimateReadinessForSessions(playerID string, playerSessionIDs []string, input []observations.Observation, priorAlpha, priorBeta float64) ReadinessResult {
	if priorAlpha <= 0 || priorBeta <= 0 {
		priorAlpha, priorBeta = 1, 1
	}
	result := ReadinessResult{PlayerSessionID: playerID, AlgorithmVersion: ReadinessAlgorithmVersion}
	eligibleSessions := map[string]struct{}{}
	for _, sessionID := range playerSessionIDs {
		eligibleSessions[sessionID] = struct{}{}
	}
	sessions := map[string]struct{}{}
	encounters := map[string]struct{}{}
	targets := map[string]struct{}{}
	for _, observation := range input {
		if _, ok := eligibleSessions[observation.ObserverPlayerSessionID]; !ok || !observation.Independent ||
			observation.CueClass != "UNEXPLAINED_IN_CAPTURED_DATA" {
			continue
		}
		if observation.StrongHiddenEligible {
			result.HiddenTrials++
			if observation.OutcomeObserved {
				result.HiddenSuccesses++
			}
		} else if observation.ControlEligible {
			result.ControlTrials++
			if observation.OutcomeObserved {
				result.ControlSuccesses++
			}
		} else {
			continue
		}
		sessions[observation.ObserverPlayerSessionID] = struct{}{}
		encounters[observation.EncounterID] = struct{}{}
		targetKey := observation.TargetIdentityKey
		if targetKey == "" {
			targetKey = observation.TargetPlayerSessionID
		}
		if targetKey != "" {
			targets[targetKey] = struct{}{}
		}
	}
	hiddenMean, hiddenVariance := betaMoments(priorAlpha+float64(result.HiddenSuccesses), priorBeta+float64(result.HiddenTrials-result.HiddenSuccesses))
	controlMean, controlVariance := betaMoments(priorAlpha+float64(result.ControlSuccesses), priorBeta+float64(result.ControlTrials-result.ControlSuccesses))
	result.HiddenPosteriorMean = hiddenMean
	result.HiddenLowerBound = clamp01(hiddenMean - 1.96*math.Sqrt(hiddenVariance))
	result.ControlPosteriorMean = controlMean
	result.ReadinessLift = hiddenMean - controlMean
	result.ReadinessLiftLowerBound = result.ReadinessLift - 1.96*math.Sqrt(hiddenVariance+controlVariance)
	result.IndependentSessionCount = len(sessions)
	result.IndependentEncounterCount = len(encounters)
	result.IndependentTargetCount = len(targets)
	return result
}

func betaMoments(alpha, beta float64) (float64, float64) {
	total := alpha + beta
	mean := alpha / total
	return mean, alpha * beta / (total * total * (total + 1))
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
