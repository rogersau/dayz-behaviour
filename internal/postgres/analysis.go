package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rogersau/dayz-behaviour/internal/features"
	"github.com/rogersau/dayz-behaviour/internal/observations"
	"github.com/rogersau/dayz-behaviour/internal/ranking"
)

type AnalysisCandidate struct {
	PlayerPseudonym string
	PlayerSessions  []string
	Readiness       features.ReadinessResult
	Matched         features.ConditionalLogitResult
	Stability       features.StabilityResult
	Controls        features.NegativeControlResult
	Sector          features.SectorResult
	PreExposure     features.PreExposureResult
	Decision        ranking.Decision
}

func (s *Store) PersistAnalysis(ctx context.Context, built []observations.Observation, strata []features.MatchedStratum, candidates []AnalysisCandidate) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	for _, observation := range built {
		if _, err = tx.ExecContext(ctx, `INSERT INTO encounters(encounter_id,server_session_id,started_ms,ended_ms,builder_version) VALUES($1,$2,$3,$4,$5) ON CONFLICT(encounter_id) DO UPDATE SET ended_ms=GREATEST(encounters.ended_ms,EXCLUDED.ended_ms)`, observation.EncounterID, observation.ServerSessionID, observation.StartedMS, observation.ClosedMS, observations.BuilderVersion); err != nil {
			return "", err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO observer_target_episodes(observer_target_episode_id,encounter_id,observer_player_session_id,target_player_session_id,started_ms,ended_ms,independence_rule_version) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(observer_target_episode_id) DO UPDATE SET ended_ms=GREATEST(observer_target_episodes.ended_ms,EXCLUDED.ended_ms)`, observation.ObserverTargetEpisodeID, observation.EncounterID, pseudonymousSessionID(observation.ObserverPlayerSessionID), pseudonymousSessionID(observation.TargetPlayerSessionID), observation.StartedMS, observation.ClosedMS, observations.IndependenceVersion); err != nil {
			return "", err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO decision_windows(decision_window_id,opportunity_id,observer_target_episode_id,refractory_window_id,opened_ms,closed_ms,outcome_type,outcome_observed,observation_builder_version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(decision_window_id) DO NOTHING`, observation.DecisionWindowID, observation.OpportunityID, observation.ObserverTargetEpisodeID, observation.RefractoryWindowID, observation.StartedMS, observation.ClosedMS, observation.OutcomeType, observation.OutcomeObserved, observations.BuilderVersion); err != nil {
			return "", fmt.Errorf("persist decision window %s: %w", observation.DecisionWindowID, err)
		}
		for _, cue := range observation.CueFacts {
			details, _ := json.Marshal(map[string]string{"description": cue.Details, "derived_class": observation.CueClass})
			cueID := "cue_" + digest(observation.ObservationID+":"+cue.SourceEventID+":"+cue.CueType)
			if _, err = tx.ExecContext(ctx, `INSERT INTO cue_facts(cue_fact_id,observer_target_episode_id,source_event_id,cue_type,occurred_ms,cue_policy_version,details) VALUES($1,$2,$3,$4,$5,'cue-ledger-v1',$6) ON CONFLICT(cue_fact_id) DO NOTHING`, cueID, observation.ObserverTargetEpisodeID, cue.SourceEventID, cue.CueType, cue.OccurredMS, details); err != nil {
				return "", err
			}
		}
		sourceIDs, _ := json.Marshal(observation.SourceEventIDs)
		values, _ := json.Marshal(observation)
		authorityQuality := 0.8
		if observation.VisibilityAuthority != "A" {
			authorityQuality = 0
		}
		timingQuality := 0.0
		if observation.TimingEligible {
			timingQuality = 1.0
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO analysis_observations(observation_id,decision_window_id,feature_family,source_event_ids,cue_class,authority_quality,timing_quality,observation_builder_version,values) VALUES($1,$2,'hidden-threat-readiness',$3,$4,$5,$6,$7,$8) ON CONFLICT(observation_id) DO UPDATE SET values=EXCLUDED.values`, observation.ObservationID, observation.DecisionWindowID, sourceIDs, observation.CueClass, authorityQuality, timingQuality, observations.BuilderVersion, values); err != nil {
			return "", err
		}
	}
	for _, stratum := range strata {
		for _, observation := range stratum.Observations {
			var controls []string
			for _, other := range stratum.Observations {
				if other.ObservationID != observation.ObservationID && other.ControlEligible != observation.ControlEligible {
					controls = append(controls, other.ObservationID)
				}
			}
			if len(controls) == 0 {
				continue
			}
			controlIDs, _ := json.Marshal(controls)
			variables, _ := json.Marshal(map[string]string{"context_key": stratum.ContextKey})
			id := "match_" + digest(stratum.StratumID+":"+observation.ObservationID)
			if _, err = tx.ExecContext(ctx, `INSERT INTO matched_control_sets(matched_control_set_id,observation_id,control_observation_ids,matching_variables,control_quality,matching_policy_version) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(matched_control_set_id) DO UPDATE SET control_observation_ids=EXCLUDED.control_observation_ids`, id, observation.ObservationID, controlIDs, variables, stratum.ControlQuality, stratum.PolicyVersion); err != nil {
				return "", err
			}
		}
	}
	algorithmMaterial := map[string]any{"builder": observations.BuilderVersion, "matching": features.MatchingPolicyVersion, "readiness": features.ReadinessAlgorithmVersion, "conditional_logit": features.ConditionalLogitVersion, "ranking": ranking.PolicyVersion, "candidates": candidates}
	material, _ := json.Marshal(algorithmMaterial)
	runID := "algorithm_" + digest(string(material))
	versions, _ := json.Marshal(map[string]string{"builder": observations.BuilderVersion, "matching": features.MatchingPolicyVersion, "readiness": features.ReadinessAlgorithmVersion, "conditional_logit": features.ConditionalLogitVersion, "ranking": ranking.PolicyVersion, "negative_controls": features.ValidationAlgorithmVersion})
	if _, err = tx.ExecContext(ctx, `INSERT INTO algorithm_runs(algorithm_run_id,status,algorithm_versions,diagnostics) VALUES($1,'COMPLETED',$2,$3) ON CONFLICT(algorithm_run_id) DO NOTHING`, runID, versions, material); err != nil {
		return "", err
	}
	for _, candidate := range candidates {
		rawCounts, _ := json.Marshal(map[string]int{"hidden_successes": candidate.Readiness.HiddenSuccesses, "hidden_trials": candidate.Readiness.HiddenTrials, "control_successes": candidate.Readiness.ControlSuccesses, "control_trials": candidate.Readiness.ControlTrials})
		diagnostics, _ := json.Marshal(map[string]any{"matched": candidate.Matched, "stability": candidate.Stability, "negative_controls": candidate.Controls})
		featureID := "feature_" + digest(runID+":"+candidate.PlayerPseudonym)
		if _, err = tx.ExecContext(ctx, `INSERT INTO feature_results(feature_result_id,player_session_id,feature_family,feature_algorithm_version,effect,lower_bound,upper_bound,independent_session_count,independent_encounter_count,independent_target_count,raw_counts,diagnostics) VALUES($1,$2,'hidden-threat-readiness',$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(feature_result_id) DO NOTHING`, featureID, candidate.PlayerPseudonym, features.ReadinessAlgorithmVersion, candidate.Readiness.ReadinessLift, candidate.Readiness.ReadinessLiftLowerBound, candidate.Readiness.ReadinessLift, candidate.Readiness.IndependentSessionCount, candidate.Readiness.IndependentEncounterCount, candidate.Readiness.IndependentTargetCount, rawCounts, diagnostics); err != nil {
			return "", err
		}
		sectorCounts, _ := json.Marshal(map[string]int{"samples": candidate.Sector.SampleCount})
		sectorDiagnostics, _ := json.Marshal(candidate.Sector)
		if _, err = tx.ExecContext(ctx, `INSERT INTO feature_results(feature_result_id,player_session_id,feature_family,feature_algorithm_version,effect,independent_session_count,independent_encounter_count,independent_target_count,raw_counts,diagnostics) VALUES($1,$2,'concealed-sector-selection',$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(feature_result_id) DO NOTHING`, "feature_"+digest(runID+":"+candidate.PlayerPseudonym+":sector"), candidate.PlayerPseudonym, features.SectorAlgorithmVersion, candidate.Sector.ObservedConcentration-candidate.Sector.NullMean, candidate.Readiness.IndependentSessionCount, candidate.Readiness.IndependentEncounterCount, candidate.Readiness.IndependentTargetCount, sectorCounts, sectorDiagnostics); err != nil {
			return "", err
		}
		preCounts, _ := json.Marshal(map[string]int{"incidents": candidate.PreExposure.IncidentCount, "pre_exposure": candidate.PreExposure.PreExposureCount})
		preDiagnostics, _ := json.Marshal(candidate.PreExposure)
		if _, err = tx.ExecContext(ctx, `INSERT INTO feature_results(feature_result_id,player_session_id,feature_family,feature_algorithm_version,effect,independent_session_count,independent_encounter_count,independent_target_count,raw_counts,diagnostics) VALUES($1,$2,'pre-exposure-readiness',$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(feature_result_id) DO NOTHING`, "feature_"+digest(runID+":"+candidate.PlayerPseudonym+":pre-exposure"), candidate.PlayerPseudonym, features.PreExposureAlgorithmVersion, candidate.PreExposure.PreExposureRate, candidate.Readiness.IndependentSessionCount, candidate.Readiness.IndependentEncounterCount, candidate.Readiness.IndependentTargetCount, preCounts, preDiagnostics); err != nil {
			return "", err
		}
		components, _ := json.Marshal(map[string]any{"candidate_id": "candidate_" + digest(runID+":"+candidate.PlayerPseudonym), "player_pseudonym": candidate.PlayerPseudonym, "review_priority": candidate.Decision.Tier, "readiness_effect": candidate.Readiness.ReadinessLift, "readiness_lower_bound": candidate.Readiness.ReadinessLiftLowerBound, "sector_selection_effect": candidate.Sector.ObservedConcentration - candidate.Sector.NullMean, "pre_exposure_effect": candidate.PreExposure.PreExposureRate, "independent_session_count": candidate.Readiness.IndependentSessionCount, "independent_encounter_count": candidate.Readiness.IndependentEncounterCount, "independent_target_count": candidate.Readiness.IndependentTargetCount, "authority_quality": 1, "control_quality": controlQuality(candidate.Matched), "telemetry_completeness": 0, "policy_versions": map[string]string{"ranking": ranking.PolicyVersion, "readiness": features.ReadinessAlgorithmVersion, "matching": features.MatchingPolicyVersion, "sector": features.SectorAlgorithmVersion, "pre_exposure": features.PreExposureAlgorithmVersion}, "limitations": []string{"uncaptured audio, external communications and human inference remain possible"}, "incident_ids": incidentIDs(candidate.PlayerSessions, built)})
		candidateID := "candidate_" + digest(runID+":"+candidate.PlayerPseudonym)
		if _, err = tx.ExecContext(ctx, `INSERT INTO candidate_rankings(candidate_ranking_id,durable_player_id,ranking_policy_version,review_tier,component_values) VALUES($1,$2,$3,$4,$5) ON CONFLICT(candidate_ranking_id) DO NOTHING`, candidateID, candidate.PlayerPseudonym, ranking.PolicyVersion, candidate.Decision.Tier, components); err != nil {
			return "", err
		}
		if candidate.Decision.Tier == ranking.Review || candidate.Decision.Tier == ranking.HighPriority {
			caseID := "case_" + digest(candidateID)
			if _, err = tx.ExecContext(ctx, `INSERT INTO review_cases(review_case_id,candidate_ranking_id,status) VALUES($1,$2,'OPEN') ON CONFLICT(review_case_id) DO NOTHING`, caseID, candidateID); err != nil {
				return "", err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return runID, nil
}

func controlQuality(result features.ConditionalLogitResult) float64 {
	if result.Converged && !result.Separated {
		return 1
	}
	if result.UsefulStrata > 0 {
		return .5
	}
	return 0
}
func incidentIDs(sessions []string, built []observations.Observation) []string {
	set := map[string]struct{}{}
	for _, s := range sessions {
		set[s] = struct{}{}
	}
	var ids []string
	for _, o := range built {
		if _, ok := set[o.ObserverPlayerSessionID]; ok && o.StrongHiddenEligible {
			ids = append(ids, o.ObservationID)
		}
	}
	return ids
}
