package review_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/review"
)

func TestAPIRequiresAuthenticationAndRecordsDisposition(t *testing.T) {
	repository := review.NewMemoryRepository([]review.Candidate{{CandidateID: "candidate-1"}}, nil)
	api, err := review.NewAPI(repository, "secret")
	if err != nil {
		t.Fatal(err)
	}

	unauthorised := httptest.NewRecorder()
	api.Handler().ServeHTTP(unauthorised, httptest.NewRequest(http.MethodGet, "/v1/candidates", nil))
	if unauthorised.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", unauthorised.Code)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/cases/case-1/dispositions",
		bytes.NewBufferString(`{"reviewer_id":"reviewer","disposition":"needs_more_context","notes":"uncertain"}`))
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}
