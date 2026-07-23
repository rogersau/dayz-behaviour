package main

import (
	"testing"

	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestAnalysisScopeFiltersWholeBatchEvents(t *testing.T) {
	scope := analysisScope{ServerID: "server-a", ServerSessionID: "session-a", MinServerTimeMS: 100, MaxServerTimeMS: 200}
	batch := schema.Batch{
		ServerID: "server-a", ServerSessionID: "session-a",
		Events: []schema.Event{{ServerTimeMS: 50}, {ServerTimeMS: 100}, {ServerTimeMS: 150}, {ServerTimeMS: 201}},
	}
	if !scope.matchesBatch(batch) {
		t.Fatal("matching server/session was rejected")
	}
	filtered := scope.filterEvents(batch)
	if len(filtered.Events) != 2 || filtered.Events[0].ServerTimeMS != 100 || filtered.Events[1].ServerTimeMS != 150 {
		t.Fatalf("unexpected filtered events: %+v", filtered.Events)
	}
	if len(batch.Events) != 4 {
		t.Fatal("scope filtering mutated the immutable source batch")
	}
}

func TestAnalysisScopeRejectsOtherServerSessions(t *testing.T) {
	scope := analysisScope{ServerID: "server-a", ServerSessionID: "session-a"}
	if scope.matchesBatch(schema.Batch{ServerID: "server-b", ServerSessionID: "session-a"}) {
		t.Fatal("other server matched")
	}
	if scope.matchesBatch(schema.Batch{ServerID: "server-a", ServerSessionID: "session-b"}) {
		t.Fatal("other server session matched")
	}
}
