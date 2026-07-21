package explorer

import (
	"context"
	"testing"
)

func TestMemoryTimelineFiltersAndOrders(t *testing.T) {
	from, to := int64(150), int64(250)
	repository := &MemoryRepository{
		Sessions: []Session{{PlayerSessionID: "ps_one", ServerID: "server", StartedMS: 100}},
		Entries: map[string][]Entry{"ps_one": {
			{SourceEventID: "late", ServerTimeMS: 300},
			{SourceEventID: "middle", ServerTimeMS: 200},
			{SourceEventID: "early", ServerTimeMS: 100},
		}},
	}
	timeline, err := repository.Timeline(context.Background(), TimelineQuery{PlayerSessionID: "ps_one", FromMS: &from, ToMS: &to})
	if err != nil {
		t.Fatal(err)
	}
	if len(timeline.Entries) != 1 || timeline.Entries[0].SourceEventID != "middle" {
		t.Fatalf("entries = %#v", timeline.Entries)
	}
}

func TestTimelineExtractsExactAndCoarseLocations(t *testing.T) {
	repository := &MemoryRepository{
		Sessions: []Session{{PlayerSessionID: "ps_one", ServerID: "server", StartedMS: 100}},
		Entries: map[string][]Entry{"ps_one": {
			{SourceEventID: "snapshot", PlayerSessionID: "ps_one", Payload: map[string]any{"position": []any{1200.0, 15.0, 3400.0}}},
			{SourceEventID: "visibility", PlayerSessionID: "ps_one", Payload: map[string]any{"position": []any{0.0, 0.0, 0.0}, "area_cell": "12:34", "map_id": "ChernarusPlus"}},
		}},
	}
	timeline, err := repository.Timeline(context.Background(), TimelineQuery{PlayerSessionID: "ps_one"})
	if err != nil {
		t.Fatal(err)
	}
	if timeline.MapID != "chernarusplus" {
		t.Fatalf("map ID = %q", timeline.MapID)
	}
	if got := timeline.Entries[0].Location; got == nil || got.X != 1200 || got.Z != 3400 || got.AccuracyMetres != 1 {
		t.Fatalf("exact location = %#v", got)
	}
	if got := timeline.Entries[1].Location; got == nil || got.X != 1250 || got.Z != 3450 || got.AccuracyMetres != 71 {
		t.Fatalf("coarse location = %#v", got)
	}
}
