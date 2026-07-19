# Behavioural Awareness Implementation and Analytics Plan

**Status:** Proposed execution plan
**Applies to:** [`behavioural-awareness-spec.md`](./behavioural-awareness-spec.md)
**Repository:** `rogersau/dayz-behaviour`
**Purpose:** Convert the behavioural-awareness specification into an implementable, testable, statistically defensible delivery plan.

This document is deliberately additive. It should be merged alongside the existing specification and current implementation work rather than replacing either. Where code already exists, the first task is to map this plan to the implemented components and amend issues accordingly; duplicate implementations should not be created.

## 1. Executive decision

Build the system as a **manual-review prioritisation and evidence platform**, not as an automated cheat verdict.

The implementation should retain the existing three-part architecture:

1. a required DayZ client collector for local camera and view-state telemetry that the dedicated server cannot reliably observe;
2. a DayZ server collector for authoritative world state, lifecycle and combat events, bounded live-world visibility probes, identity binding, buffering and export;
3. an external Go service for durable ingestion, replay, observation construction, matched controls, behavioural modelling, outlier ranking and review evidence.

The following changes are required before the telemetry schema is treated as stable:

- introduce **random prospective opportunity sampling** in addition to event-triggered evidence enrichment;
- distinguish **first-person visibility evidence** from third-person head-origin occlusion;
- record sampling probabilities, queue admission and load-shedding decisions;
- capture high-value decision edges with bounded low-latency RPCs;
- define authority tiers and observation independence explicitly;
- retain visibility and cue evidence as vectors rather than collapsing them prematurely;
- make the first operational analysis use matched, explainable models with shrinkage and uncertainty;
- defer route pursuit, avoidance, squad propagation and general anomaly detection until the core observation design is validated.

The first operational release should score only:

1. hidden-threat readiness lift;
2. concealed-sector selection;
3. pre-exposure readiness.

Strong hidden-visibility evidence should initially be limited to validated **first-person** observations. Third-person evidence can still be collected and displayed, but should not create the highest review priority until a conservative camera-envelope model is proven.

## 2. Outcomes and non-goals

### 2.1 Required outcomes

The system must:

- identify repeated behavioural responses to concealed players after controlling for context;
- make the sampling process statistically inspectable and replayable;
- preserve the raw events and policy versions used to construct every observation;
- express uncertainty, missing cues and plausible legitimate explanations;
- rank cases for human review using repeated evidence across sessions, encounters and targets;
- expose the strongest incidents, matched controls and data-quality limitations;
- remain bounded under representative DayZ server load;
- continue collecting safely through sidecar outages;
- support model replacement and historical replay without recollecting telemetry.

### 2.2 Explicit non-goals

The system must not:

- automatically ban, kick, shadow-ban or otherwise affect gameplay;
- claim to prove the existence of a DMA device, radar or ESP;
- call an output a `cheat_probability`;
- infer exact human sight or hearing from server geometry;
- treat absent telemetry as evidence of cheating;
- treat third-person head-origin occlusion as equivalent to camera occlusion;
- run population-wide statistical analysis in the DayZ process;
- use a black-box anomaly detector as the primary ranking method;
- allow one incident, one target or one session to create a high-priority case.

## 3. Integration with work already in progress

Another implementation stream is already modifying the repository. Integration should therefore begin with a short reconciliation pass rather than immediately adding parallel components.

### 3.1 Repository reconciliation

Create an implementation inventory containing:

| Area | Expected capability | Current status | Existing path | Gap | Owner |
|---|---|---|---|---|---|
| Client collector | accumulator sampling | unknown | inventory first | assess | assign |
| Client collector | transition detector | unknown | inventory first | assess | assign |
| Client-to-server | versioned RPC batches | unknown | inventory first | assess | assign |
| Server collector | identity binding | unknown | inventory first | assess | assign |
| Server collector | authoritative snapshots | unknown | inventory first | assess | assign |
| Server collector | combat/lifecycle events | unknown | inventory first | assess | assign |
| Server collector | probe scheduler | unknown | inventory first | assess | assign |
| Export | async HTTP and spool | unknown | inventory first | assess | assign |
| Go service | durable raw ingestion | unknown | inventory first | assess | assign |
| Go service | replay | unknown | inventory first | assess | assign |
| Analytics | observation builder | unknown | inventory first | assess | assign |
| Analytics | primary models | unknown | inventory first | assess | assign |
| Review | evidence API | unknown | inventory first | assess | assign |

For every existing component:

1. identify the implemented contract and tests;
2. compare it with the schema and invariants in this plan;
3. retain working code where it meets the invariant;
4. open a focused migration issue where it does not;
5. avoid renaming public types or paths solely to match this document;
6. add compatibility adapters when concurrent work has already established a useful interface.

### 3.2 Conflict-minimising merge policy

- Add new schema fields compatibly before making them required.
- Version sampling, visibility, cue and feature policies independently.
- Keep raw event ingestion backward-compatible for at least one schema generation.
- Put migrations and replay tests in the same pull request as schema changes.
- Do not combine collector refactoring, statistical-model changes and UI work in one pull request.
- Use deterministic fixtures to prove that old raw events can still be replayed.

## 4. Changes required before schema freeze

| Priority | Change | Reason | Completion evidence |
|---|---|---|---|
| P0 | Split random opportunity sampling from event enrichment | Event-only probing creates outcome-dependent sampling and cannot estimate readiness incidence cleanly | Replay fixture shows both streams and known inclusion probabilities |
| P0 | Record authority and sampling metadata | Derived evidence cannot be interpreted without knowing source trust and selection mechanism | Schema tests reject observations missing required policy metadata |
| P0 | Separate first-person and third-person visibility semantics | A head-origin ray does not reproduce a third-person camera | Controlled visibility confusion matrix by camera mode |
| P0 | Define observation independence | Repeated samples from one firefight must not masquerade as independent evidence | Episode/window builder tests with expected counts |
| P0 | Record decision-to-probe timing and clock uncertainty | Visibility may change between a client decision and server probe | Timing fixture and latency budget report |
| P1 | Retain multi-point visibility evidence | A ternary label hides partial exposure and ambiguous blockers | Probe fixtures preserve point and blocker results |
| P1 | Retain cue vectors and last-known-position uncertainty | Early cue collapse loses legitimate explanations | Cue replay test reproduces derived cue class |
| P1 | Add model shrinkage and stability checks | Sparse players otherwise dominate rankings | Small-sample fixture shrinks toward cohort and fails the evidence gate |
| P2 | Add changepoint and general anomaly discovery | Useful after reliable primary effects exist | Offline validation only; cannot create a case alone |
| P2 | Add route and squad models | Requires stronger map and communication assumptions | Separate research milestone |

## 5. Target architecture and ownership boundaries

```text
DayZ client mod
  local player sampler
  sampled state-transition detector
  bounded edge-event sender
  bounded sample ring buffer
  versioned ScriptRPC batches
            |
            v
DayZ server mod
  authenticated sender binding
  authoritative player snapshots
  combat and lifecycle events
  random opportunity scheduler
  event-enrichment scheduler
  bounded visibility probe workers
  async exporter and rotated disk spool
            |
            v
Go service
  authenticated ingestion
  immutable raw batch store
  normalization and clock alignment
  session/encounter/episode reconstruction
  cue and visibility interpretation
  prospective observation builder
  matched controls and models
  review candidate/evidence API
            |
            v
Reviewer
  ranked cases
  incident timelines and controls
  uncertainty and data-health details
  human disposition only
```

### 5.1 DayZ client responsibilities

The client mod owns only local, bounded collection:

- current camera position and direction;
- weapon-raised, ironsight, optics and camera-mode state;
- local sampled transitions;
- same-frame client shot marker where the verified weapon seam permits it;
- monotonic client sequence and time;
- bounded ring buffer and batch serialization;
- bounded, coalesced high-value edge events;
- telemetry-health counters.

It must not enumerate remote players, infer visibility, build controls, calculate features or contain a secret that is assumed tamper-proof.

### 5.2 DayZ server responsibilities

The server mod owns:

- binding incoming RPCs to the actual server-supplied `PlayerIdentity`;
- schema, size, sequence, nonce and rate validation;
- server session IDs and server sequences;
- authoritative player position, orientation, lifecycle, hit and death events;
- server-observable movement transitions;
- server shot markers only where the dedicated-server hook is proven reliable;
- random opportunity and event-enrichment scheduling;
- bounded candidate construction and live-world visibility probes;
- queue, probe, serialization, export and spool metrics;
- asynchronous export and replay of rotated spool files.

It must not query a database, calculate lifetime statistics, run cohort analysis or create a punishment decision.

### 5.3 Go service responsibilities

The Go service owns:

- authenticated ingestion and durable raw persistence before acknowledgement;
- idempotency, schema migration and replay;
- event normalization and clock alignment;
- sessions, encounters, target episodes and decision-window construction;
- matched controls and data-quality assessment;
- feature extraction, effect estimation, shrinkage and stability testing;
- candidate ranking and evidence packaging;
- retention, deletion, access control and review audit records.

## 6. Authority and trust model

Every raw and derived value must carry a source authority.

| Tier | Meaning | Examples | Permitted role |
|---|---|---|---|
| A | Server-authoritative or server-observed | positions, movement, hits, deaths, verified server shot, server raycasts, server receipt time | Can support primary evidence |
| B | Client-supplied supporting telemetry | camera direction, optics, third-person state, client shot, raised state if not proven server-observable | Can add context or corroboration; cannot independently create highest priority |
| C | Telemetry integrity and health | missing batches, sequence gaps, clock drift, implausible camera offset, buffer overflow | Changes confidence and reviewer context only |

Rules:

1. A high-priority case must contain repeated Tier A-supported observations.
2. Tier B data may explain behaviour, refine angular features or lower confidence.
3. Missing or implausible Tier B data must not increase hidden-awareness evidence.
4. Tier C signals are never DMA/radar indicators on their own.
5. A client-reported camera position that makes a target plausibly visible can reduce or suppress an incident; it must never turn an otherwise ambiguous incident into stronger hidden evidence.

Recommended common fields:

```text
source_authority
source_component
source_event_id
source_schema_version
collector_version
server_id
server_session_id
player_session_id
server_sequence
client_sequence where applicable
client_monotonic_ms where applicable
server_receive_ms
server_event_ms where applicable
dayz_build
dayz_scripts_revision
script_diff_commit
configuration_hash
```

## 7. DayZ capability and performance spike

Before implementation relies on a script surface, prove its execution context on the target DayZ build.

### 7.1 Capability matrix

Produce a checked-in matrix with at least these columns:

| Capability | Client local | Server owner | Server remote | Dedicated server | Resulting authority | Notes |
|---|---:|---:|---:|---:|---|---|
| `MissionGameplay.OnUpdate` | test | n/a | n/a | n/a | B | accumulator only |
| `MissionServer.OnUpdate` | n/a | n/a | n/a | test | A | bounded queue drain |
| current camera transform | test | test | test | test | likely B | validate locality |
| `IsFireWeaponRaised` | test | test | test | test | highest verified tier | remote replication is important |
| ironsights/optics | test | test | test | test | likely B | do not assume |
| third-person state | test | test | test | test | likely B | do not assume |
| `Weapon_Base.OnFire` | test | test | test | test | conditional | verify duplication and ownership |
| `Weapon_Base.EEFired` | test | test | test | test | conditional | test as alternate server marker |
| `EEHitBy` | test | test | test | test | A | verify source resolution |
| `EEKilled` | test | test | test | test | A | lifecycle anchor |
| bone positions by stance | test | test | test | test | A where server-owned | include animation transitions |
| `RaycastRV`/`RaycastRVProxy` | test | test | test | test | A | identify blocker semantics |
| async `RestContext.POST` | n/a | n/a | n/a | test | A transport | prohibit blocking call |
| spool write/replay | n/a | n/a | n/a | test | A transport | rotation and corruption tests |

### 7.2 Controlled hook tests

For each hook, record:

- invocation count per logical event;
- client, listen-server and dedicated-server execution;
- whether the entity is local owner or remote proxy;
- timing relative to native action completion;
- whether calling `super` is required;
- duplicate and reconnect behaviour;
- target DayZ build and script revision.

### 7.3 Performance spike

Measure at representative player counts:

- client sample CPU time and serialized bytes per second;
- RPC messages and bytes per player per minute;
- server snapshot time;
- candidate-construction time;
- raycast latency by blocker class;
- queue depth and wait time;
- serialization and asynchronous dispatch time;
- spool throughput and recovery time;
- gameplay tick percentiles with collection disabled and enabled.

No permanent probe budget should be chosen until these measurements exist.

## 8. Sampling design

### 8.1 Why event-only probing is insufficient

When probes are created only after a meaningful decision, the captured data primarily estimates:

```text
P(hidden target | decision occurred)
```

The primary behavioural question requires:

```text
P(decision occurs | hidden target, context)
```

Those quantities are not interchangeable. Matching after an event cannot recreate the missing non-event opportunities. The server therefore needs two explicitly labelled sampling streams.

### 8.2 Stream A: random prospective opportunities

Purpose: provide an inference-capable denominator for readiness and related behaviour.

For each active observer:

1. Schedule opportunity triggers independently of their current decision behaviour.
2. Use exponential inter-arrival times or equivalent randomized jitter, not a synchronized fixed interval.
3. At a trigger, construct the configured eligible target risk set without using the observer's subsequent camera turn, route change or readiness outcome.
4. Select candidates using a versioned policy with known inclusion probabilities.
5. Perform bounded visibility probes.
6. Open a short prospective decision window.
7. Record whether a configured outcome occurs after the opportunity begins.
8. Close the window on timeout, death, disconnect or invalidation.

Initial outcome windows should be validated experimentally; candidate development values are two to five seconds, not production thresholds.

Potential outcomes:

```text
weapon_raised
ads_entered
deliberate_camera_turn
target_sector_selected
movement_stopped
route_heading_changed
shot_fired
```

A single opportunity may contain several correlated outcomes, but it remains one primary opportunity for independence and clustering.

### 8.3 Stream B: event-enrichment observations

Purpose: collect detailed evidence immediately after high-value decisions.

Triggers may include:

- weapon raise;
- ADS/optics entry;
- deliberate camera turn;
- large server-derived route change;
- significant stop or slowdown;
- shot, hit or kill;
- transition into a diagnostic pair window.

The server may prioritize salient target pairs in this stream using distance, time to contact, prior exposure transition, route intersection or active engagement context. Because selection is influenced by an observed decision, this stream must not be used as an unbiased readiness denominator unless its selection mechanism is explicitly modelled.

Stream B is appropriate for:

- incident evidence;
- conditional questions such as which sector was selected after a turn;
- short visibility timelines around exposure;
- post-engagement handoff evidence;
- debugging and calibration.

### 8.4 Required sampling metadata

Every opportunity or enriched observation must record:

```text
sampling_stream
sampling_policy_version
sampling_reason
observer_eligible_count
observer_inclusion_probability
target_eligible_count
target_inclusion_probability
risk_set_definition
risk_set_complete
queue_admission_probability
scheduler_load_state
queue_delay_ms
drop_reason
```

Where unequal inclusion probabilities are unavoidable, the analysis may use design weights or inverse-probability weighting. Prefer bounded sampling designs with reasonably uniform probabilities because extreme weights create unstable estimates. Any weighted analysis must expose effective sample size and weight distribution.

### 8.5 Candidate selection for inference

The inference policy must be independent of the outcome being measured.

Recommended approach:

- define configurable distance bands;
- select a bounded number of candidates from non-empty bands using deterministic random seeds recorded with the observation;
- avoid selecting candidates based on the observer's chosen view sector;
- record the complete candidate count and inclusion probability in each band;
- treat a control as fully observed only when the configured risk set has been sufficiently classified;
- otherwise mark it `PARTIALLY_OBSERVED` and either exclude it from primary analysis or use an explicitly design-weighted method.

Distance bands must be derived from profiling and gameplay calibration, not embedded as cheat rules.

### 8.6 Load shedding

Reserve separate budgets for the two streams.

Under pressure:

1. retain authoritative combat and lifecycle events;
2. retain already-admitted random opportunities where possible;
3. reduce or drop event-enrichment diagnostic probes;
4. reduce client background sample frequency;
5. drop low-value camera samples before authoritative events;
6. record every admission and drop decision.

The inference stream must not be selectively dropped based on whether the target later appears suspicious. If inference work must be reduced, use a randomized or uniformly applied admission rule and record its probability.

## 9. Client collector implementation

### 9.1 State sampler

Use an accumulator in `MissionGameplay.OnUpdate` so collection cadence does not depend on frame rate.

Suggested configurable modes:

| Mode | Purpose | Candidate development cadence |
|---|---|---:|
| Baseline | ordinary movement and view context | 2 Hz |
| Ready | weapon raised, ironsights or optics | up to 10 Hz |
| Diagnostic | shot, hit, server request or active incident window | up to 10 Hz |
| Inactive | dead, unconscious, menu or not in gameplay | lifecycle only or reduced |

The final rates are performance-test outputs, not fixed detection thresholds.

Each state sample should contain:

```text
client_schema_version
client_sequence
client_monotonic_ms
camera_position
camera_direction
player_position_client_observed
is_weapon_raised
is_in_ironsights
is_in_optics
is_in_third_person
item_in_hands_type_id
sample_mode
sample_flags
```

Quantization should be versioned and tested for its impact on angular and timing features.

### 9.2 Pure transition detector

Implement a side-effect-free component that consumes successive sampled states and emits transitions. It must be testable without engine calls.

Required transitions:

```text
WEAPON_RAISED
WEAPON_LOWERED
ADS_ENTERED
ADS_EXITED
OPTICS_ENTERED
OPTICS_EXITED
CAMERA_MODE_CHANGED
DELIBERATE_CAMERA_TURN
SHOT_FIRED_CLIENT
```

Camera turns should be detected from angular displacement over a configured interval with hysteresis and a refractory period. Avoid per-sample turn events during one continuous sweep.

### 9.3 Low-latency decision-edge RPC

Normal batches may wait up to the configured batch interval. That delay is too large for high-value visibility reconstruction.

Add a compact `DECISION_EDGE` RPC for:

- weapon raised;
- ADS/optics entered;
- deliberate camera turn;
- client successful-fire marker.

Controls:

- per-identity token bucket;
- coalescing of repeated equivalent transitions;
- maximum serialized size;
- minimum event separation;
- normal batch still contains the durable copy;
- edge messages are advisory and may be dropped;
- server receipt time is authoritative.

### 9.4 Batching and health

- Use a bounded ring buffer.
- Batch at most at the configured interval.
- Cap sample count and bytes per batch.
- Include the most recent server-issued session nonce.
- Never block waiting for acknowledgement.
- Track dropped samples, ring overflow, rejected batches and last successful send.
- Do not treat health failures as hidden-awareness evidence.

## 10. Server collector implementation

### 10.1 RPC validation

A pure validation seam should accept sender identity, schema, nonce, sequence, sizes and metadata and return a structured accept/reject result.

Validation includes:

- sender identity from the server receive path;
- supported schema range;
- expected session nonce;
- monotonic sequence with bounded duplicate handling;
- maximum batch samples and bytes;
- vector and numeric sanity checks;
- per-identity rate and byte limits;
- explicit rejection reason and metric.

Client-supplied player identifiers are ignored for attribution.

### 10.2 Authoritative snapshots

At a configurable low rate, record:

```text
server_sequence
server_event_ms
player_session_id
position
orientation
alive
unconscious
item_in_hands_type_id where available
movement_state where verified
```

Derive speed, movement heading, stop/start and significant route changes from successive authoritative positions. Require sufficient displacement before accepting a heading change.

### 10.3 Combat and lifecycle events

Capture:

- connect, ready, respawn, reconnect and disconnect;
- server shot marker where a reliable hook is proven;
- separate client shot marker;
- hit source/root shooter where resolvable;
- victim, component, damage zone, ammunition, weapon and available hit position;
- death and encounter closure anchors.

Server and client shot markers remain separate records and may be correlated later.

### 10.4 Scheduler queues

Maintain separate bounded queues for:

- random opportunities;
- event enrichment;
- active diagnostic pair windows;
- export batches;
- spool replay.

Each queue exposes depth, age, admitted, completed, dropped and failure metrics. Work is drained under a strict per-update budget.

## 11. Timing and clock alignment

Record distinct timestamps rather than replacing one with another:

```text
client_monotonic_event_ms
server_receive_ms
server_event_ms
probe_queued_ms
probe_started_ms
probe_completed_ms
export_queued_ms
clock_offset_estimate_ms
clock_drift_estimate
clock_uncertainty_ms
```

### 11.1 Alignment protocol

Use periodic four-timestamp exchanges:

1. server sends a challenge with server time;
2. client records receive time;
3. client sends response with client receive and send times;
4. server records response time.

Estimate round-trip time and clock offset, retain the lowest-latency samples, and fit drift across the session. The client clock remains untrusted; alignment exists to bound timing uncertainty, not to make client time authoritative.

### 11.2 Feature suppression

A timing-sensitive feature must be suppressed or discounted when:

- edge RPC receipt is too delayed;
- probe queue delay exceeds the validated limit;
- clock uncertainty is too large;
- the observer or target moved materially between event and probe;
- the visibility state changed before the relevant window could be reconstructed.

The acceptable limits are outputs of controlled tests and must be versioned.

## 12. Visibility semantics and probing

### 12.1 Camera-mode distinction

A server ray from the player's head is a reasonable approximation for first-person sight after validation. It is not a valid reconstruction of a third-person camera.

Record an origin mode:

```text
FIRST_PERSON_EYE
PLAYER_HEAD_APPROXIMATION
THIRD_PERSON_CAMERA_ENVELOPE
CLIENT_REPORTED_CAMERA
UNKNOWN_ORIGIN
```

Initial policy:

- permit strong `ROBUSTLY_OCCLUDED` evidence only for validated first-person origins;
- label third-person head-ray results `HEAD_ORIGIN_OCCLUDED` rather than hard-occluded;
- collect third-person incidents but discount or suppress them in primary ranking;
- later validate a conservative third-person camera envelope by stance, action and nearby geometry;
- require every plausible envelope origin to be blocked before considering strong third-person occlusion.

### 12.2 Adaptive multi-point probes

A two-ray head/torso check is useful for an initial exposure test but can overstate full occlusion. Use an adaptive evidence vector.

1. Probe upper torso.
2. Probe head.
3. If either point is clear, classify exposed or partially exposed.
4. If both are hard-blocked and the observation is high-value, probe pelvis and left/right upper-body points.
5. Preserve every point result and blocker category.
6. Require a validated minimum occlusion duration before strong use.

Suggested derived classes:

```text
EXPOSED
PARTIALLY_EXPOSED
ROBUSTLY_OCCLUDED
HEAD_ORIGIN_OCCLUDED
AMBIGUOUS
NOT_PROBED
```

Required evidence fields:

```text
visibility_policy_version
observer_origin_mode
points_attempted
points_clear
points_hard_blocked
points_ambiguous
point_results
first_blocker_categories
dynamic_blocker_present
bone_validity
observer_position_at_probe
target_position_at_probe
probe_queue_delay_ms
probe_duration_ms
occlusion_duration_ms
```

### 12.3 Ambiguity rules

Do not promote to strong occlusion when any required point is invalid or the first contact is ambiguous. Treat these as ambiguous unless controlled tests prove otherwise:

- vegetation and foliage proxies;
- rapidly moving vehicles;
- uncertain modded geometry;
- invalid or transitional bones;
- missing world objects;
- incomplete raycast execution;
- dynamic door state that changed during the event/probe interval.

### 12.4 Controlled validation matrix

Build labelled fixtures covering:

- standing, crouched and prone;
- first-person and third-person;
- lean, freelook and shoulder changes where relevant;
- windows, doors, stairs, rooftops and ridgelines;
- terrain, buildings, base-building objects and vehicles;
- partial head, shoulder, torso and lower-body exposure;
- moving observer, moving target and both moving;
- modded map geometry.

Produce confusion matrices by camera mode, stance and blocker class. Strong evidence requires a conservative false-occlusion rate.

## 13. Cue ledger and legitimate-knowledge model

Keep source cues as individual facts and derive display classes later.

Required cue fields include:

```text
last_direct_visual_exposure_ms
last_direct_visual_position
last_target_shot_ms
shot_distance
shot_suppression_state_or_unknown
last_damage_interaction_ms
last_group_member_contact_ms
last_known_target_position
last_known_position_age_ms
last_known_position_uncertainty_radius
ongoing_engagement
recent_target_visibility_transition
audio_cue_possible
external_communication_unobservable
```

### 13.1 Last-known-position envelope

When a target has been legitimately observed, create a conservative region in which the observer could reasonably expect the target to be.

MVP:

- start from the last known position;
- expand by elapsed time and a conservative feasible movement speed;
- add uncertainty for missing snapshots and route ambiguity;
- discount behaviour directed anywhere inside the envelope.

Later:

- constrain the envelope using a map route graph, passable regions, building entrances and vertical access;
- retain multiple reachable regions rather than one circle where topology matters.

### 13.2 Derived cue classes

The UI may derive:

- `KNOWN`: captured direct visual or combat knowledge explains the response;
- `PLAUSIBLE`: a recorded shot, group interaction, last-known envelope or ongoing engagement may explain it;
- `UNEXPLAINED_IN_CAPTURED_DATA`: no captured cue explains it.

The last class must always be accompanied by text stating that uncaptured audio, external communications and human inference remain possible.

## 14. Observation construction and independence

Raw samples are not independent behavioural observations. The Go service must build versioned analysis units.

### 14.1 Required identifiers

```text
encounter_id
observer_target_episode_id
opportunity_id
decision_window_id
refractory_window_id
feature_family
independence_rule_version
```

### 14.2 Session

A session begins after authenticated readiness and ends on disconnect, identity replacement, server restart boundary or a configured invalidation condition. Reconnect handling must preserve a durable account link without silently merging unrelated character lives.

### 14.3 Encounter

An encounter groups players with sustained contact opportunity or combat interaction. It closes on death, long separation, validated timeout or session end.

### 14.4 Observer-target episode

An episode spans continuous opportunity for one observer to respond to one target. It should not restart on every probe. It ends on death, session end, prolonged separation, group reclassification or a configured inactivity timeout.

### 14.5 Decision window

A decision window contains one prospectively defined opportunity and its bounded outcomes. Within a window:

- weapon raise followed immediately by ADS is correlated readiness evidence, not two independent incidents;
- repeated camera samples are one turn episode;
- diagnostic raycasts are state measurements, not new behavioural observations;
- at most one primary outcome per feature family is counted unless the feature explicitly models recurrent events.

### 14.6 Statistical clustering

Confidence intervals and validation resampling must cluster or block by at least session and encounter. Where relevant, also cluster by target and known/inferred squad. Do not bootstrap individual telemetry records.

## 15. Schema, storage and replay

### 15.1 Versioning

Version these independently:

```text
wire_schema_version
normalized_event_schema_version
sampling_policy_version
visibility_policy_version
cue_policy_version
observation_builder_version
feature_algorithm_version
ranking_policy_version
```

Every derived record must include source event IDs and all relevant policy versions.

### 15.2 Raw persistence

Recommended design:

- write immutable, append-only raw batch objects or rotated compressed files partitioned by server and date;
- record content hash, byte length, first/last server sequence and ingestion time in PostgreSQL;
- acknowledge only after durable write succeeds;
- never rewrite raw batches during normalization;
- retain malformed-but-authenticated payload metadata in a quarantine path when safe;
- encrypt transport and restrict raw identifier access.

Compressed NDJSON is acceptable for the first release because it is inspectable and replayable. Columnar snapshots may be generated later for analytical efficiency without replacing raw events.

### 15.3 Normalized PostgreSQL model

Suggested logical tables:

```text
servers
server_sessions
player_identities
player_sessions
raw_batches
normalized_events
player_snapshots
combat_events
client_state_samples
decision_edges
sampling_opportunities
candidate_selections
visibility_probe_runs
visibility_point_results
cue_facts
encounters
observer_target_episodes
decision_windows
analysis_observations
matched_control_sets
feature_results
candidate_rankings
review_cases
review_incidents
review_dispositions
algorithm_runs
```

### 15.4 Idempotency

Use an idempotency key based on server ID, server session ID and server sequence or batch ID. Replay must be safe to run repeatedly into a clean or partially populated database.

### 15.5 Golden replay fixtures

Check in fixtures for:

- clean normal session;
- duplicate and out-of-order batches;
- reconnect and respawn;
- sidecar outage and spool replay;
- first-person exposed and occluded pairs;
- third-person head-occluded but camera-visible case;
- decision edge with low and high queue delay;
- known cue and unexplained-in-captured-data cases;
- one encounter containing many correlated transitions;
- random opportunity with no outcome;
- event enrichment with high-value incident;
- schema migration from the previous version.

A fixture must produce deterministic normalized records and feature outputs for a fixed algorithm version.

## 16. Primary behavioural features

The primary model asks whether behaviour changes unusually in the presence of concealed players after accounting for context. It does not score ordinary aim accuracy.

### 16.1 Hidden-threat readiness lift

Question:

> Does this player's probability of becoming ready increase when a robustly occluded opponent is present, compared with matched opportunities for the same player and context?

Primary outcomes:

- weapon raised;
- ADS/optics entered;
- significant stop or slowdown;
- route change toward cover or away from the target sector.

Primary inputs:

- Stream A opportunities only, unless an explicitly validated sampling correction is used;
- validated visibility state;
- cue vector and cue class;
- context and control quality;
- authority composition;
- timing uncertainty.

Outputs:

```text
hidden_opportunity_count
control_opportunity_count
hidden_outcome_rate
control_outcome_rate
risk_difference
odds_ratio_or_lift
uncertainty_interval
independent_encounter_count
independent_target_count
session_count
control_quality
leave_one_session_out_range
```

### 16.2 Concealed-sector selection

Question:

> When a deliberate camera or route choice occurs, does the player select the concealed target's direction more accurately than expected from their own scanning behaviour and the available alternatives?

Use circular angular error rather than relying only on fixed sectors:

```text
angular_error = minimum wrapped angle between selected direction and eligible hidden target directions
```

Compare against:

- matched empty opportunities;
- the player's own scanning distribution;
- within-event random rotations;
- geometry-matched alternative sectors;
- total angular width occupied by candidate sectors.

Potential choice-model inputs:

```text
contains_hidden_target
contains_exposed_target
recent_cue_direction
last_known_position_envelope
doorway_or_exit_prior
route_continuation_prior
sector_angular_width
number_of_plausible_choices
```

Do not score an obvious doorway, road continuation or recent-gunfire direction as unexplained without a discount.

### 16.3 Pre-exposure readiness

Question:

> Did the observer prepare for the correct target before that target became exposed, and what event caused exposure?

Record exposure cause:

```text
TARGET_MOVED_INTO_VIEW
OBSERVER_PEEKED_OR_MOVED
DYNAMIC_GEOMETRY_CHANGED
UNKNOWN
```

Target-driven exposure is generally stronger evidence than preparation before the observer deliberately peeks a previously known doorway.

Outputs:

```text
readiness_to_first_exposure_ms
angular_error_at_readiness
exposure_cause
was_target_recently_known
cue_classification
window_censored
visibility_confidence
timing_uncertainty_ms
```

## 17. Statistical and outlier algorithms

### 17.1 MVP: matched risk sets and conditional logistic analysis

Construct matched strata within the same player where practical. Candidate matching variables include:

- server and map;
- approximate area or map cell;
- distance band;
- observer movement and stance;
- baseline weapon/ADS state;
- first-person/third-person mode;
- time since shot, hit or engagement;
- server population band;
- hotspot classification;
- time-of-day/weather identifiers when cheaply available;
- cue state and last-known envelope;
- recent target exposure;
- sampling-policy version.

A conceptual model is:

```text
logit P(Y_pij = 1) = alpha_stratum + beta_p * hidden_pij + gamma' * context_pij
```

Where:

- `Y` is a prospectively observed readiness decision;
- `hidden` identifies a validated concealed-target condition;
- `alpha_stratum` controls the matched opportunity set;
- `beta_p` is the player-specific hidden-state response effect;
- `context` contains remaining covariates.

Use conditional logistic regression when strata contain useful within-stratum variation. Detect separation and sparse strata; do not emit unstable infinite estimates.

### 17.2 MVP fallback: empirical-Bayes beta-binomial

For early data volumes, estimate hidden and control readiness rates with beta-binomial shrinkage.

For each player:

```text
p_hidden  ~ Beta(a0 + hidden_successes,  b0 + hidden_failures)
p_control ~ Beta(a0 + control_successes, b0 + control_failures)
```

Derive posterior distributions for:

- risk difference;
- lift ratio;
- odds ratio;
- probability the effect exceeds a predeclared practical review threshold.

Hyperparameters should be estimated from a relevant cohort or set conservatively and versioned. This prevents a player with three successes from outranking a stable player with a large body of evidence solely because of a raw 100% rate.

### 17.3 Target model: hierarchical logistic partial pooling

After enough data exists, use partial pooling:

```text
beta_player ~ Normal(mu_cohort, tau^2)
```

Potential additional random effects:

- server/map;
- area;
- target;
- session;
- encounter;
- known squad.

The operational output is the estimated hidden-response effect and uncertainty, not a probability that the player cheats.

Production implementation should remain deterministic, versioned and replayable. An offline R or Python validation notebook may be used to verify the Go implementation, but production results must not depend on an untracked notebook state.

### 17.4 Circular permutation and conditional choice models

For sector selection:

1. calculate circular angular error;
2. construct a within-player null from matched scanning opportunities;
3. use random rotations or label permutations that preserve the event's candidate geometry;
4. estimate the probability of observing equal or better target-direction selection;
5. optionally fit a conditional multinomial choice model across plausible sectors.

This avoids fixed-sector boundary artifacts and accounts for events with different numbers of plausible choices.

### 17.5 Discrete-time survival and competing risks

For pre-exposure readiness, model time to exposure in small discrete intervals. Treat exposure causes as competing risks and diagnostic-window expiry as censoring.

This handles:

- different window lengths;
- target-driven versus observer-driven exposure;
- events that never reach exposure;
- repeated state checks without counting them as independent observations.

### 17.6 Robust population ranking

Rank context-adjusted feature estimates, not raw counts.

Use:

- robust cohort median and median absolute deviation;
- lower credible/confidence bounds rather than point estimates alone;
- session/encounter block bootstrap;
- leave-one-session-out stability;
- effect-size threshold and data-quality threshold;
- independent target, encounter and session breadth;
- authority-tier composition;
- control quality and effective sample size.

A high raw percentile with an unstable lower bound should remain low confidence.

### 17.7 Multiple testing controls

Keep a small preregistered primary feature set. Exploratory feature screens may use false-discovery-rate controls such as Benjamini-Hochberg, but an exploratory adjusted p-value must not create a review case without independent evidence and holdout stability.

### 17.8 Phase 2: CUSUM and Bayesian online changepoint detection

Apply drift detection to session-level or weekly hidden-response effects, not raw frames. This can identify a material change in behaviour after an account begins using additional information or changes play style.

Outputs should show:

- likely change interval;
- pre- and post-change effect estimates;
- number of supporting sessions;
- sensitivity to removing the strongest session.

A changepoint is a review aid, not proof.

### 17.9 Phase 2: Isolation Forest

Use Isolation Forest only as a secondary discovery layer over reliable, context-adjusted aggregate features such as:

- readiness effect and uncertainty;
- sector-selection residual;
- pre-exposure hazard residual;
- session stability;
- route-search residual;
- cue-adjusted handoff residual.

It must not consume raw positions or ordinary combat totals as its primary input, and it must not independently create the highest-priority case. Its purpose is to identify unusual combinations that the explainable feature rules may not rank highly.

### 17.10 Phase 3: route graph and hidden/semi-Markov models

For hidden pursuit and avoidance:

- build a map-specific route graph or passable-region model;
- estimate ordinary route priors by area and objective type;
- derive search entropy, route efficiency and target-convergence residuals;
- use a hidden or semi-Markov model for states such as travel, search, track, engage and disengage;
- compare transitions following concealed target movement with matched empty situations.

These features require substantially more data and map-specific validation. They remain supporting evidence.

### 17.11 Phase 3: squad information propagation

Start with lagged permutation tests:

- identify which member first expresses a target-relevant decision;
- measure group convergence or readiness after that event;
- permute member labels and time offsets within comparable encounters;
- compare known groups separately from inferred groups.

Only consider Hawkes-process or network excitation models after the simpler lagged null is demonstrably useful. External voice and Discord communications are unobserved, so squad propagation must always carry lower confidence.

### 17.12 Algorithms not recommended for the first release

Do not apply these directly to raw telemetry:

- k-means or DBSCAN on player positions and counts;
- one-class SVM;
- deep autoencoders;
- black-box gradient-boosted cheat classifiers without credible labels;
- generic aim-accuracy or headshot scoring;
- single-feature z-score thresholds;
- unclustered frame-level significance tests.

These approaches are likely to surface map specialists, skilled PvP players, different control styles and unusual session patterns before isolating hidden-information responsiveness.

## 18. Review ranking policy

Avoid one opaque score. Produce a transparent candidate record with separate dimensions.

Suggested dimensions:

```text
readiness_effect
readiness_lower_bound
sector_selection_effect
pre_exposure_effect
independent_session_count
independent_encounter_count
independent_target_count
authority_quality
control_quality
telemetry_completeness
leave_one_session_out_stability
supporting_advanced_patterns
```

A versioned review-priority policy may convert these into tiers:

- `INSUFFICIENT_DATA`;
- `MONITOR`;
- `REVIEW`;
- `HIGH_PRIORITY_REVIEW`.

The policy should require:

- minimum independent evidence breadth;
- at least one validated primary feature effect;
- a lower-bound or posterior-threshold condition;
- acceptable control and timing quality;
- repeated Tier A support;
- no dependence on a single session or target.

Final numeric gates must be selected only after calibration. Store the component values so changing a tiering policy does not require recalculating raw features.

## 19. Calibration and evaluation

### 19.1 Silent calibration

Run without exposing rankings operationally. Include:

- trusted regular players;
- strong PvP players;
- administrators performing scripted behaviours;
- different maps, population bands and camera modes;
- known groups and solo players;
- ordinary high-alert areas and low-traffic areas.

### 19.2 Controlled information intervention

Use trusted participants in comparable scenarios under:

1. no external information;
2. legitimate recent visual contact;
3. administrator-provided hidden-target callouts.

This creates known extra-information behaviour without deploying cheat software. The primary features should respond to the information condition while discounting the legitimate-knowledge condition.

### 19.3 Negative controls

The analysis must not produce strong effects for:

- future target positions;
- time-shuffled target positions;
- randomly reassigned target identities;
- pseudo-target sectors;
- outcomes outside the declared decision window;
- deliberately delayed visibility probes beyond the feature limit;
- random opportunity labels permuted within matched strata.

Unexpected signal in these controls indicates leakage, timing errors or confounding.

### 19.4 Holdout validation

Evaluate on held-out:

- dates;
- map areas;
- player sessions;
- target identities;
- server population bands;
- trusted-player cohorts not used for calibration.

Require rank and effect stability when the strongest session is removed.

### 19.5 Operational metrics

Track:

```text
precision_at_k_for_review_worthy_cases
lift_over_random_case_review
reviewer_minutes_per_useful_case
trusted_cohort_high_priority_rate
rank_stability_across_session_bootstraps
percent_suppressed_for_poor_controls
percent_suppressed_for_timing_or_visibility_ambiguity
probe_cost_per_usable_observation
```

Success means improved review efficiency and evidence quality. It does not mean an estimated confirmed-cheater detection rate from this system alone.

### 19.6 Blinded review

Where practical:

- hide player identity during first-pass incident review;
- mix top-ranked and random cases;
- record reviewer agreement and false-positive categories;
- do not treat admin disposition as perfect ground-truth labels;
- use dispositions to refine controls and evidence presentation before model weights.

## 20. Evidence API and review package

### 20.1 Candidate API

Minimum endpoints:

```text
GET  /v1/review-candidates
GET  /v1/review-candidates/{candidate_id}
GET  /v1/review-cases/{case_id}
GET  /v1/review-incidents/{incident_id}
POST /v1/review-cases/{case_id}/dispositions
POST /v1/replay-runs
GET  /v1/algorithm-runs/{run_id}
```

Filtering should support server, date range, camera mode, squad status, confidence, feature family and review disposition.

### 20.2 Candidate summary

Expose:

- raw hidden and control rates;
- effect sizes and uncertainty;
- opportunity, encounter, target and session counts;
- control quality;
- authority-tier distribution;
- telemetry completeness;
- sampling and algorithm versions;
- stability when strongest session is removed;
- reason the case entered its tier.

### 20.3 Incident package

Each incident contains:

- 2D route and positions over time;
- observer camera direction when available;
- weapon/ADS state;
- visibility point results and derived class;
- first-person/third-person semantics;
- queue and timing uncertainty;
- shots, hits, deaths and engagement context;
- cue facts and last-known-position envelope;
- matched-control examples and matching variables;
- explicit uncertainty notes;
- raw source-event references.

### 20.4 Language

Use:

- `review priority`;
- `hidden-awareness response effect`;
- `unexplained in captured data`;
- `supporting pattern`;
- `data confidence`;
- `insufficient data`.

Do not use:

- `cheat probability`;
- `confirmed radar`;
- `impossible knowledge`;
- `proof`.

## 21. Observability and performance

### 21.1 DayZ client metrics

- effective sample rate by mode;
- ring-buffer occupancy;
- dropped samples;
- edge RPC admitted/coalesced/dropped;
- batch bytes and messages;
- serialization time;
- last successful send.

### 21.2 DayZ server metrics

- snapshot duration;
- candidate construction duration;
- random and enrichment queue depth/age;
- probes admitted/completed/dropped/failed;
- raycast duration by policy and blocker class;
- decision-to-probe queue delay;
- diagnostic windows active;
- export queue depth and dispatch time;
- spool bytes/files/oldest age;
- replay success/failure;
- gameplay tick percentiles.

### 21.3 Go service metrics

- ingestion requests, bytes and latency;
- durable-write latency;
- duplicate/rejected batches;
- normalization lag;
- replay throughput;
- observation build lag;
- matched-control yield and quality;
- feature-run duration and failures;
- candidates suppressed by each quality gate;
- review API latency;
- retention/deletion job results.

### 21.4 Alerts

Alert on:

- sustained spool growth;
- authoritative event loss;
- random opportunity admission collapse;
- clock uncertainty degradation;
- abnormal visibility ambiguity rate;
- schema mismatch;
- replay divergence from golden fixtures;
- material collector overhead regression.

## 22. Security, privacy and retention

- Bind the Go ingestion service to localhost or a private network by default.
- Authenticate server-to-service transport with a per-server secret or mTLS.
- Rotate credentials without invalidating historical event attribution.
- Store raw identifiers separately from analyst-facing pseudonymous IDs where practical.
- Restrict raw telemetry and audit review access.
- Do not write sensitive identifiers to public logs.
- Make raw, normalized and review-evidence retention independently configurable.
- Document collection to server users as required by operator policy and applicable law.
- Support deletion by durable player identity while preserving aggregate, non-identifying calibration data where policy permits.
- Never send analysis results back into gameplay logic in the MVP.

## 23. Delivery milestones

### Milestone 0: reconcile current work

Deliverables:

- repository implementation inventory;
- gap matrix against this plan;
- decision record for retained interfaces;
- issue list with owners and dependencies;
- no duplicate component work.

Exit criteria:

- every current component is mapped;
- schema conflicts are documented;
- migration strategy is agreed;
- the next milestone can proceed without two implementations of the same seam.

### Milestone 0A: capability matrix

Deliverables:

- development-only DayZ spike;
- client/server/remote/dedicated execution matrix;
- `OnFire`, `EEFired`, raised-state, optics, third-person and RPC findings;
- pinned DayZ build, scripts revision and source commit;
- hook tests and logs.

Exit criteria:

- every MVP hook is verified or replaced;
- resulting authority tier is recorded;
- no fictional callback remains in the design.

### Milestone 0B: visibility experiment

Deliverables:

- controlled labelled scene matrix;
- first-person and third-person result separation;
- adaptive multi-point probe prototype;
- blocker taxonomy;
- confusion matrix and ambiguity rates.

Exit criteria:

- a conservative first-person `ROBUSTLY_OCCLUDED` policy is defined;
- third-person strong evidence remains disabled unless proven;
- invalid/ambiguous cases cannot be promoted.

### Milestone 0C: sampling and latency experiment

Deliverables:

- random opportunity scheduler;
- event-enrichment scheduler;
- low-latency decision-edge RPC prototype;
- four-timestamp alignment prototype;
- queue/load-shedding metrics;
- representative load report.

Exit criteria:

- both streams are distinguishable and replayable;
- inclusion/admission probabilities are recorded;
- decision-to-probe latency is measured;
- safe initial budgets are documented.

### Milestone 1: versioned telemetry collector

Deliverables:

- client sampler and pure transition detector;
- bounded edge and batch RPC paths;
- server validation and identity binding;
- authoritative snapshots and combat/lifecycle events;
- dual-stream scheduler and probe queue;
- async exporter, rotated spool and health metrics;
- NDJSON development dump and golden fixtures.

Exit criteria:

- all queues and buffers are bounded;
- no blocking network call exists;
- collector replay fixture passes;
- schema and policy versions are present;
- representative overhead is within operator-approved tolerance.

### Milestone 2: ingestion, normalization and replay

Deliverables:

- versioned authenticated HTTP endpoint;
- durable raw store before acknowledgement;
- PostgreSQL normalized schema and migrations;
- idempotent replay command;
- clock-alignment records;
- Docker Compose development environment;
- ingestion and replay metrics.

Exit criteria:

- duplicate ingestion is harmless;
- raw events replay into a clean database deterministically;
- sidecar outage/spool replay fixture passes;
- normalized records retain source authority and policy versions.

### Milestone 3: observation builder and calibration dataset

Deliverables:

- sessions, encounters, episodes and windows;
- cue ledger and last-known envelopes;
- visibility interpretation;
- matched-control construction and quality score;
- random opportunity observations;
- controlled and trusted calibration captures.

Exit criteria:

- independence rules pass fixtures;
- event-enrichment data cannot silently enter the primary readiness denominator;
- control quality and partial observation are explicit;
- negative-control datasets can be generated.

### Milestone 4: core analytics

Deliverables:

- readiness beta-binomial baseline;
- matched conditional logistic validation;
- circular sector permutation test;
- pre-exposure discrete-time analysis;
- robust cohort residuals;
- session/encounter block bootstrap;
- leave-one-session-out stability;
- versioned JSON/CSV outputs.

Exit criteria:

- sparse players are shrunk rather than over-ranked;
- negative controls show no material signal;
- trusted cohorts do not create an unacceptable high-priority rate;
- every result includes effect, uncertainty, counts and evidence references.

### Milestone 5: review API and basic UI

Deliverables:

- candidate, case, incident and disposition APIs;
- route/timeline visualization;
- matched controls and uncertainty display;
- blinded-review mode;
- audit trail;
- exportable evidence package.

Exit criteria:

- a reviewer can understand why a case was selected;
- the interface never presents an automatic verdict;
- top-ranked review yield materially exceeds random-case review in calibration.

### Milestone 6: advanced behaviour research

Candidate work:

- changepoint detection;
- Isolation Forest discovery layer;
- map route graph;
- pursuit/avoidance residuals;
- hidden/semi-Markov state model;
- known/inferred squad propagation;
- lagged permutation and optional Hawkes modelling.

Exit criteria:

- each algorithm improves held-out review efficiency beyond the core feature set;
- each produces explainable supporting evidence;
- none can independently bypass primary evidence gates.

## 24. Suggested repository layout

Adapt this to current paths rather than moving working code unnecessarily.

```text
docs/
  behavioural-awareness-spec.md
  behavioural-awareness-implementation-plan.md
  capability-matrix.md
  visibility-validation.md
  sampling-and-latency-report.md
  adr/
    001-authority-tiers.md
    002-random-opportunity-sampling.md
    003-first-vs-third-person-visibility.md
    004-observation-independence.md
    005-raw-storage-and-replay.md

schemas/
  wire/
  normalized/
  observations/
  examples/

fixtures/
  dayz/
  ingestion/
  replay/
  visibility/
  analysis/

mod/
  client/
  server/
  shared/

go/
  cmd/
    ingest/
    replay/
    analyse/
  internal/
    api/
    rawstore/
    normalize/
    clockalign/
    episodes/
    cues/
    observations/
    matching/
    features/
    ranking/
    review/
  migrations/

evaluation/
  controlled-scenarios/
  negative-controls/
  reports/

load-tests/
```

## 25. Recommended issue breakdown

### Epic A: integration and contracts

1. Inventory current repository implementation.
2. Publish component and schema gap matrix.
3. Define common event envelope and authority tiers.
4. Define independent policy-version fields.
5. Add backward-compatible schema migration policy.

### Epic B: DayZ feasibility

6. Verify client/server execution context for raised, optics and third-person state.
7. Verify `Weapon_Base.OnFire` and `EEFired` on dedicated server.
8. Verify hit, death, RPC and lifecycle hooks.
9. Validate bone points in all supported stances.
10. Build first-person/third-person visibility fixture scenes.
11. Profile raycast, snapshot, serialization and export cost.

### Epic C: collection and scheduling

12. Implement pure client state-transition detector.
13. Implement bounded client sampler and ring buffer.
14. Implement low-latency decision-edge RPC with token bucket.
15. Implement normal RPC batches and server validator.
16. Implement authoritative snapshots and movement transitions.
17. Implement random opportunity scheduler.
18. Implement event-enrichment scheduler.
19. Implement adaptive visibility probe worker.
20. Implement diagnostic pair windows.
21. Implement asynchronous exporter and rotated spool.
22. Add collector metrics and load shedding.

### Epic D: ingestion and replay

23. Define raw batch durability contract.
24. Implement authenticated versioned ingest endpoint.
25. Implement idempotency and raw metadata.
26. Implement normalization and migrations.
27. Implement four-timestamp clock alignment.
28. Implement spool/raw replay command.
29. Add golden end-to-end replay tests.

### Epic E: observation construction

30. Implement sessions and reconnect semantics.
31. Implement encounters and target episodes.
32. Implement decision/refractory windows.
33. Implement visibility evidence interpretation.
34. Implement cue ledger and last-known envelope.
35. Implement random-opportunity observation builder.
36. Implement matched-control search and quality scoring.
37. Add partial-observation and design-weight handling.

### Epic F: primary analytics

38. Implement beta-binomial readiness estimator.
39. Implement matched conditional logistic validator.
40. Implement circular angular-error and permutation test.
41. Implement discrete-time pre-exposure model.
42. Implement robust cohort residuals.
43. Implement block bootstrap and leave-one-session-out tests.
44. Implement transparent review tier policy.
45. Add negative-control and holdout evaluation runner.

### Epic G: review experience

46. Implement candidate and case APIs.
47. Implement incident timeline and map trace.
48. Display authority, sampling, cue and visibility details.
49. Implement reviewer dispositions and audit trail.
50. Add blinded comparison with random cases.

### Epic H: advanced research

51. Evaluate CUSUM and Bayesian online changepoint detection.
52. Evaluate Isolation Forest on adjusted aggregates.
53. Build map route graph and route priors.
54. Evaluate hidden/semi-Markov route states.
55. Evaluate squad lagged permutation and network models.

## 26. Definition of done

A production-capable first release is complete only when:

1. Every DayZ hook used by the MVP is verified on the target build.
2. The client and server collectors use bounded work and no blocking HTTP call.
3. Server identity attribution ignores client-supplied identity fields.
4. Raw data is durably stored before acknowledgement.
5. Raw data can be replayed after schema and algorithm changes.
6. Random opportunity and event-enrichment streams are separate and labelled.
7. Inclusion probabilities, queue admission and drop reasons are recorded.
8. Strong hidden evidence is limited to validated visibility semantics.
9. Third-person head occlusion cannot masquerade as camera occlusion.
10. Client telemetry remains supporting rather than authoritative.
11. Observation independence is enforced through episodes and windows.
12. Primary models use matched context, shrinkage and uncertainty.
13. Rankings require repeated evidence across sessions, encounters and targets.
14. Negative controls do not show material hidden-response signal.
15. Trusted-player validation meets the agreed false-priority tolerance.
16. Every candidate exposes raw rates, controls, counts, uncertainty and versions.
17. No automated punishment path exists.
18. Representative load testing meets the operator-approved gameplay tolerance.
19. Review yield is demonstrably better than random case selection.
20. Retention, access control and deletion policies are implemented and documented.

## 27. Main risks and mitigations

| Risk | Consequence | Mitigation |
|---|---|---|
| Event-dependent sampling | Biased readiness estimates | Random prospective opportunity stream |
| Third-person visibility mismatch | False hidden classifications | 1PP-only strong evidence; validate camera envelope later |
| Client telemetry forgery or suppression | Misleading camera/readiness context | Authority tiers; never increase suspicion for missing client data |
| Probe latency | Visibility measured after the relevant state changed | Edge RPC, queue timing, suppression by uncertainty |
| Correlated samples | One firefight appears as many incidents | Episodes, refractory windows and block bootstrap |
| Unobserved audio or Discord callouts | Legitimate knowledge appears unexplained | Cue uncertainty, conservative language and lower confidence |
| Sparse combat data | Extreme unstable rates | Partial pooling, evidence gates and long calibration windows |
| Skilled/map-specialist players | Behavioural outliers without cheating | Same-player controls, map/area matching and trusted cohorts |
| Modded/dynamic geometry | Incorrect occlusion | Ambiguous classification and fixture expansion |
| Load pressure changes sample population | Biased data under busy conditions | Reserved inference budget and recorded admission probabilities |
| Reviewer automation bias | Outlier score treated as proof | Component-level evidence, blind review and no enforcement path |
| Adaptive adversaries | Behaviour changes around known features | Multiple feature families, changepoint monitoring and private operational thresholds |

## 28. Open decisions to resolve through evidence

1. Target DayZ build, maps and mod stack for the first spike.
2. Whether weapon-raised state is reliable on server remote entities.
3. Whether `OnFire` or `EEFired` provides the best authoritative shot seam.
4. Safe sampling, edge-RPC and probe budgets by population.
5. Random opportunity rate and prospective window duration.
6. Candidate risk-set and distance-band definitions.
7. First-person occlusion policy and acceptable ambiguity rate.
8. Whether any third-person camera envelope is reliable enough for later strong use.
9. Raw retention duration and normalized/review retention duration.
10. Initial trusted calibration cohorts.
11. Map hotspot and route-prior source.
12. Group-mod adapter target.
13. Operational review-tier thresholds after calibration.
14. Whether the hierarchical model is implemented directly in Go or validated offline before a Go implementation.
15. Reviewer workflow and UI technology after the JSON/API prototype proves value.

## 29. Recommended first operational release

Ship only after calibration with this scope:

- first-person observations only for strong concealed-visibility evidence;
- random prospective opportunity sampling plus separate event enrichment;
- authoritative server movement, combat and visibility evidence;
- client camera and ADS/raised state as supporting telemetry where server state is unavailable;
- hidden-threat readiness lift;
- concealed-sector selection;
- pre-exposure readiness;
- beta-binomial shrinkage and matched-model validation;
- circular permutation tests;
- independent session, encounter and target gates;
- JSON/CSV evidence packages and basic review API;
- no pursuit, avoidance, squad inference, opaque anomaly score or automatic action.

This release is sufficient to answer the central product question: **does context-adjusted responsiveness to concealed opponents improve manual review efficiency without creating an unacceptable rate of strong-player false positives?**

## 30. Primary technical references

DayZ implementation must be revalidated against the exact target build and scripts revision. The current specification pins a source basis in the official script-diff repository.

- [Bohemia Interactive DayZ Script Diff](https://github.com/BohemiaInteractive/DayZ-Script-Diff)
- [Mission gameplay update surface](https://github.com/BohemiaInteractive/DayZ-Script-Diff/blob/main/scripts/5_mission/mission/missiongameplay.c)
- [Mission server update and player enumeration](https://github.com/BohemiaInteractive/DayZ-Script-Diff/blob/main/scripts/5_mission/mission/missionserver.c)
- [Player view-state implementation](https://github.com/BohemiaInteractive/DayZ-Script-Diff/blob/main/scripts/4_world/entities/dayzplayerimplement.c)
- [Weapon fire state](https://github.com/BohemiaInteractive/DayZ-Script-Diff/blob/main/scripts/4_world/entities/firearms/fsm/states/weaponfire.c)
- [Weapon base fire surfaces](https://github.com/BohemiaInteractive/DayZ-Script-Diff/blob/main/scripts/4_world/entities/firearms/weapon_base.c)
- [DayZ physics raycasts](https://github.com/BohemiaInteractive/DayZ-Script-Diff/blob/main/scripts/3_game/global/dayzphysics.c)
- Adams and MacKay, [Bayesian Online Changepoint Detection](https://arxiv.org/abs/0710.3742)
- Liu, Ting and Zhou, [Isolation Forest](https://doi.org/10.1109/ICDM.2008.17)
- Pewsey, Neuhäuser and Ruxton, [Circular Statistics in R](https://www.jstatsoft.org/article/view/v031b10)
