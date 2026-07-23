package features

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
	"strings"

	"github.com/rogersau/dayz-behaviour/internal/observations"
)

const (
	MatchingPolicyVersion   = "exact-context-v2-neutral-controls"
	ConditionalLogitVersion = "conditional-logit-v1"
)

type MatchedStratum struct {
	StratumID       string                     `json:"stratum_id"`
	PlayerSessionID string                     `json:"player_session_id"`
	ContextKey      string                     `json:"context_key"`
	Observations    []observations.Observation `json:"observations"`
	ControlQuality  float64                    `json:"control_quality"`
	PolicyVersion   string                     `json:"policy_version"`
}

type ConditionalLogitResult struct {
	PlayerSessionID  string  `json:"player_session_id"`
	LogOddsRatio     float64 `json:"log_odds_ratio"`
	OddsRatio        float64 `json:"odds_ratio"`
	Lower95          float64 `json:"lower_95"`
	Upper95          float64 `json:"upper_95"`
	StandardError    float64 `json:"standard_error"`
	UsefulStrata     int     `json:"useful_strata"`
	ObservationCount int     `json:"observation_count"`
	Converged        bool    `json:"converged"`
	Separated        bool    `json:"separated"`
	AlgorithmVersion string  `json:"algorithm_version"`
}

// BuildMatchedStrata performs exact within-player matching on context available
// to both hidden-target and neutral no-target opportunities. Target distance is
// intentionally excluded until a geometry-matched empty-target control exists;
// the reduced control quality records that limitation explicitly.
func BuildMatchedStrata(input []observations.Observation) []MatchedStratum {
	groups := map[string][]observations.Observation{}
	for _, observation := range input {
		if !observation.Independent || observation.CueClass != "UNEXPLAINED_IN_CAPTURED_DATA" ||
			(!observation.StrongHiddenEligible && !observation.ControlEligible) {
			continue
		}
		key := strings.Join([]string{
			observation.ObserverPlayerSessionID, observation.ServerID, observation.MapID, observation.AreaCell,
			observation.ObserverMovementBand, intString(observation.ObserverStanceID), observation.BaselineWeaponState,
			observation.CameraMode, observation.ServerPopulationBand, observation.CueClass, observation.SamplingPolicyVersion,
		}, "|")
		groups[key] = append(groups[key], observation)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]MatchedStratum, 0, len(keys))
	for _, key := range keys {
		items := groups[key]
		hidden, controls := 0, 0
		for _, item := range items {
			if item.StrongHiddenEligible {
				hidden++
			}
			if item.ControlEligible {
				controls++
			}
		}
		if hidden == 0 || controls == 0 {
			continue
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ObservationID < items[j].ObservationID })
		result = append(result, MatchedStratum{
			StratumID: "stratum_" + hash16(key), PlayerSessionID: items[0].ObserverPlayerSessionID,
			ContextKey: key, Observations: items, ControlQuality: 0.5, PolicyVersion: MatchingPolicyVersion,
		})
	}
	return result
}

// FitConditionalLogit fits a one-coefficient conditional logistic model. The
// stratum intercept is conditioned out exactly by summing over possible hidden
// outcome counts. Sparse or separated fits are explicitly suppressed.
func FitConditionalLogit(playerSessionID string, strata []MatchedStratum) ConditionalLogitResult {
	return FitConditionalLogitForSessions(playerSessionID, []string{playerSessionID}, strata)
}

func FitConditionalLogitForSessions(playerID string, sessionIDs []string, strata []MatchedStratum) ConditionalLogitResult {
	result := ConditionalLogitResult{PlayerSessionID: playerID, AlgorithmVersion: ConditionalLogitVersion}
	eligible := map[string]struct{}{}
	for _, sessionID := range sessionIDs {
		eligible[sessionID] = struct{}{}
	}
	type sufficient struct{ hidden, control, outcomes, observedHidden int }
	var useful []sufficient
	for _, stratum := range strata {
		if _, ok := eligible[stratum.PlayerSessionID]; !ok {
			continue
		}
		var value sufficient
		for _, observation := range stratum.Observations {
			if observation.StrongHiddenEligible {
				value.hidden++
			}
			if observation.ControlEligible {
				value.control++
			}
			if observation.OutcomeObserved {
				value.outcomes++
				if observation.StrongHiddenEligible {
					value.observedHidden++
				}
			}
		}
		if value.hidden == 0 || value.control == 0 || value.outcomes == 0 || value.outcomes == value.hidden+value.control {
			continue
		}
		useful = append(useful, value)
		result.ObservationCount += value.hidden + value.control
	}
	result.UsefulStrata = len(useful)
	if len(useful) == 0 {
		return result
	}
	beta := 0.0
	for iteration := 0; iteration < 50; iteration++ {
		score, information := 0.0, 0.0
		for _, value := range useful {
			expected, variance := conditionalMoments(value.hidden, value.control, value.outcomes, beta)
			score += float64(value.observedHidden) - expected
			information += variance
		}
		if information < 1e-10 {
			result.Separated = true
			return result
		}
		step := score / information
		if step > 2 {
			step = 2
		}
		if step < -2 {
			step = -2
		}
		beta += step
		if math.Abs(beta) > 20 {
			result.Separated = true
			return result
		}
		if math.Abs(step) < 1e-8 {
			result.Converged = true
			break
		}
	}
	if !result.Converged {
		return result
	}
	information := 0.0
	for _, value := range useful {
		_, variance := conditionalMoments(value.hidden, value.control, value.outcomes, beta)
		information += variance
	}
	if information <= 0 {
		result.Separated = true
		result.Converged = false
		return result
	}
	result.LogOddsRatio = beta
	result.StandardError = 1 / math.Sqrt(information)
	result.OddsRatio = math.Exp(beta)
	result.Lower95 = math.Exp(beta - 1.96*result.StandardError)
	result.Upper95 = math.Exp(beta + 1.96*result.StandardError)
	return result
}

func conditionalMoments(hidden, control, outcomes int, beta float64) (float64, float64) {
	minimum := maxInt(0, outcomes-control)
	maximum := minInt(hidden, outcomes)
	logs := make([]float64, 0, maximum-minimum+1)
	maxLog := math.Inf(-1)
	for count := minimum; count <= maximum; count++ {
		value := logChoose(hidden, count) + logChoose(control, outcomes-count) + beta*float64(count)
		logs = append(logs, value)
		if value > maxLog {
			maxLog = value
		}
	}
	weightSum, first, second := 0.0, 0.0, 0.0
	for index, value := range logs {
		count := float64(minimum + index)
		weight := math.Exp(value - maxLog)
		weightSum += weight
		first += count * weight
		second += count * count * weight
	}
	mean := first / weightSum
	return mean, second/weightSum - mean*mean
}

func logChoose(n, k int) float64 {
	left, _ := math.Lgamma(float64(n + 1))
	rightA, _ := math.Lgamma(float64(k + 1))
	rightB, _ := math.Lgamma(float64(n - k + 1))
	return left - rightA - rightB
}

func hash16(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:16])
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var bytes [24]byte
	index := len(bytes)
	for value > 0 {
		index--
		bytes[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		bytes[index] = '-'
	}
	return string(bytes[index:])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
