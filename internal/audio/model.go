package audio

import (
	"math"
	"strings"
)

const ModelVersion = "server-derived-audio-v1"

type Audibility string

const (
	NotAudible Audibility = "NOT_AUDIBLE_BY_MODEL"
	Possible   Audibility = "POSSIBLE_AUDIO_CUE"
	Likely     Audibility = "LIKELY_AUDIO_CUE"
	Strong     Audibility = "CAPTURED_STRONG_CUE"
)

type Result struct {
	Audibility    Audibility `json:"audibility"`
	LikelyRangeM  float64    `json:"likely_range_m"`
	MaximumRangeM float64    `json:"maximum_range_m"`
	ModelVersion  string     `json:"model_version"`
}

type GunshotInput struct {
	DistanceM  float64
	Suppressed bool
}

type FootstepInput struct {
	DistanceM   float64
	SpeedMPS    float64
	Stance      string
	SurfaceType string
	Footwear    string
}

func ClassifyGunshot(input GunshotInput) Result {
	strongRange, likelyRange, maximumRange := 300.0, 900.0, 1800.0
	if input.Suppressed {
		strongRange, likelyRange, maximumRange = 60, 160, 350
	}
	return classify(input.DistanceM, strongRange, likelyRange, maximumRange)
}

func ClassifyFootstep(input FootstepInput) Result {
	if input.SpeedMPS < 0.25 {
		return Result{Audibility: NotAudible, ModelVersion: ModelVersion}
	}

	likelyRange := 4.0
	switch {
	case input.SpeedMPS >= 4.5:
		likelyRange = 24
	case input.SpeedMPS >= 2.5:
		likelyRange = 16
	case input.SpeedMPS >= 1.2:
		likelyRange = 9
	}

	stance := strings.ToUpper(input.Stance)
	if strings.Contains(stance, "PRONE") {
		likelyRange *= 0.45
	} else if strings.Contains(stance, "CROUCH") {
		likelyRange *= 0.7
	}

	likelyRange *= surfaceMultiplier(input.SurfaceType)
	likelyRange *= footwearMultiplier(input.Footwear)
	maximumRange := likelyRange * 1.75
	return classify(input.DistanceM, 0, likelyRange, maximumRange)
}

func classify(distance, strongRange, likelyRange, maximumRange float64) Result {
	result := Result{LikelyRangeM: likelyRange, MaximumRangeM: maximumRange, ModelVersion: ModelVersion}
	if distance < 0 || math.IsNaN(distance) || math.IsInf(distance, 0) {
		result.Audibility = NotAudible
		return result
	}
	switch {
	case strongRange > 0 && distance <= strongRange:
		result.Audibility = Strong
	case distance <= likelyRange:
		result.Audibility = Likely
	case distance <= maximumRange:
		result.Audibility = Possible
	default:
		result.Audibility = NotAudible
	}
	return result
}

func surfaceMultiplier(value string) float64 {
	value = strings.ToLower(value)
	switch {
	case containsAny(value, "metal", "concrete", "asphalt", "road", "gravel", "wood", "floor", "stone"):
		return 1.25
	case containsAny(value, "grass", "forest", "soil", "dirt", "sand", "snow", "moss"):
		return 0.8
	default:
		return 1
	}
}

func footwearMultiplier(value string) float64 {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || containsAny(value, "barefoot", "none") {
		return 0.75
	}
	if containsAny(value, "boot", "combat", "hiking", "wellies") {
		return 1.1
	}
	return 1
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
