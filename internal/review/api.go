package review

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Candidate struct {
	CandidateID               string            `json:"candidate_id"`
	PlayerID                  string            `json:"player_id"`
	ReviewPriority            string            `json:"review_priority"`
	ReadinessEffect           float64           `json:"readiness_effect"`
	ReadinessLowerBound       float64           `json:"readiness_lower_bound"`
	SectorSelectionEffect     float64           `json:"sector_selection_effect"`
	PreExposureEffect         float64           `json:"pre_exposure_effect"`
	IndependentSessionCount   int               `json:"independent_session_count"`
	IndependentEncounterCount int               `json:"independent_encounter_count"`
	IndependentTargetCount    int               `json:"independent_target_count"`
	AuthorityQuality          float64           `json:"authority_quality"`
	ControlQuality            float64           `json:"control_quality"`
	TelemetryCompleteness     float64           `json:"telemetry_completeness"`
	PolicyVersions            map[string]string `json:"policy_versions"`
	Limitations               []string          `json:"limitations"`
	IncidentIDs               []string          `json:"incident_ids"`
	ServerID                  string            `json:"server_id,omitempty"`
	CameraMode                string            `json:"camera_mode,omitempty"`
	SquadStatus               string            `json:"squad_status,omitempty"`
	FeatureFamily             string            `json:"feature_family,omitempty"`
	CreatedAt                 time.Time         `json:"created_at,omitempty"`
}

type Incident struct {
	IncidentID         string           `json:"incident_id"`
	CandidateID        string           `json:"candidate_id"`
	SourceEventIDs     []string         `json:"source_event_ids"`
	VisibilityEvidence map[string]any   `json:"visibility_evidence"`
	CueFacts           []map[string]any `json:"cue_facts"`
	TimingUncertainty  map[string]any   `json:"timing_uncertainty"`
	MatchedControls    []map[string]any `json:"matched_controls"`
	UncertaintyNotes   []string         `json:"uncertainty_notes"`
}

type Disposition struct {
	CaseID      string    `json:"case_id"`
	ReviewerID  string    `json:"reviewer_id"`
	Disposition string    `json:"disposition"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
}

type Case struct {
	CaseID      string    `json:"case_id"`
	CandidateID string    `json:"candidate_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type ReplayRun struct {
	RunID       string    `json:"run_id"`
	Status      string    `json:"status"`
	RequestedBy string    `json:"requested_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type AlgorithmRun struct {
	RunID             string            `json:"run_id"`
	Status            string            `json:"status"`
	AlgorithmVersions map[string]string `json:"algorithm_versions"`
	Diagnostics       map[string]any    `json:"diagnostics"`
	CreatedAt         time.Time         `json:"created_at"`
}

type Repository interface {
	ListCandidates() ([]Candidate, error)
	GetCandidate(string) (Candidate, bool, error)
	GetCase(string) (Case, bool, error)
	GetIncident(string) (Incident, bool, error)
	AddDisposition(Disposition) error
	CreateReplayRun(ReplayRun) error
	GetAlgorithmRun(string) (AlgorithmRun, bool, error)
}

type MemoryRepository struct {
	mu            sync.RWMutex
	candidates    map[string]Candidate
	incidents     map[string]Incident
	cases         map[string]Case
	replayRuns    map[string]ReplayRun
	algorithmRuns map[string]AlgorithmRun
	dispositions  []Disposition
}

func NewMemoryRepository(candidates []Candidate, incidents []Incident) *MemoryRepository {
	repository := &MemoryRepository{candidates: map[string]Candidate{}, incidents: map[string]Incident{}, cases: map[string]Case{}, replayRuns: map[string]ReplayRun{}, algorithmRuns: map[string]AlgorithmRun{}}
	for _, candidate := range candidates {
		repository.candidates[candidate.CandidateID] = candidate
	}
	for _, incident := range incidents {
		repository.incidents[incident.IncidentID] = incident
	}
	return repository
}

func (r *MemoryRepository) ListCandidates() ([]Candidate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Candidate, 0, len(r.candidates))
	for _, candidate := range r.candidates {
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CandidateID < result[j].CandidateID })
	return result, nil
}

func (r *MemoryRepository) GetCandidate(id string) (Candidate, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.candidates[id]
	return value, ok, nil
}

func (r *MemoryRepository) GetCase(id string) (Case, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.cases[id]
	return value, ok, nil
}

func (r *MemoryRepository) GetIncident(id string) (Incident, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.incidents[id]
	return value, ok, nil
}

func (r *MemoryRepository) CreateReplayRun(value ReplayRun) error {
	if value.RunID == "" || value.RequestedBy == "" {
		return errors.New("run_id and requested_by are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replayRuns[value.RunID] = value
	return nil
}

func (r *MemoryRepository) GetAlgorithmRun(id string) (AlgorithmRun, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.algorithmRuns[id]
	return value, ok, nil
}

func (r *MemoryRepository) AddDisposition(value Disposition) error {
	if value.CaseID == "" || value.ReviewerID == "" || value.Disposition == "" {
		return errors.New("case_id, reviewer_id and disposition are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dispositions = append(r.dispositions, value)
	return nil
}

type API struct {
	repository Repository
	token      string
	mux        *http.ServeMux
}

func NewAPI(repository Repository, token string) (*API, error) {
	if repository == nil {
		return nil, errors.New("review repository is required")
	}
	api := &API{repository: repository, token: token, mux: http.NewServeMux()}
	api.routes()
	return api, nil
}

func (a *API) Handler() http.Handler {
	return a.authorise(a.mux)
}

func (a *API) routes() {
	a.mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'")
		_, _ = w.Write([]byte(reviewHTML))
	})
	a.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	a.mux.HandleFunc("GET /v1/candidates", func(w http.ResponseWriter, _ *http.Request) {
		values, err := a.repository.ListCandidates()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "repository unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"candidates": values})
	})
	a.mux.HandleFunc("GET /v1/review-candidates", a.listCandidates)
	a.mux.HandleFunc("GET /v1/candidates/{id}", func(w http.ResponseWriter, request *http.Request) {
		value, ok, err := a.repository.GetCandidate(request.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "repository unavailable"})
			return
		}
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "candidate not found"})
			return
		}
		writeJSON(w, http.StatusOK, value)
	})
	a.mux.HandleFunc("GET /v1/review-candidates/{id}", a.getCandidate)
	a.mux.HandleFunc("GET /v1/review-cases/{id}", func(w http.ResponseWriter, request *http.Request) {
		value, ok, err := a.repository.GetCase(request.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "repository unavailable"})
			return
		}
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "case not found"})
			return
		}
		writeJSON(w, http.StatusOK, value)
	})
	a.mux.HandleFunc("GET /v1/incidents/{id}", func(w http.ResponseWriter, request *http.Request) {
		value, ok, err := a.repository.GetIncident(request.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "repository unavailable"})
			return
		}
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "incident not found"})
			return
		}
		writeJSON(w, http.StatusOK, value)
	})
	a.mux.HandleFunc("GET /v1/review-incidents/{id}", a.getIncident)
	a.mux.HandleFunc("POST /v1/cases/{id}/dispositions", func(w http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(w, request.Body, 64*1024)
		var value Disposition
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&value); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid disposition"})
			return
		}
		value.CaseID = request.PathValue("id")
		value.CreatedAt = time.Now().UTC()
		if err := a.repository.AddDisposition(value); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, value)
	})
}

func (a *API) authorise(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/healthz" || request.URL.Path == "/" {
			next.ServeHTTP(w, request)
			return
		}
		if a.token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication is not configured"})
			return
		}
		value := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(value), []byte(a.token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, request)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func parseLimit(request *http.Request, fallback, maximum int) int {
	value, err := strconv.Atoi(request.URL.Query().Get("limit"))
	if err != nil || value <= 0 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}
