package features

import "sort"

const PreExposureAlgorithmVersion = "interval-pre-exposure-supporting-v2"

const PreExposureStatus = "EXPERIMENTAL_SUPPORTING_ONLY"

type PreExposureIncident struct {
	ReadinessLowerMS int64
	ReadinessUpperMS int64
	ExposureLowerMS  int64
	ExposureUpperMS  int64
	Censored         bool
}

type PreExposureResult struct {
	Status                   string  `json:"status"`
	IncidentCount            int     `json:"incident_count"`
	DefinitePreExposureCount int     `json:"definite_pre_exposure_count"`
	AmbiguousTimingCount     int     `json:"ambiguous_timing_count"`
	MedianLeadLowerMS        int64   `json:"median_lead_lower_ms"`
	MedianLeadUpperMS        int64   `json:"median_lead_upper_ms"`
	PreExposureRateLower     float64 `json:"pre_exposure_rate_lower"`
	PreExposureRateUpper     float64 `json:"pre_exposure_rate_upper"`
	AlgorithmVersion         string  `json:"algorithm_version"`
}

func EstimatePreExposure(input []PreExposureIncident) PreExposureResult {
	result := PreExposureResult{Status: PreExposureStatus, AlgorithmVersion: PreExposureAlgorithmVersion}
	var lowerLeads, upperLeads []int64
	for _, incident := range input {
		if incident.Censored || incident.ExposureLowerMS <= 0 || incident.ExposureUpperMS < incident.ExposureLowerMS {
			continue
		}
		result.IncidentCount++
		if incident.ReadinessLowerMS <= 0 || incident.ReadinessUpperMS < incident.ReadinessLowerMS {
			continue
		}
		if incident.ReadinessUpperMS < incident.ExposureLowerMS {
			result.DefinitePreExposureCount++
			lowerLeads = append(lowerLeads, incident.ExposureLowerMS-incident.ReadinessUpperMS)
			upperLeads = append(upperLeads, incident.ExposureUpperMS-incident.ReadinessLowerMS)
			continue
		}
		if incident.ReadinessLowerMS < incident.ExposureUpperMS {
			result.AmbiguousTimingCount++
		}
	}
	if result.IncidentCount > 0 {
		result.PreExposureRateLower = float64(result.DefinitePreExposureCount) / float64(result.IncidentCount)
		result.PreExposureRateUpper = float64(result.DefinitePreExposureCount+result.AmbiguousTimingCount) / float64(result.IncidentCount)
	}
	result.MedianLeadLowerMS = medianInt64(lowerLeads)
	result.MedianLeadUpperMS = medianInt64(upperLeads)
	return result
}

func medianInt64(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
}
