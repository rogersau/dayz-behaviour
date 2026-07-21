package features_test

import (
	"math"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/features"
)

func TestCircularPermutationDetectsAlignedSectors(t *testing.T) {
	headings := []float64{0, 0.1, -0.1, 0.2}
	bearings := []float64{0, 0.1, -0.1, 0.2}
	nulls := []float64{math.Pi / 2, math.Pi, -math.Pi / 2}
	result := features.CircularPermutation(headings, bearings, nulls)
	if result.ObservedConcentration < 0.99 || result.PermutationPValue > 0.3 {
		t.Fatalf("result = %+v", result)
	}
}
