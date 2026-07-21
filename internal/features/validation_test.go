package features

import "testing"

func TestBenjaminiHochbergIsMonotone(t *testing.T) {
	result := BenjaminiHochberg(map[string]float64{"a": .01, "b": .04, "c": .03})
	for index := 1; index < len(result); index++ {
		if result[index].Adjusted < result[index-1].Adjusted {
			t.Fatalf("not monotone: %+v", result)
		}
	}
}

func TestRobustScoreResistsSingleExtremeValue(t *testing.T) {
	result := RobustScore(10, []float64{1, 1.1, .9, 1.05, 100})
	if result.Median > 1.2 || result.Score < 10 {
		t.Fatalf("unexpected robust score: %+v", result)
	}
}
