package features

import (
	"encoding/json"
	"math"
	"sort"

	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

type sectorEvent struct {
	event   schema.Event
	payload struct {
		SamplingReason  string    `json:"sampling_reason"`
		SamplingStream  string    `json:"sampling_stream"`
		Classification  string    `json:"classification"`
		CameraDirection []float64 `json:"camera_direction"`
		ObserverOrigin  []float64 `json:"observer_origin"`
		TargetHead      []float64 `json:"target_head"`
	}
}

func EstimateConcealedSectorForSessions(sessionIDs []string, batches []schema.Batch) SectorResult {
	eligible := map[string]struct{}{}
	for _, id := range sessionIDs {
		eligible[id] = struct{}{}
	}
	var events []sectorEvent
	for _, batch := range batches {
		for _, event := range batch.Events {
			var item sectorEvent
			item.event = event
			if len(event.Payload) > 0 {
				_ = json.Unmarshal(event.Payload, &item.payload)
			}
			events = append(events, item)
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].event.ServerTimeMS == events[j].event.ServerTimeMS {
			return events[i].event.ServerSequence < events[j].event.ServerSequence
		}
		return events[i].event.ServerTimeMS < events[j].event.ServerTimeMS
	})
	var headings, bearings []float64
	for index, item := range events {
		if _, ok := eligible[item.event.PlayerSessionID]; !ok || item.event.EventType != "DECISION_EDGE" || item.payload.SamplingReason != "DELIBERATE_CAMERA_TURN" || len(item.payload.CameraDirection) != 3 {
			continue
		}
		for _, candidate := range events[index+1:] {
			if candidate.event.ServerTimeMS-item.event.ServerTimeMS > 1500 {
				break
			}
			if candidate.event.PlayerSessionID != item.event.PlayerSessionID || candidate.event.EventType != "VISIBILITY_OBSERVATION" || candidate.payload.SamplingStream != "event_enrichment" {
				continue
			}
			if candidate.payload.Classification != "HEAD_ORIGIN_OCCLUDED" && candidate.payload.Classification != "ROBUSTLY_OCCLUDED" {
				continue
			}
			if len(candidate.payload.ObserverOrigin) != 3 || len(candidate.payload.TargetHead) != 3 {
				continue
			}
			headings = append(headings, math.Atan2(item.payload.CameraDirection[0], item.payload.CameraDirection[2]))
			bearings = append(bearings, math.Atan2(candidate.payload.TargetHead[0]-candidate.payload.ObserverOrigin[0], candidate.payload.TargetHead[2]-candidate.payload.ObserverOrigin[2]))
			break
		}
	}
	shifts := make([]float64, 35)
	for index := range shifts {
		shifts[index] = float64(index+1) * 2 * math.Pi / 36
	}
	return CircularPermutation(headings, bearings, shifts)
}
