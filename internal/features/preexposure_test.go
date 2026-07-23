package features_test

import (
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/features"
)

func TestPreExposureUsesIntervalBounds(t *testing.T) {
	result := features.EstimatePreExposure([]features.PreExposureIncident{
		{ReadinessLowerMS: 1_000, ReadinessUpperMS: 1_100, ExposureLowerMS: 1_500, ExposureUpperMS: 1_600},
		{ReadinessLowerMS: 2_000, ReadinessUpperMS: 2_500, ExposureLowerMS: 2_300, ExposureUpperMS: 2_600},
		{ReadinessLowerMS: 3_000, ReadinessUpperMS: 3_100, ExposureLowerMS: 2_800, ExposureUpperMS: 2_900},
	})
	if result.Status != features.PreExposureStatus {
		t.Fatalf("status = %q", result.Status)
	}
	if result.IncidentCount != 3 || result.DefinitePreExposureCount != 1 || result.AmbiguousTimingCount != 1 {
		t.Fatalf("unexpected counts: %+v", result)
	}
	if result.MedianLeadLowerMS != 400 || result.MedianLeadUpperMS != 600 {
		t.Fatalf("unexpected lead interval: %+v", result)
	}
	if result.PreExposureRateLower != 1.0/3.0 || result.PreExposureRateUpper != 2.0/3.0 {
		t.Fatalf("unexpected rate bounds: %+v", result)
	}
}
