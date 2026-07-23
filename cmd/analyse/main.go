package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"sort"
	"strings"

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
	FeatureAlgorithmVersion   string      `json:"feature_algorithm_version"`
	RankingPolicyVersion      string      `json:"ranking_policy_version"`
	PseudonymPolicyVersion    string      `json:"pseudonym_policy_version"`
	PseudonymKeyID            string      `json:"pseudonym_key_id"`
	ObservationCount          int         `json:"observation_count"`
	Candidates                []candidate `json:"candidates"`
	DataQuality               dataQuality `json:"data_quality"`
	AlgorithmRunID            string      `json:"algorithm_run_id,omitempty"`
}

type candidate struct {
	PlayerPseudonym  string                          `json:"player_pseudonym"`
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
	StrongHiddenObservationCount   int      `json:"strong_hidden_observation_count"`
	NeutralControlObservationCount int      `json:"neutral_control_observation_count"`
	VisiblePositiveControlCount    int      `json:"visible_positive_control_observation_count"`
	DroppedRandomOpportunityCount  int      `json:"dropped_random_opportunity_count"`
	Limitations                    []string `json:"limitations"`
}

func main() {
	rawRoot := flag.String("raw-dir", "./data/raw", "immutable raw batch root")
	databaseURL := flag.String("database-url", os.Getenv("DBA_DATABASE_URL"), "optional PostgreSQL URL for durable analysis output")
	flag.Parse()

	pseudonymPolicy, err := identity.CurrentPolicy()
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

	players := map[string]map[string]struct{}{}
	result := output{
		ObservationBuilderVersion: observations.BuilderVersion,
		FeatureAlgorithmVersion:   features.ReadinessAlgorithmVersion,
		RankingPolicyVersion:      ranking.PolicyVersion,
		PseudonymPolicyVersion:    pseudonymPolicy.Version,
		PseudonymKeyID:            pseudonymPolicy.KeyID,
		ObservationCount:          len(built),
	}
	for _, observation := range built {
		playerID := playerPseudonym(observation.ObserverPlayerSessionID)
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
			PlayerPseudonym:  playerID,
			PlayerSessions:   pseudonymousSessions(sessions),
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
	if pseudonymPolicy.Version == identity.LegacyPolicyVersion {
		result.DataQuality.Limitations = append(result.DataQuality.Limitations,
			"unkeyed development pseudonyms are active; configure DBA_PSEUDONYM_SECRET and DBA_PSEUDONYM_KEY_ID before production collection")
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		log.Fatal(err)
	}
}

func playerPseudonym(playerSessionID string) string {
	value := playerSessionID
	if index := strings.LastIndex(playerSessionID, ":"); index >= 0 {
		value = playerSessionID[index+1:]
	}
	return identity.DurableID(value)
}

func pseudonymousSessions(input []string) []string {
	result := make([]string, 0, len(input))
	for _, value := range input {
		result = append(result, identity.SessionID(value))
	}
	return result
}

func countDroppedRandomOpportunities(batches []schema.Batch) int {
	count := 0
	for _, batch := range batches {
		for _, event := range batch.Events {
			if event.EventType == "SAMPLING_OPPORTUNITY_DROPPED" {
				count++
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
