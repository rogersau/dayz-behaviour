package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/rogersau/dayz-behaviour/internal/cues"
	"github.com/rogersau/dayz-behaviour/internal/features"
	"github.com/rogersau/dayz-behaviour/internal/identity"
	"github.com/rogersau/dayz-behaviour/internal/observations"
	"github.com/rogersau/dayz-behaviour/internal/postgres"
	"github.com/rogersau/dayz-behaviour/internal/ranking"
	"github.com/rogersau/dayz-behaviour/internal/replay"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

type output struct {
	ObservationBuilderVersion string      `json:"observation_builder_version"`
	AudioCueLedgerVersion     string      `json:"audio_cue_ledger_version"`
	FeatureAlgorithmVersion   string      `json:"feature_algorithm_version"`
	RankingPolicyVersion      string      `json:"ranking_policy_version"`
	IdentityPolicyVersion     string      `json:"identity_policy_version"`
	ObservationCount          int         `json:"observation_count"`
	Candidates                []candidate `json:"candidates"`
	DataQuality               dataQuality `json:"data_quality"`
	AlgorithmRunID            string      `json:"algorithm_run_id,omitempty"`
}

type candidate struct {
	PlayerID         string                          `json:"player_id"`
	PlayerSessions   []string                        `json:"player_sessions"`
	Readiness        features.ReadinessResult        `json:"readiness"`
	MatchedModel     features.ConditionalLogitResult `json:"matched_model"`
	Stability        features.StabilityResult        `json:"stability"`
	NegativeControls features.NegativeControlResult  `json:"negative_controls"`
	SectorSelection  features.SectorResult           `json:"sector_selection"`
	PreExposure      features.PreExposureResult      `json:"pre_exposure"`
	ReviewPriority   ranking.Decision                `json:"review_priority"`
}

type dataQuality struct {
	StrongHiddenObservationCount    int      `json:"strong_hidden_observation_count"`
	NeutralControlObservationCount  int      `json:"neutral_control_observation_count"`
	VisiblePositiveControlCount     int      `json:"visible_positive_control_observation_count"`
	DroppedRandomOpportunityCount   int      `json:"dropped_random_opportunity_count"`
	AudioExplanatoryObservationCount int     `json:"audio_explanatory_observation_count"`
	GunshotCueCount                 int      `json:"gunshot_cue_count"`
	FootstepCueCount                int      `json:"footstep_cue_count"`
	Limitations                     []string `json:"limitations"`
}

func main() {
	rawRoot := flag.String("raw-dir", "./data/raw", "immutable raw batch root")
	databaseURL := flag.String("database-url", os.Getenv("DBA_DATABASE_URL"), "optional PostgreSQL URL for durable analysis output")
	flag.Parse()

	identityPolicy, err := identity.CurrentPolicy()
	if err != nil {
		log.Fatal(err)
	}

	var batches []schema.Batch
	_, err = replay.Run(context.Background(), *rawRoot, replay.SinkFunc(func(_ context.Context, batch schema.Batch) error {
		batches = append(batches, batch)
		return nil
	}))
	if err != nil {
		log.Fatal(err)
	}
	built, err := observations.Build(batches, observations.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}
	if err := cues.Enrich(built, batches, cues.DefaultConfig()); err != nil {
		log.Fatal(err)
	}

	players := map[string]map[string]struct{}{}
	result := output{
		ObservationBuilderVersion: observations.BuilderVersion,
		AudioCueLedgerVersion:     cues.LedgerVersion,
		FeatureAlgorithmVersion:   features.ReadinessAlgorithmVersion,
		RankingPolicyVersion:      ranking.PolicyVersion,
		IdentityPolicyVersion:     identityPolicy.Version,
		ObservationCount:          len(built),
	}
	for _, observation := range built {
		playerID := playerIDFromSession(observation.ObserverPlayerSessionID)
		if players[playerID] == nil {
			players[playerID] = map[string]struct{}{}
		}
		players[playerID][observation.ObserverPlayerSessionID] = struct{}{}
		if observation.StrongHiddenEligible {
			result.DataQuality.StrongHiddenObservationCount++
		}
		if observation.ControlEligible {
			result.DataQuality.NeutralControlObservationCount++
		}
		if observation.PositiveControlEligible {
			result.DataQuality.VisiblePositiveControlCount++
		}
		if observation.CueClass == "KNOWN" || observation.CueClass == "PLAUSIBLE" {
			result.DataQuality.AudioExplanatoryObservationCount += countAudioFacts(observation.CueFacts) > 0
		}
		for _, fact := range observation.CueFacts {
			switch fact.CueType {
			case "GUNSHOT_AUDIO_OPPORTUNITY":
				result.DataQuality.GunshotCueCount++
			case "FOOTSTEP_AUDIO_OPPORTUNITY":
				result.DataQuality.FootstepCueCount++
			}
		}
	}
	result.DataQuality.DroppedRandomOpportunityCount = countDroppedRandomOpportunities(batches)

	var playerIDs []string
	for playerID := range players {
		playerIDs = append(playerIDs, playerID)
	}
	sort.Strings(playerIDs)
	matchedStrata := features.BuildMatchedStrata(built)
	negativeControls := features.RunNegativeControls(matchedStrata, 0.05)
	var persistentCandidates []postgres.AnalysisCandidate
	for _, playerID := range playerIDs {
		var sessions []string
		for sessionID := range players[playerID] {
			sessions = append(sessions, sessionID)
		}
		sort.Strings(sessions)
		readiness := features.EstimateReadinessForSessions(playerID, sessions, built, 5, 5)
		matched := features.FitConditionalLogitForSessions(playerID, sessions, matchedStrata)
		stability := features.LeaveOneSessionOut(playerID, sessions, built, 5, 5)
		sector := features.EstimateConcealedSectorForSessions(sessions, batches)
		preExposure := estimatePreExposureForSessions(sessions, built)
		decision := ranking.ApplyValidatedEvidence(readiness, matched, stability, negativeControls, ranking.DefaultGates())
		result.Candidates = append(result.Candidates, candidate{
			PlayerID:         playerID,
			PlayerSessions:   sessions,
			Readiness:        readiness,
			MatchedModel:     matched,
			Stability:        stability,
			NegativeControls: negativeControls,
			SectorSelection:  sector,
			PreExposure:      preExposure,
			ReviewPriority:   decision,
		})
		persistentCandidates = append(persistentCandidates, postgres.AnalysisCandidate{PlayerPseudonym: playerID, PlayerSessions: sessions, Readiness: readiness, Matched: matched, Stability: stability, Controls: negativeControls, Sector: sector, PreExposure: preExposure, Decision: decision})
	}
	if *databaseURL != "" {
		ctx := context.Background()
		store, err := postgres.Open(ctx, *databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
		if err := store.Migrate(ctx); err != nil {
			log.Fatal(err)
		}
		for _, batch := range batches {
			if err := store.Accept(ctx, batch); err != nil {
				log.Fatal(err)
			}
		}
		runID, err := store.PersistAnalysis(ctx, built, matchedStrata, persistentCandidates)
		if err != nil {
			log.Fatal(err)
		}
		result.AlgorithmRunID = runID
	}
	if result.DataQuality.StrongHiddenObservationCount == 0 {
		result.DataQuality.Limitations = append(result.DataQuality.Limitations,
			"no validated first-person robust-occlusion observations; no hidden-awareness effect can be promoted")
	}
	if result.DataQuality.NeutralControlObservationCount == 0 {
		result.DataQuality.Limitations = append(result.DataQuality.Limitations,
			"no neutral no-relevant-target controls were available; readiness lift cannot be estimated")
	}
	if len(built) == 0 {
		result.DataQuality.Limitations = append(result.DataQuality.Limitations,
			"no random prospective opportunities were available")
	}
	result.DataQuality.Limitations = append(result.DataQuality.Limitations,
		"audio cues are server-derived audibility opportunities, not proof that a player heard a sound")

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		log.Fatal(err)
	}
}

func playerIDFromSession(playerSessionID string) string {
	if index := strings.LastIndex(playerSessionID, ":"); index >= 0 {
		return playerSessionID[index+1:]
	}
	return playerSessionID
}

func countAudioFacts(input []observations.CueFact) int {
	count := 0
	for _, fact := range input {
		if fact.CueType == "GUNSHOT_AUDIO_OPPORTUNITY" || fact.CueType == "FOOTSTEP_AUDIO_OPPORTUNITY" {
			count++
		}
	}
	return count
}

func countDroppedRandomOpportunities(batches []schema.Batch) int {
	count := 0
	for _, batch := range batches {
		for _, event := range batch.Events {
			if event.EventType == "SAMPLING_OPPORTUNITY_DROPPED" {
				count++
			}
		}
	}
	return count
}

func estimatePreExposureForSessions(sessionIDs []string, built []observations.Observation) features.PreExposureResult {
	eligible := map[string]struct{}{}
	for _, id := range sessionIDs {
		eligible[id] = struct{}{}
	}
	var incidents []features.PreExposureIncident
	for _, observation := range built {
		if _, ok := eligible[observation.ObserverPlayerSessionID]; !ok || observation.FirstExposureMS == 0 || !observation.StrongHiddenEligible || !observation.Independent || observation.CueClass != "UNEXPLAINED_IN_CAPTURED_DATA" {
			continue
		}
		incidents = append(incidents, features.PreExposureIncident{
			ReadinessLowerMS: observation.OutcomeLowerMS,
			ReadinessUpperMS: observation.OutcomeUpperMS,
			ExposureLowerMS:  observation.FirstExposureLowerMS,
			ExposureUpperMS:  observation.FirstExposureUpperMS,
			Censored:         observation.ExposureWindowCensored,
		})
	}
	return features.EstimatePreExposure(incidents)
}
