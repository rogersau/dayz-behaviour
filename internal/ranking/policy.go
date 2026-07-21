package ranking

import "github.com/rogersau/dayz-behaviour/internal/features"

const PolicyVersion = "transparent-gates-v1"

type Tier string

const (
	InsufficientData Tier = "INSUFFICIENT_DATA"
	Monitor          Tier = "MONITOR"
	Review           Tier = "REVIEW"
	HighPriority     Tier = "HIGH_PRIORITY_REVIEW"
)

type Decision struct {
	Tier          Tier     `json:"tier"`
	PolicyVersion string   `json:"policy_version"`
	Reasons       []string `json:"reasons"`
}

type Gates struct {
	MinimumHiddenTrials  int
	MinimumSessions      int
	MinimumEncounters    int
	MinimumTargets       int
	ReviewLiftLowerBound float64
	HighLiftLowerBound   float64
}

func DefaultGates() Gates {
	return Gates{
		MinimumHiddenTrials: 20, MinimumSessions: 3, MinimumEncounters: 5, MinimumTargets: 5,
		ReviewLiftLowerBound: 0.05, HighLiftLowerBound: 0.15,
	}
}

func Apply(readiness features.ReadinessResult, gates Gates) Decision {
	return apply(readiness, nil, nil, gates)
}

func ApplyEvidence(readiness features.ReadinessResult, matched features.ConditionalLogitResult, stability features.StabilityResult, gates Gates) Decision {
	return apply(readiness, &matched, &stability, gates)
}

func ApplyValidatedEvidence(readiness features.ReadinessResult, matched features.ConditionalLogitResult, stability features.StabilityResult, controls features.NegativeControlResult, gates Gates) Decision {
	decision := apply(readiness, &matched, &stability, gates)
	if decision.Tier == HighPriority && !controls.Passed {
		decision.Tier = Review
		decision.Reasons = append(decision.Reasons, "high-priority suppressed: preregistered negative controls are incomplete or failed")
	}
	return decision
}

func apply(readiness features.ReadinessResult, matched *features.ConditionalLogitResult, stability *features.StabilityResult, gates Gates) Decision {
	decision := Decision{Tier: InsufficientData, PolicyVersion: PolicyVersion}
	if readiness.HiddenTrials < gates.MinimumHiddenTrials {
		decision.Reasons = append(decision.Reasons, "insufficient hidden opportunities")
	}
	if readiness.IndependentSessionCount < gates.MinimumSessions {
		decision.Reasons = append(decision.Reasons, "insufficient independent sessions")
	}
	if readiness.IndependentEncounterCount < gates.MinimumEncounters {
		decision.Reasons = append(decision.Reasons, "insufficient independent encounters")
	}
	if readiness.IndependentTargetCount < gates.MinimumTargets {
		decision.Reasons = append(decision.Reasons, "insufficient independent targets")
	}
	if len(decision.Reasons) > 0 {
		return decision
	}
	decision.Tier = Monitor
	decision.Reasons = []string{"evidence breadth gates passed"}
	if readiness.ReadinessLiftLowerBound >= gates.ReviewLiftLowerBound {
		decision.Tier = Review
		decision.Reasons = append(decision.Reasons, "readiness lift lower bound passed review gate")
	}
	if readiness.ReadinessLiftLowerBound >= gates.HighLiftLowerBound {
		if matched == nil || stability == nil {
			decision.Tier = HighPriority
			decision.Reasons = append(decision.Reasons, "readiness lift lower bound passed high-priority gate")
		} else if matched.Converged && !matched.Separated && matched.UsefulStrata >= 3 && matched.Lower95 > 1 && stability.DirectionStable {
			decision.Tier = HighPriority
			decision.Reasons = append(decision.Reasons, "matched odds-ratio and leave-one-session-out stability gates passed")
		} else {
			decision.Reasons = append(decision.Reasons, "high-priority suppressed: matched-model or stability validation did not pass")
		}
	}
	return decision
}
