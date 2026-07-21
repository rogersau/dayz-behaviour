package features

import "math"

const SectorAlgorithmVersion = "circular-permutation-v1"

type SectorResult struct {
	ObservedConcentration float64 `json:"observed_concentration"`
	NullMean              float64 `json:"null_mean"`
	PermutationPValue     float64 `json:"permutation_p_value"`
	SampleCount           int     `json:"sample_count"`
	AlgorithmVersion      string  `json:"algorithm_version"`
}

// CircularPermutation compares observer headings with target bearings, then
// rotates every target bearing by each supplied null shift. Inputs are radians.
func CircularPermutation(observerHeadings, targetBearings, nullShifts []float64) SectorResult {
	result := SectorResult{AlgorithmVersion: SectorAlgorithmVersion}
	if len(observerHeadings) == 0 || len(observerHeadings) != len(targetBearings) || len(nullShifts) == 0 {
		return result
	}
	result.SampleCount = len(observerHeadings)
	result.ObservedConcentration = concentration(observerHeadings, targetBearings, 0)
	var nullSum float64
	extreme := 0
	for _, shift := range nullShifts {
		value := concentration(observerHeadings, targetBearings, shift)
		nullSum += value
		if value >= result.ObservedConcentration {
			extreme++
		}
	}
	result.NullMean = nullSum / float64(len(nullShifts))
	result.PermutationPValue = float64(extreme+1) / float64(len(nullShifts)+1)
	return result
}

func concentration(headings, bearings []float64, shift float64) float64 {
	var sum float64
	for index := range headings {
		sum += math.Cos(angularDifference(headings[index], bearings[index]+shift))
	}
	return sum / float64(len(headings))
}

func angularDifference(left, right float64) float64 {
	difference := math.Mod(left-right+math.Pi, 2*math.Pi)
	if difference < 0 {
		difference += 2 * math.Pi
	}
	return difference - math.Pi
}
