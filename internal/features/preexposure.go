package features

import (
	"math"
	"sort"
)

const PreExposureAlgorithmVersion = "discrete-time-pre-exposure-v1"

type PreExposureIncident struct {
	ReadinessMS int64
	ExposureMS  int64
	Censored    bool
}

type PreExposureResult struct {
	IncidentCount    int     `json:"incident_count"`
	PreExposureCount int     `json:"pre_exposure_count"`
	MedianLeadMS     int64   `json:"median_lead_ms"`
	PreExposureRate  float64 `json:"pre_exposure_rate"`
	AlgorithmVersion string  `json:"algorithm_version"`
}

func EstimatePreExposure(input []PreExposureIncident) PreExposureResult {
	result := PreExposureResult{AlgorithmVersion: PreExposureAlgorithmVersion}
	var leads []int64
	for _, incident := range input {
		if incident.Censored || incident.ExposureMS <= 0 {
			continue
		}
		result.IncidentCount++
		if incident.ReadinessMS > 0 && incident.ReadinessMS < incident.ExposureMS {
			result.PreExposureCount++
			leads = append(leads, incident.ExposureMS-incident.ReadinessMS)
		}
	}
	if result.IncidentCount > 0 {
		result.PreExposureRate = float64(result.PreExposureCount) / float64(result.IncidentCount)
	}
	if len(leads) > 0 {
		sort.Slice(leads, func(i, j int) bool { return leads[i] < leads[j] })
		result.MedianLeadMS = leads[int(math.Floor(float64(len(leads)-1)/2))]
	}
	return result
}
