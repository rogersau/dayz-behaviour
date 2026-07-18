# Behavioural Awareness Telemetry for DMA/Radar Review Prioritisation

**Status:** Proposed MVP  
**Decision:** Build as a review-prioritisation and evidence system only. It must never produce an authoritative ban verdict.

## Problem statement

Existing DayZ anti-cheat tooling is strongest against cheats that alter game state or produce mechanically impossible results, such as invalid movement, impossible inventory operations, redirected bullets, or crude aim manipulation.

External-information cheats are different. A DMA card, second-PC radar, or other read-only ESP can leave the DayZ process and normal shot mechanics untouched. The player still receives hidden information and eventually expresses that advantage through ordinary gameplay decisions:

- slowing or stopping before an unseen threat;
- raising a weapon before legitimate contact;
- choosing the occupied route, window, ridgeline, or flank;
- pursuing a concealed player with unusually efficient route changes;
- avoiding occupied positions while treating equivalent empty positions normally;
- preparing for the correct emergence point before a target becomes visible;
- passing concealed-player knowledge to teammates.

Administrators need a system that identifies accounts and squads whose repeated decisions correlate unusually strongly with hidden-player information, then presents the strongest incidents for manual review.

## Proposed solution

Build a three-part telemetry and analysis system:

1. A required DayZ client mod collects low-cost local camera and combat-readiness state that the server cannot independently observe.
2. A DayZ server mod collects authoritative world state, player lifecycle and combat events, performs a tightly bounded number of live-world visibility probes, joins client telemetry to the authenticated player identity, and exports batches.
3. An external Go service durably ingests events, reconstructs timelines, derives behavioural features, compares players against matched controls and relevant cohorts, ranks review candidates, and produces explainable evidence packages.

The DayZ process is a collector and live-world probe. It is not the statistical analysis engine.

## Goals

1. Rank players and squads for additional manual review based on repeated hidden-awareness behaviour.
2. Detect behavioural leakage from read-only external-information cheats, including DMA radar, without requiring process inspection.
3. Keep expensive pairwise calculations, historical queries and outlier analysis outside the DayZ gameplay script workload.
4. Produce evidence an administrator can understand: timelines, routes, visibility state, relevant cues, matched controls and repeated pattern counts.
5. Preserve raw telemetry so improved analysis can be rerun against historical sessions.
6. Keep DayZ-side CPU, network and storage costs bounded and measurable.
7. Treat client telemetry as untrusted supporting evidence rather than authority.

## Non-goals

1. Automatically ban, kick or punish a player.
2. Prove that a DMA device or radar exists.
3. Replace BattlEye, CFTools or existing movement, inventory and shot-integrity checks.
4. Reconstruct exactly what a player saw or heard.
5. Perform full-population pairwise visibility calculations every frame.
6. Detect every careful radar user.
7. Train a black-box machine-learning classifier in the first release.
8. Build a pixel-perfect replay of the rendered client scene.
9. Assume vanilla DayZ exposes squad membership; group integration is optional and server-specific.

## Verified DayZ script capabilities

All MVP collection points are based on real surfaces present in `BohemiaInteractive/DayZ-Script-Diff`. Implementation must revalidate these surfaces against the DayZ version targeted by the mod.

| Capability | Real script surface | Intended use | Constraint |
|---|---|---|---|
| Client periodic sampling | `MissionGameplay.OnUpdate(float timeslice)` or `Timer`/call queue | Accumulator-driven local sampling | Must preserve vanilla update behaviour and remain bounded |
| Server periodic sampling | `MissionServer.OnUpdate(float timeslice)` and gameplay call queue | Snapshot players and drain bounded work queues | No statistical analysis in this loop |
| Enumerate connected players | `g_Game.GetPlayers(array<Man>)` | Build server snapshots and candidate lists | Filter invalid, dead and disconnecting players |
| Client camera transform | `g_Game.GetCurrentCameraPosition()` and `g_Game.GetCurrentCameraDirection()` | Capture exact local camera origin/direction | Client-owned and forgeable |
| Client combat view state | `DayZPlayerImplement.IsInIronsights()`, `IsInOptics()`, `IsInThirdPerson()` and `IsFireWeaponRaised()` | Record sampled state and transitions | There is no assumed weapon-raised callback |
| Client-to-server telemetry | `ScriptRPC.Write(...)`, `ScriptRPC.Send(...)`, received through a modded `OnRPC(...)` surface such as `PlayerBase.OnRPC` | Send compact telemetry batches | Bind to server-supplied sender identity |
| Successful fire seam | `WeaponFire.OnEntry` invokes native `TryFireWeapon(...)`, then `m_weapon.OnFire(muzzleIndex)` after success | Capture a same-frame local shot marker | Spike must confirm execution context on client and dedicated server |
| Player hit event | `PlayerBase.EEHitBy(...)` | Record server-observed hit data | Does not expose the complete original native shot packet |
| Player death event | `PlayerBase.EEKilled(...)` | Close engagement windows and anchor handoff analysis | Preserve vanilla behaviour |
| Hard-geometry visibility | `DayZPhysics.RaycastRV(...)` / `RaycastRVProxy(...)` with `ObjIntersectView` | Classify a small number of candidate pairs | Geometry exposure is not human perception |
| Player/bone positions | entity position/orientation plus bone lookup/world-position APIs | Use head/torso probe points | Validate bones across standing, crouched and prone states |
| External HTTP export | asynchronous `RestContext.POST(...)` through `GetRestApi()` | Send batches to the local Go service | `POST_now(...)` is prohibited in gameplay code |
| Local spool fallback | `OpenFile`, `FPrint`, `CloseFile`, `JsonSerializer` / `JsonFileLoader` | Write completed append-only batches | Do not rewrite a single growing JSON document |
| Timing/profiling | `g_Game.GetTime()` and `TickCount(...)` | Event timing and collector-cost instrumentation | Client/server clock alignment remains approximate |

### Primary source paths

- `scripts/5_mission/mission/missiongameplay.c`
- `scripts/5_mission/mission/missionserver.c`
- `scripts/4_world/entities/itembase/rangefinder.c`
- `scripts/4_world/classes/weapondebug.c`
- `scripts/4_world/entities/dayzplayerimplement.c`
- `scripts/4_world/entities/firearms/fsm/states/weaponfire.c`
- `scripts/4_world/entities/firearms/weapon_base.c`
- `scripts/4_world/entities/manbase/playerbase.c`
- `scripts/3_game/global/dayzphysics.c`
- `scripts/3_game/http/restapi.c`
- `scripts/3_game/tools/jsonfileloader.c`
- `scripts/4_world/plugins/pluginbase/plugindeveloper/pluginremoteplayerdebugclient.c`

Pinned source basis used during design: DayZ-Script-Diff commit `f17541a021e574ad70836405426877708263a7a5` where returned by repository search.

## High-level architecture

```text
DayZ client mod
  - local camera and combat-state sampler
  - small ring buffer
  - batched ScriptRPC telemetry
               |
               v
DayZ server mod
  - authenticates telemetry sender
  - authoritative player snapshots and combat events
  - sampled state-transition detector
  - bounded live visibility probe scheduler
  - batcher, async exporter and disk spool
               |
               v
External Go service
  - HTTP ingestion and durable raw storage
  - event normalisation and clock alignment
  - session/encounter reconstruction
  - feature extraction and matched controls
  - cohort/outlier analysis
  - review candidate and evidence API
```

## Ownership boundaries

### DayZ client mod owns

1. Sampling the local player only.
2. Reading current camera position and direction.
3. Reading local combat view state: weapon raised, ironsights, optics and third-person state.
4. Deriving local transitions by comparing current and previous samples.
5. Maintaining a bounded in-memory ring buffer.
6. Creating a same-frame telemetry sample when local `Weapon_Base.OnFire` occurs.
7. Batching and sending telemetry through `ScriptRPC`.
8. Including monotonic client sequence numbers and the most recent server-issued session nonce.
9. Reporting telemetry health, including dropped samples and buffer overflow.

The client mod does not enumerate or analyse remote players, decide visibility, calculate scores, identify review candidates, or contain a secret assumed safe from tampering.

### DayZ server mod owns

1. Binding every received telemetry batch to the `PlayerIdentity` supplied by the RPC receive path.
2. Rejecting malformed, oversized, stale, duplicate or out-of-order batches.
3. Assigning server session IDs, receive times and server sequences.
4. Recording connect, ready, respawn, reconnect and disconnect lifecycle events.
5. Sampling authoritative player position and orientation at a configurable low rate.
6. Deriving minimal movement state from successive authoritative positions: speed, movement heading, start/stop and large route-heading changes.
7. Recording authoritative hit and death events through `PlayerBase` extension points.
8. Recording server shot markers only if the feasibility spike proves the dedicated-server `Weapon_Base.OnFire` seam reliable.
9. Detecting a bounded set of decision events:
   - weapon raised/lowered from client samples;
   - ADS/optics entered/exited from client samples;
   - significant server-derived stop/start;
   - significant server-derived route-heading change;
   - shot, hit and death events.
10. Selecting a bounded number of nearby candidate targets for a decision event.
11. Performing live-world visibility probes for those candidate pairs.
12. Recording visibility as `HARD_OCCLUDED`, `GEOMETRICALLY_EXPOSED` or `UNKNOWN`.
13. Maintaining short bounded queues and ring buffers only.
14. Exporting telemetry batches asynchronously to the Go service.
15. Spooling completed batches to disk when export fails.
16. Measuring its own sampling, probe, serialisation and export cost.

The DayZ server mod does not run population-wide behavioural scoring, lifetime percentiles, database queries, matched-control generation, DMA inference or punishments.

### External Go service owns

1. Receiving DayZ batches over a versioned HTTP API.
2. Authenticating the DayZ server using a server-to-service secret or mTLS where required.
3. Durably storing raw batches before acknowledgement.
4. De-duplicating by server ID, session ID and sequence.
5. Normalising events and maintaining schema migrations.
6. Aligning client event times with server times while preserving uncertainty.
7. Reconstructing sessions, routes, decisions, engagements and observer/target relationships.
8. Creating matched control observations.
9. Calculating behavioural features.
10. Building player, squad and cohort baselines.
11. Ranking outliers for manual review.
12. Producing explainable evidence packages.
13. Replaying raw telemetry when feature logic changes.
14. Exposing an admin API and later a lightweight review UI.
15. Applying retention and deletion policies.

The Go service does not claim exact visual/audible perception, trust client telemetry as authority, ban players, or send gameplay-affecting decisions back into DayZ in the MVP.

## Collection model

### Client sampling cadence

Use an accumulator inside a modded `MissionGameplay.OnUpdate` so collection cadence is independent of frame rate.

Suggested initial rates:

- baseline: 2 Hz while alive and in gameplay;
- raised weapon or ADS/optics: 10 Hz;
- after a local shot, taking a hit, or a server-requested diagnostic window: 10 Hz for 10-15 seconds;
- dead, unconscious or outside gameplay: stop or reduce to lifecycle-only reporting.

Each client camera sample contains:

```text
schema_version
client_sequence
client_monotonic_time_ms
camera_position
camera_direction
player_position_client_observed
is_weapon_raised
is_in_ironsights
is_in_optics
is_in_third_person
item_in_hands_type_id
sample_flags
```

Position and direction should be quantised before RPC serialization after measuring actual Enforce RPC overhead.

### Client batching

- bounded ring buffer, initially ten seconds at maximum rate;
- at most one normal batch per second;
- optional immediate flush after a shot marker subject to rate limits;
- strict cap on samples and bytes per batch;
- no waiting for acknowledgement;
- counters for locally dropped samples and server-rejected batches.

### Server snapshots

At an initial 2 Hz cadence, record for each active player:

```text
server_sequence
server_time_ms
player_session_id
position
orientation
alive
unconscious
item_in_hands_type_id where available
```

### Combat events

Record:

- `SHOT_MARKER_SERVER` only where server execution is proven;
- `SHOT_MARKER_CLIENT` for the local client report;
- `PLAYER_HIT` from `EEHitBy`;
- `PLAYER_KILLED` from `EEKilled`;
- victim, source/root shooter where resolvable, weapon/ammo identifiers, component, zone and callback hit-position data.

Client and server shot markers remain separate records.

## Decision-event detection

The MVP must not invent callbacks DayZ does not expose. Decision events are derived from real sampled state transitions.

### Client-derived

- `WEAPON_RAISED` / `WEAPON_LOWERED`: change in `IsFireWeaponRaised()`.
- `ADS_ENTERED` / `ADS_EXITED`: change in ironsight or optics state.
- `CAMERA_TURN`: angular displacement exceeds a configured threshold over a configured interval.
- `SHOT_FIRED_CLIENT`: local `Weapon_Base.OnFire` marker.

### Server-derived

- `MOVEMENT_STOPPED` / `MOVEMENT_STARTED`: derived speed crosses a configured threshold.
- `ROUTE_HEADING_CHANGED`: movement heading changes beyond a configured threshold after sufficient displacement.
- `PLAYER_HIT`.
- `PLAYER_KILLED`.
- `SHOT_FIRED_SERVER` if verified.

Thresholds are collection configuration, not cheat verdict rules.

## Candidate selection and live visibility probes

The external service cannot reproduce the complete live DayZ collision world, including dynamic doors, vehicles, base-building objects and modded map state. DayZ therefore owns limited visibility classification at event time.

### Candidate selection

When a meaningful decision occurs, the server:

1. identifies the observer;
2. selects nearby active players from the current snapshot;
3. excludes the observer and invalid/dead entities;
4. optionally excludes known group members when an adapter exists;
5. sorts by distance;
6. selects no more than a configured maximum, initially three candidates within a configurable 1,000 m radius;
7. queues probes rather than executing an unbounded immediate loop.

### Probe implementation

For each selected pair:

1. Determine a server-owned observer origin, preferably a validated head/eye bone world position.
2. Determine target head and torso points using validated bone positions.
3. Perform at most two `ObjIntersectView` probes using the simplest DayZ physics API that returns enough information to identify the blocker.
4. Exclude the observer.
5. Classify:
   - `GEOMETRICALLY_EXPOSED` when at least one validated target point is reached without a blocking contact;
   - `HARD_OCCLUDED` when both validated target points are blocked by recognised hard geometry;
   - `UNKNOWN` for vegetation, ambiguous proxy results, invalid bones, missing entities or incomplete work.
6. Record blocker category where available.

### Probe budget

- global maximum probes per server update/tick;
- per-observer cooldown for repeated equivalent probes;
- per-pair cooldown unless a diagnostic window is active;
- queue depth limit with low-priority work dropped first;
- metrics for queued, completed, dropped and failed probes.

The implementation degrades by losing optional observations, never by delaying gameplay.

### Diagnostic pair windows

After a high-value event such as a weapon raise near a hard-occluded candidate, the server may track that pair for up to five seconds at a bounded 2-5 Hz probe rate. Simultaneous windows are capped globally and per observer.

## Cue ledger

The analyser distinguishes behaviour with a captured explanation from behaviour where no captured explanation exists.

MVP captured cues:

- target was geometrically exposed during a recent probe;
- observer or a known group member recently hit or killed the target;
- target recently fired a recorded shot within a conservative distance;
- observer was recently hit;
- ongoing engagement with the same target/group;
- target was previously exposed within a configurable memory window.

The system does not claim whether footsteps, distant gunfire or Discord communication were heard.

Cue classifications:

- `KNOWN`;
- `PLAUSIBLE`;
- `UNEXPLAINED_IN_CAPTURED_DATA`.

The last classification deliberately does not mean impossible legitimately.

## External Go service design

### Suggested structure

```text
/cmd/ingestd
/cmd/analyzerd
/internal/ingest
/internal/storage
/internal/timeline
/internal/features
/internal/cohorts
/internal/review
/pkg/schema
/deploy
```

A single binary with subcommands is acceptable for the MVP if package boundaries remain clear.

### Ingestion API

`POST /v1/telemetry/batches`

Requirements:

- server authentication;
- request size limit;
- schema validation;
- idempotency by `(server_id, batch_sequence)`;
- durable raw write before success response;
- no feature calculation in request path;
- structured rejection reasons;
- health, readiness and metrics endpoints.

### Storage

MVP recommendation:

- compressed append-only raw batch files for exact replay;
- PostgreSQL for normalised events, sessions, observations, features and review cases;
- time-based partitions for high-volume event tables;
- configurable retention.

Suggested initial retention:

- raw high-frequency telemetry: 14-30 days;
- normalised decision/visibility/combat events: 90 days;
- derived aggregate features and manually reviewed cases: longer, subject to policy.

### Clock alignment

Preserve:

- client monotonic timestamp;
- server receipt timestamp;
- server event timestamp;
- client/server sequences;
- estimated client-to-server offset and uncertainty.

Sub-second analysis must retain uncertainty instead of silently treating clocks as identical.

## MVP behavioural features

The first release focuses on decision patterns useful for DMA/radar review and avoids ordinary aim-accuracy scoring.

### 1. Hidden-threat readiness lift

Does the player become combat-ready more often when a hard-occluded enemy is nearby than in matched situations without one?

Transitions:

- weapon raise;
- ADS/optics entry;
- movement stop/slowdown;
- route-heading change.

Outputs:

```text
weapon_raise_rate_hidden
weapon_raise_rate_control
readiness_lift_ratio
sample_count
independent_target_count
session_count
```

### 2. Correct sector selection

At a deliberate camera or route-heading change, does the player select the sector containing a concealed enemy unusually often?

- divide observer-relative horizontal space into configurable angular sectors;
- mark sectors containing hard-occluded candidates;
- record first deliberate camera sector after entering a decision window;
- compare with matched controls and the player’s own scan distribution.

Discount obvious doorways, recent gunfire directions and previously known locations.

### 3. Pre-exposure readiness

Did the observer become ready for the correct target shortly before a hard-occluded target became geometrically exposed?

Values:

```text
readiness_to_first_exposure_ms
angular_error_at_readiness
was_target_recently_known
cue_classification
```

Do not invent the number of plausible exits unless supported by map-specific modelling.

### 4. Hidden pursuit efficiency

Does a route repeatedly converge on concealed players with less search behaviour than expected?

Derived in Go using authoritative positions plus visibility/cue observations:

- distance reduction after route decisions;
- path efficiency toward concealed target positions;
- route changes following concealed target movement;
- unrelated sectors bypassed;
- hotspot/control-area context.

This feature requires larger sample sizes and lower confidence.

### 5. Hidden threat avoidance

Does the player repeatedly stop, reverse or detour before concealed contact compared with equivalent empty routes?

This is noisy and supporting-only; it must never dominate ranking.

### 6. Post-engagement concealed-target handoff

After a hit or kill, does the observer immediately turn, move or take cover relative to another concealed, previously unexplained player?

Values:

```text
time_from_engagement_event_to_turn_ms
turn_angle
angular_error_to_hidden_candidate
candidate_visibility_state
candidate_cue_classification
independent_group_id
```

### 7. Squad information propagation

Do multiple players coordinate around a concealed target when no member has a captured legitimate cue?

Support:

- optional adapter for the server’s group/clan mod;
- fallback inferred grouping using repeated proximity/co-movement, clearly labelled lower-confidence.

Do not claim which member owns a radar.

### 8. Telemetry integrity

Supporting health/review signals:

- missing batches;
- duplicate/replayed sequences;
- timestamp drift;
- telemetry stopping disproportionately during combat;
- impossible camera position relative to the player;
- camera/body orientation disagreement outside reasonable freelook/third-person cases.

These are not DMA indicators by themselves.

## Matched controls and cohorts

### Match on available context

- map and approximate area;
- distance band;
- observer speed/state;
- weapon/ADS baseline;
- time since last shot/hit;
- population band;
- time-of-day/weather identifiers where cheap;
- first-/third-person state;
- hotspot classification;
- recent exposure/cue state.

Control sources:

1. same player in equivalent empty situations;
2. trusted/cohort players in the same context;
3. synthetic empty angular sectors around the same observation where geometry information permits.

Every ratio includes sample count and control-quality metadata.

### Initial cohorts

- server/map;
- distance band;
- first-/third-person;
- approximate playtime/session count;
- combat intensity;
- solo versus known/inferred group.

Use robust statistics and percentiles; do not assume a normal distribution.

## Ranking model

Use explainable weighted evidence with minimum gates:

- minimum independent observations;
- minimum unrelated targets;
- minimum distinct sessions;
- minimum control quality;
- discounts for plausible cues, missing telemetry and inferred group information.

No single incident creates a high-priority case. Avoid labels such as `cheat probability`.

Preferred language:

- `review priority`;
- `hidden-awareness outlier percentile`;
- `unexplained observations`;
- `supporting pattern`;
- `data confidence`.

## Review evidence package

A review case contains:

1. player/session summary;
2. feature percentiles, raw rates and control rates;
3. observations, independent targets and session counts;
4. telemetry completeness/confidence;
5. strongest incidents;
6. for each incident:
   - simple 2D route trace;
   - observer/candidate positions over time;
   - camera direction when available;
   - raised/ADS state;
   - visibility timeline;
   - shots, hits and deaths;
   - cue classification;
   - matched control explanation;
   - uncertainty notes;
7. squad observations where applicable;
8. admin disposition: no concern, monitor, request more evidence, confirmed by separate evidence, or false-positive category.

Admin disposition supports evaluation but is not automatically ground-truth cheat labelling.

## Performance requirements

### DayZ client

- accumulator sampling, not unbounded per-frame work;
- bounded ring buffers and RPC batches;
- no remote-player enumeration;
- no synchronous network or disk I/O;
- safe degradation when the client stalls.

### DayZ server

- no `POST_now`;
- no database access;
- no full pairwise raycast sweep;
- visibility work queued and globally budgeted;
- bounded export queue and spool;
- `TickCount` instrumentation around sampling, candidate selection, probes, serialisation and export dispatch;
- performance gate set from representative load tests, not guessed.

### Go service

- acknowledge ingestion only after durable raw storage;
- asynchronous feature calculations;
- algorithm/schema version attached to derived values;
- raw-event replay into a clean database.

## Trust and security model

1. Client telemetry is untrusted.
2. RPC sender identity is authoritative for attribution; client-supplied identity is ignored.
3. Sequences/nonces prevent accidental duplication and basic replay but do not make the client tamper-proof.
4. Server-to-Go transport is authenticated separately.
5. Bind the service to localhost/private networking by default.
6. Sensitive identifiers are access-controlled and excluded from public logs.
7. Retention is configurable and documented.

## Failure handling

### Client-to-server

- reject malformed/oversized batches;
- record missing sequences without punishment;
- rate-limit RPC flood per identity;
- safely disable unsupported schema/mod versions.

### Server-to-Go

- memory queue to configured limit;
- rotated spool files when the service is unavailable;
- preserve authoritative combat/lifecycle events before optional high-rate camera data;
- asynchronous retry with bounded backoff;
- Go idempotency prevents duplicate normalisation.

### Analysis

- insufficient samples: no ranking;
- poor controls: low confidence;
- missing camera data: suppress camera-dependent features;
- unknown visibility: exclude from strong hard-occlusion features.

## Testing seams

### Client state-to-event seam

A pure component accepts successive samples and emits transitions. It is testable without camera or RPC calls.

### Server batch-validation seam

Accepts sender identity, schema, sequence, nonce and payload metadata; returns accept/reject plus reason.

### Visibility seam

Returns `HARD_OCCLUDED`, `GEOMETRICALLY_EXPOSED` or `UNKNOWN`. Production uses DayZ physics; tests use recorded fixtures.

### Export seam

Server batcher writes through an exporter interface:

- asynchronous REST;
- disk spool;
- development log.

### Go feature seam

Each calculator consumes a versioned timeline/observation model and emits values plus evidence references.

### End-to-end replay seam

A captured NDJSON controlled-session fixture replays through ingestion, normalisation and analysis with deterministic review results.

## Delivery milestones

### Milestone 0: DayZ feasibility and performance spike

Prove and profile:

- modded `MissionGameplay.OnUpdate` sampling;
- camera position/direction capture;
- ironsight/optics/third-person/raised-state capture;
- client-to-server `ScriptRPC` batching and receive path;
- `g_Game.GetPlayers` server snapshots;
- `Weapon_Base.OnFire` execution context on client and dedicated server;
- `PlayerBase.EEHitBy` and `EEKilled` capture;
- head/torso bone lookup across stances;
- visibility probes against terrain, buildings, doors and base-building objects;
- asynchronous localhost `RestContext.POST`;
- disk spool and replay;
- CPU timing and payload size under representative player counts.

Exit criteria:

- every required surface works on the target DayZ version or the spec is amended with a verified replacement;
- no unverified hook remains in the MVP;
- safe initial sampling/probe budgets are documented.

### Milestone 1: Versioned telemetry collector

- client sampler, ring buffer and batches;
- server identity binding, validation and sequences;
- snapshots and lifecycle/combat events;
- bounded decision transition detector;
- health metrics and NDJSON development output.

### Milestone 2: Go ingestion and replay

- versioned endpoint;
- durable raw storage;
- PostgreSQL normalisation;
- idempotency/replay command;
- Docker Compose deployment;
- ingestion metrics.

### Milestone 3: Visibility scheduler

- event-gated candidate selection;
- bounded probes;
- diagnostic pair windows;
- degradation and workload metrics.

### Milestone 4: Initial explainable analysis

- hidden-threat readiness lift;
- correct sector selection;
- pre-exposure readiness;
- post-engagement handoff;
- telemetry integrity;
- controls and minimum evidence gates;
- JSON/CSV/basic API output before a polished UI.

### Milestone 5: Review experience and squad analysis

- route/timeline visualisation;
- case workflow;
- optional group adapter;
- inferred group analysis with lower confidence;
- admin feedback categories.

## User stories

1. As a server administrator, I want players ranked for manual review so limited review time is focused effectively.
2. As an administrator, I want every ranking backed by incidents, rates, controls and sample counts.
3. As an administrator, I want uncertainty and plausible legitimate cues shown explicitly.
4. As an administrator, I want to filter candidates by server, date, squad and confidence.
5. As an administrator, I want to replay a short route/state timeline around an incident.
6. As an operator, I want expensive analysis outside the DayZ process.
7. As an operator, I want strict budgets for raycasts, queues, RPCs and exports.
8. As an operator, I want collection to survive sidecar outages and replay spooled data.
9. As an operator, I want health metrics for samples, RPC rejects, probes, exports and spool usage.
10. As a developer, I want immutable raw events and versioned schemas.
11. As a developer, I want every DayZ surface documented and verified against the target version.
12. As an analyst, I want matched controls and relevant cohorts.
13. As an analyst, I want squad-level patterns because the radar user may not take the shots.
14. As a reviewed player, I must never be automatically banned by this system alone.

## Acceptance criteria

1. Only verified DayZ script surfaces documented here are used.
2. Weapon raise, ADS and route changes are sampled transitions, not fictional hooks.
3. Client telemetry is explicitly untrusted in schema and analysis.
4. Server attribution ignores client-supplied identity.
5. DayZ performs no lifetime scoring or database queries.
6. DayZ uses no blocking REST call.
7. Visibility probes are event-gated, queued and bounded.
8. Ambiguous visibility is never promoted into strong evidence.
9. Go ingestion durably stores raw data before acknowledgement.
10. Raw data is replayable after feature changes.
11. Rankings require multiple independent observations, targets and sessions.
12. Every candidate exposes rates, controls, counts and confidence.
13. There is no automatic ban/kick integration.
14. Representative load testing meets the operator-approved overhead tolerance.
15. Trusted controlled sessions are used to calibrate false-positive patterns before production ranking.

## Evaluation plan

1. Run silently for a calibration period.
2. Include trusted regulars, strong PvP players and administrators performing controlled behaviours.
3. Generate blind rankings where practical.
4. Compare top-ranked cases with random cases.
5. Continue only if the top-ranked group consistently produces more review-worthy unexplained behaviour than random selection.
6. Record common false-positive categories and improve controls rather than only raising thresholds.

Success means improved review efficiency, not authoritative cheat detection.

## Risks

1. Client telemetry can be forged or suppressed.
2. Discord/external callouts are not observable.
3. Exact sound and visual perception cannot be reconstructed.
4. Skilled players and map specialists can be behavioural outliers.
5. Sparse combat may require long collection windows.
6. Group membership may be missing or inferred incorrectly.
7. Probes can become expensive if budgets fail.
8. Dynamic/modded geometry can produce ambiguous classifications.
9. A polished dashboard can create false confidence.
10. Cheat users may adapt once detection concepts become known.

## Open decisions

1. Target DayZ server version and maps for the spike.
2. Standalone client/server mod versus extension of an existing package.
3. Initial group/clan adapter, if any.
4. Raw compressed files plus PostgreSQL versus PostgreSQL-only raw storage.
5. Retention values.
6. Review UI technology after JSON/CSV proves value.
7. Existing anti-cheat events that can be imported as context without coupling enforcement.
8. Trusted calibration cohort definition.
9. Operational threshold for creating a review case, chosen only after calibration.

## Recommended first decision

Proceed with Milestone 0 and Milestones 1-2 only. Do not build a polished review UI or sophisticated scoring until real telemetry demonstrates that hard-occluded readiness and sector-selection features produce better review candidates than random sampling.
