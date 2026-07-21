CREATE TABLE IF NOT EXISTS schema_migrations (
    version text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS raw_batches (
    server_id text NOT NULL,
    server_session_id text NOT NULL,
    batch_sequence bigint NOT NULL,
    schema_version integer NOT NULL,
    server_time_ms bigint NOT NULL,
    collector_version text NOT NULL DEFAULT '',
    dayz_build text NOT NULL DEFAULT '',
    configuration_hash text NOT NULL DEFAULT '',
    content_sha256 text NOT NULL,
    byte_length bigint NOT NULL,
    first_server_sequence bigint,
    last_server_sequence bigint,
    ingested_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (server_id, server_session_id, batch_sequence)
);

CREATE TABLE IF NOT EXISTS normalized_events (
    source_event_id text PRIMARY KEY,
    server_id text NOT NULL,
    server_session_id text NOT NULL,
    batch_sequence bigint NOT NULL,
    event_type text NOT NULL,
    source text NOT NULL,
    source_authority text NOT NULL,
    source_component text NOT NULL,
    source_schema_version integer NOT NULL,
    collector_version text NOT NULL,
    server_sequence bigint NOT NULL,
    server_time_ms bigint NOT NULL,
    server_receive_ms bigint NOT NULL,
    player_session_id text NOT NULL,
    client_sequence bigint,
    client_monotonic_time_ms bigint,
    payload jsonb NOT NULL,
    normalized_event_schema_version text NOT NULL,
    UNIQUE (server_id, server_session_id, server_sequence)
);

CREATE INDEX IF NOT EXISTS normalized_events_player_time_idx ON normalized_events (player_session_id, server_time_ms);
CREATE INDEX IF NOT EXISTS normalized_events_type_time_idx ON normalized_events (event_type, server_time_ms);

CREATE TABLE IF NOT EXISTS player_sessions (
    player_session_id text PRIMARY KEY,
    server_id text NOT NULL,
    server_session_id text NOT NULL,
    durable_player_id text,
    started_ms bigint NOT NULL,
    ended_ms bigint,
    reconnect_semantics_version text NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS sampling_opportunities (
    opportunity_id text PRIMARY KEY,
    source_event_id text NOT NULL REFERENCES normalized_events(source_event_id),
    observer_player_session_id text NOT NULL,
    target_player_session_id text,
    sampling_stream text NOT NULL,
    sampling_policy_version text NOT NULL,
    sampling_reason text NOT NULL,
    observer_eligible_count integer NOT NULL,
    observer_inclusion_probability double precision NOT NULL,
    target_eligible_count integer NOT NULL,
    target_inclusion_probability double precision NOT NULL,
    risk_set_definition text NOT NULL,
    risk_set_complete boolean NOT NULL,
    queue_admission_probability double precision NOT NULL,
    scheduler_load_state text NOT NULL,
    queue_delay_ms integer NOT NULL,
    drop_reason text NOT NULL
);

CREATE TABLE IF NOT EXISTS visibility_probe_runs (
    source_event_id text PRIMARY KEY REFERENCES normalized_events(source_event_id),
    observer_player_session_id text NOT NULL,
    target_player_session_id text NOT NULL,
    visibility_policy_version text NOT NULL,
    observer_origin_mode text NOT NULL,
    derived_class text NOT NULL,
    blocker_categories text NOT NULL,
    probe_queued_ms bigint NOT NULL,
    probe_started_ms bigint NOT NULL,
    probe_completed_ms bigint NOT NULL,
    raw_evidence jsonb NOT NULL
);

CREATE TABLE IF NOT EXISTS encounters (
    encounter_id text PRIMARY KEY,
    server_session_id text NOT NULL,
    started_ms bigint NOT NULL,
    ended_ms bigint,
    builder_version text NOT NULL
);

CREATE TABLE IF NOT EXISTS observer_target_episodes (
    observer_target_episode_id text PRIMARY KEY,
    encounter_id text NOT NULL REFERENCES encounters(encounter_id),
    observer_player_session_id text NOT NULL,
    target_player_session_id text NOT NULL,
    started_ms bigint NOT NULL,
    ended_ms bigint,
    independence_rule_version text NOT NULL
);

CREATE TABLE IF NOT EXISTS decision_windows (
    decision_window_id text PRIMARY KEY,
    opportunity_id text NOT NULL REFERENCES sampling_opportunities(opportunity_id),
    observer_target_episode_id text REFERENCES observer_target_episodes(observer_target_episode_id),
    refractory_window_id text NOT NULL,
    opened_ms bigint NOT NULL,
    closed_ms bigint NOT NULL,
    outcome_type text NOT NULL,
    outcome_observed boolean NOT NULL,
    observation_builder_version text NOT NULL
);

CREATE TABLE IF NOT EXISTS cue_facts (
    cue_fact_id text PRIMARY KEY,
    observer_target_episode_id text NOT NULL REFERENCES observer_target_episodes(observer_target_episode_id),
    source_event_id text NOT NULL REFERENCES normalized_events(source_event_id),
    cue_type text NOT NULL,
    occurred_ms bigint NOT NULL,
    cue_policy_version text NOT NULL,
    details jsonb NOT NULL
);

CREATE TABLE IF NOT EXISTS analysis_observations (
    observation_id text PRIMARY KEY,
    decision_window_id text NOT NULL REFERENCES decision_windows(decision_window_id),
    feature_family text NOT NULL,
    source_event_ids jsonb NOT NULL,
    cue_class text NOT NULL,
    authority_quality double precision NOT NULL,
    timing_quality double precision NOT NULL,
    observation_builder_version text NOT NULL,
    values jsonb NOT NULL
);

CREATE TABLE IF NOT EXISTS matched_control_sets (
    matched_control_set_id text PRIMARY KEY,
    observation_id text NOT NULL REFERENCES analysis_observations(observation_id),
    control_observation_ids jsonb NOT NULL,
    matching_variables jsonb NOT NULL,
    control_quality double precision NOT NULL,
    matching_policy_version text NOT NULL
);

CREATE TABLE IF NOT EXISTS feature_results (
    feature_result_id text PRIMARY KEY,
    player_session_id text NOT NULL,
    feature_family text NOT NULL,
    feature_algorithm_version text NOT NULL,
    effect double precision,
    lower_bound double precision,
    upper_bound double precision,
    independent_session_count integer NOT NULL,
    independent_encounter_count integer NOT NULL,
    independent_target_count integer NOT NULL,
    raw_counts jsonb NOT NULL,
    diagnostics jsonb NOT NULL
);

CREATE TABLE IF NOT EXISTS candidate_rankings (
    candidate_ranking_id text PRIMARY KEY,
    durable_player_id text NOT NULL,
    ranking_policy_version text NOT NULL,
    review_tier text NOT NULL,
    component_values jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS review_cases (
    review_case_id text PRIMARY KEY,
    candidate_ranking_id text NOT NULL REFERENCES candidate_rankings(candidate_ranking_id),
    status text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS review_dispositions (
    review_disposition_id text PRIMARY KEY,
    review_case_id text NOT NULL REFERENCES review_cases(review_case_id),
    reviewer_id text NOT NULL,
    disposition text NOT NULL,
    notes text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
