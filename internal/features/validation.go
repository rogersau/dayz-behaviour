package features

import (
	"math"
	"sort"
)

const ValidationAlgorithmVersion = "negative-controls-v1"

type NegativeControlCheck struct {
	Name      string  `json:"name"`
	Available bool    `json:"available"`
	Effect    float64 `json:"effect"`
	Passed    bool    `json:"passed"`
	Reason    string  `json:"reason,omitempty"`
}

type NegativeControlResult struct {
	Passed           bool                   `json:"passed"`
	MaterialityLimit float64                `json:"materiality_limit"`
	Checks           []NegativeControlCheck `json:"checks"`
	AlgorithmVersion string                 `json:"algorithm_version"`
}

// RunNegativeControls executes controls that can be derived from constructed
// risk sets. Position-dependent controls are explicitly unavailable until the
// source replay contains sufficient target trajectories; unavailable controls
// fail closed for high-priority ranking.
func RunNegativeControls(strata []MatchedStratum, materialityLimit float64) NegativeControlResult {
	if materialityLimit <= 0 {
		materialityLimit = 0.05
	}
	result := NegativeControlResult{Passed: true, MaterialityLimit: materialityLimit, AlgorithmVersion: ValidationAlgorithmVersion}
	labelEffect, labelAvailable := rotatedEffect(strata, false)
	outcomeEffect, outcomeAvailable := rotatedEffect(strata, true)
	result.Checks = append(result.Checks,
		check("random opportunity labels permuted within matched strata", labelAvailable, labelEffect, materialityLimit, "requires at least one rotatable matched stratum"),
		check("outcomes shifted outside the declared decision window", outcomeAvailable, outcomeEffect, materialityLimit, "requires at least one rotatable matched stratum"),
		NegativeControlCheck{Name: "future and time-shuffled target positions", Available: false, Reason: "requires replayed target trajectories"},
		NegativeControlCheck{Name: "randomly reassigned target identities", Available: false, Reason: "requires multiple eligible target trajectories per risk set"},
		NegativeControlCheck{Name: "pseudo-target sectors", Available: false, Reason: "requires validated camera headings and candidate-sector geometry"},
		NegativeControlCheck{Name: "deliberately delayed visibility probes", Available: false, Reason: "requires controlled delayed-probe fixture"},
	)
	for _, item := range result.Checks {
		if !item.Available || !item.Passed {
			result.Passed = false
		}
	}
	return result
}

func check(name string, available bool, effect, limit float64, unavailableReason string) NegativeControlCheck {
	value := NegativeControlCheck{Name: name, Available: available, Effect: effect}
	if !available {
		value.Reason = unavailableReason
		return value
	}
	value.Passed = math.Abs(effect) < limit
	if !value.Passed {
		value.Reason = "material signal detected in negative control"
	}
	return value
}

func rotatedEffect(strata []MatchedStratum, rotateOutcome bool) (float64, bool) {
	hiddenSuccess, hiddenN, controlSuccess, controlN := 0, 0, 0, 0
	available := false
	for _, stratum := range strata {
		items := stratum.Observations
		if len(items) < 2 {
			continue
		}
		available = true
		for index, item := range items {
			rotated := items[(index+1)%len(items)]
			hidden := item.StrongHiddenEligible
			outcome := item.OutcomeObserved
			if rotateOutcome {
				outcome = rotated.OutcomeObserved
			} else {
				hidden = rotated.StrongHiddenEligible
			}
			if hidden {
				hiddenN++
				if outcome {
					hiddenSuccess++
				}
			} else if item.ControlEligible || (!rotateOutcome && rotated.ControlEligible) {
				controlN++
				if outcome {
					controlSuccess++
				}
			}
		}
	}
	if !available || hiddenN == 0 || controlN == 0 {
		return 0, false
	}
	return float64(hiddenSuccess)/float64(hiddenN) - float64(controlSuccess)/float64(controlN), true
}

type AdjustedPValue struct {
	ID       string  `json:"id"`
	Raw      float64 `json:"raw"`
	Adjusted float64 `json:"adjusted"`
}

// BenjaminiHochberg controls false discovery rate for exploratory screens. It
// is not used to promote a primary review case.
func BenjaminiHochberg(values map[string]float64) []AdjustedPValue {
	result := make([]AdjustedPValue, 0, len(values))
	for id, value := range values {
		if value < 0 {
			value = 0
		}
		if value > 1 {
			value = 1
		}
		result = append(result, AdjustedPValue{ID: id, Raw: value})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Raw == result[j].Raw {
			return result[i].ID < result[j].ID
		}
		return result[i].Raw < result[j].Raw
	})
	previous := 1.0
	for index := len(result) - 1; index >= 0; index-- {
		adjusted := result[index].Raw * float64(len(result)) / float64(index+1)
		if adjusted > previous {
			adjusted = previous
		}
		if adjusted > 1 {
			adjusted = 1
		}
		result[index].Adjusted = adjusted
		previous = adjusted
	}
	return result
}

type RobustCohortScore struct {
	Median float64 `json:"median"`
	MAD    float64 `json:"mad"`
	Score  float64 `json:"robust_z_score"`
}

func RobustScore(value float64, cohort []float64) RobustCohortScore {
	if len(cohort) == 0 {
		return RobustCohortScore{}
	}
	median := sampleMedian(cohort)
	deviations := make([]float64, len(cohort))
	for index, item := range cohort {
		deviations[index] = math.Abs(item - median)
	}
	mad := sampleMedian(deviations)
	result := RobustCohortScore{Median: median, MAD: mad}
	if mad > 0 {
		result.Score = 0.67448975 * (value - median) / mad
	}
	return result
}

func sampleMedian(input []float64) float64 {
	values := append([]float64(nil), input...)
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
}
