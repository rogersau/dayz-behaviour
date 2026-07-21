package features

import (
	"math"
	"sort"

	"github.com/rogersau/dayz-behaviour/internal/observations"
)

const StabilityAlgorithmVersion = "leave-one-session-out-v1"

type StabilityResult struct {
	SessionCount      int     `json:"session_count"`
	MinimumLift       float64 `json:"minimum_lift"`
	MaximumLift       float64 `json:"maximum_lift"`
	MaximumRankChange int     `json:"maximum_rank_change"`
	DirectionStable   bool    `json:"direction_stable"`
	AlgorithmVersion  string  `json:"algorithm_version"`
}

func LeaveOneSessionOut(playerID string, sessionIDs []string, input []observations.Observation, priorAlpha, priorBeta float64) StabilityResult {
	result := StabilityResult{SessionCount: len(sessionIDs), MinimumLift: math.Inf(1), MaximumLift: math.Inf(-1), DirectionStable: true, AlgorithmVersion: StabilityAlgorithmVersion}
	if len(sessionIDs) < 2 {
		result.MinimumLift, result.MaximumLift, result.DirectionStable = 0, 0, false
		return result
	}
	sorted := append([]string(nil), sessionIDs...)
	sort.Strings(sorted)
	for excluded := range sorted {
		kept := make([]string, 0, len(sorted)-1)
		for index, sessionID := range sorted {
			if index != excluded {
				kept = append(kept, sessionID)
			}
		}
		estimate := EstimateReadinessForSessions(playerID, kept, input, priorAlpha, priorBeta)
		if estimate.ReadinessLift < result.MinimumLift {
			result.MinimumLift = estimate.ReadinessLift
		}
		if estimate.ReadinessLift > result.MaximumLift {
			result.MaximumLift = estimate.ReadinessLift
		}
		if estimate.ReadinessLiftLowerBound <= 0 {
			result.DirectionStable = false
		}
	}
	return result
}
