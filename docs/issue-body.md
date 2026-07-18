# Spec: Behavioural awareness telemetry for DMA/radar review prioritisation

## Status and decision

Build an evidence and review-prioritisation system only. It must never automatically ban, kick or punish a player.

The system targets external-information cheats such as DMA radar and second-PC ESP that may not modify DayZ state or produce mechanically impossible shots. It looks for repeated gameplay decisions that correlate unusually strongly with hidden-player information, then presents the strongest patterns to administrators for manual review.

The long-form specification is in [`docs/behavioural-awareness-spec.md`](./behavioural-awareness-spec.md). The verified API register is in [`docs/dayz-script-surface-register.md`](./dayz-script-surface-register.md). This issue is the implementation contract.

## Problem statement

Existing anti-cheat tools catch many lazy cheats through impossible movement, invalid inventory actions, redirected bullets and crude aim manipulation. A read-only external radar can avoid those signatures.

The radar advantage must still be expressed through normal inputs and decisions, including:

- slowing or stopping before an unseen threat;
- raising a weapon before legitimate contact;
- selecting the occupied route, sector, window or flank;
- efficiently pursuing a concealed player;
- avoiding occupied positions while treating equivalent empty positions normally;
- preparing for the correct emergence point before exposure;
- passing concealed-player information to teammates.

Random review is inefficient. Administrators need accounts and squads ranked by repeated hidden-awareness patterns, with explainable evidence and uncertainty.

## Solution

Build three separately owned components:

```text
DayZ client mod
  -> local camera/combat-state samples
  -> bounded ring buffer and ScriptRPC batches

DayZ server mod
  -> authenticated identity binding
  -> authoritative positions/lifecycle/combat events
  -> bounded live visibility probes
  -> async export and disk spool

External Go service
  -> durable ingestion
  -> timeline reconstruction
  -> matched controls and outlier analysis
  -> review cases and evidence
```

DayZ collects only information that requires the live engine. The Go service owns all expensive historical and population-wide analysis.

## Real DayZ script surfaces

Implementation must use and revalidate these existing surfaces against the target DayZ version:

| Requirement | Verified script surface |
|---|---|
| Client periodic sampling | `MissionGameplay.OnUpdate(float timeslice)` using an accumulator, or bounded timer/call queue |
| Server periodic sampling | `MissionServer.OnUpdate(float timeslice)` and gameplay call queue |
| Enumerate active players | `g_Game.GetPlayers(array<Man>)`, as used by `MissionServer.UpdatePlayersStats` |
| Local camera origin/direction | `g_Game.GetCurrentCameraPosition()` and `g_Game.GetCurrentCameraDirection()` demonstrated by rangefinder and weapon-debug scripts |
| Raised/ADS/optic/camera mode | `DayZPlayerImplement.IsFireWeaponRaised()`, `IsInIronsights()`, `IsInOptics()`, `IsInThirdPerson()` |
| Client telemetry transport | `ScriptRPC.Write(...)` and `ScriptRPC.Send(...)`, received through a real modded `OnRPC(...)` surface |
| Successful-fire seam | `WeaponFire.OnEntry` calls native `TryFireWeapon(...)`, then `m_weapon.OnFire(muzzleIndex)` on success |
| Hit/death events | `PlayerBase.EEHitBy(...)` and `PlayerBase.EEKilled(...)` |
| Live visibility probes | `DayZPhysics.RaycastRV(...)` / `RaycastRVProxy(...)` using `ObjIntersectView` |
| External export | asynchronous `RestContext.POST(...)`; blocking `POST_now(...)` is prohibited |
| Local fallback spool | `OpenFile`, `FPrint`, `CloseFile`, `JsonSerializer` / `JsonFileLoader` |
| Cost measurement | `TickCount(...)` and server operational metrics |

Source paths and spike-required assumptions are documented in `docs/dayz-script-surface-register.md`.

No fictional callback such as `OnWeaponRaised` may be introduced. Weapon raise, ADS and route changes are sampled transitions.

## Ownership

### DayZ client mod

Owns:

- sampling only the local player;
- camera position/direction;
- raised, ironsight, optics and third-person state;
- deriving state transitions from successive samples;
- a bounded local ring buffer;
- same-frame local shot sample through a modded `Weapon_Base.OnFire` where local ownership is confirmed;
- batched `ScriptRPC` delivery;
- client sequence, server nonce and telemetry-health counters.

Does not:

- enumerate remote players;
- decide whether another player is hidden;
- calculate suspicion or review priority;
- contain a secret assumed safe from client tampering.

### DayZ server mod

Owns:

- binding RPC batches to the server-supplied `PlayerIdentity`;
- schema, size, sequence, nonce and rate validation;
- server session/player-session IDs and sequences;
- connect, ready, respawn, reconnect and disconnect events;
- low-rate authoritative player snapshots;
- server-derived speed, movement heading, start/stop and route-heading changes;
- authoritative hit/death events;
- server shot markers only if the spike proves the seam executes reliably on dedicated server;
- event-gated nearby-target candidate selection;
- bounded live-world visibility probes;
- `HARD_OCCLUDED`, `GEOMETRICALLY_EXPOSED` or `UNKNOWN` classification;
- bounded queues/ring buffers;
- asynchronous export, disk spool and collector metrics.

Does not:

- run lifetime/population scoring;
- query a database;
- generate matched controls;
- infer DMA use;
- trigger enforcement.

### External Go service

Owns:

- authenticated versioned ingestion;
- durable raw storage before acknowledgement;
- de-duplication and normalisation;
- client/server clock alignment with uncertainty;
- session, route, encounter and observer-target reconstruction;
- matched controls and cohorts;
- behavioural features and outlier ranking;
- squad-level information-propagation analysis;
- explainable review packages;
- raw replay after algorithm changes;
- retention, review API and later UI.

Does not:

- claim exact visual/audible perception;
- trust client telemetry as authoritative;
- ban players;
- affect gameplay in the MVP.

## Collection design

### Client samples

Use an accumulator in `MissionGameplay.OnUpdate`:

- baseline: 2 Hz while alive/in gameplay;
- raised weapon or ADS/optics: 10 Hz;
- local combat/diagnostic window: 10 Hz for 10-15 seconds;
- dead/unconscious/outside gameplay: stop or lifecycle-only.

Sample fields:

```text
schema_version
client_sequence
client_monotonic_time_ms
camera_position
camera_direction
client_observed_player_position
is_weapon_raised
is_in_ironsights
is_in_optics
is_in_third_person
item_in_hands_type_id
sample_flags
```

Batch at most once per second normally, cap batch count/bytes, never block for acknowledgement, and report dropped samples.

### Server snapshots

Initial cadence: 2 Hz.

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

### Decision events

Client-derived sampled transitions:

- `WEAPON_RAISED` / `WEAPON_LOWERED`;
- `ADS_ENTERED` / `ADS_EXITED`;
- `CAMERA_TURN` after configured angular movement;
- `SHOT_FIRED_CLIENT` from local `Weapon_Base.OnFire`.

Server-derived transitions/events:

- `MOVEMENT_STARTED` / `MOVEMENT_STOPPED`;
- `ROUTE_HEADING_CHANGED` after sufficient displacement;
- `PLAYER_HIT`;
- `PLAYER_KILLED`;
- `SHOT_FIRED_SERVER` only if verified.

Thresholds control observation generation, not cheat verdicts.

## Visibility probes

The Go service cannot reproduce current doors, vehicles, base-building objects and modded collision state. DayZ therefore performs limited visibility probes at decision time.

When a meaningful decision occurs:

1. identify the observer;
2. select nearby valid players from the current snapshot;
3. optionally exclude known group members through an adapter;
4. sort by distance;
5. select no more than a configured maximum, initially three within a configurable 1,000 m radius;
6. queue probes rather than running an unbounded loop.

For each pair:

1. use a validated server-owned observer head/eye point;
2. use validated target head and torso points;
3. perform at most two `ObjIntersectView` probes;
4. exclude the observer;
5. classify:
   - `GEOMETRICALLY_EXPOSED`: at least one target point reached without a blocker;
   - `HARD_OCCLUDED`: both blocked by recognised hard geometry;
   - `UNKNOWN`: vegetation, ambiguous result, invalid bones/entities or incomplete work.

Probe budgets:

- global per-update maximum;
- per-observer and per-pair cooldowns;
- bounded queue;
- capped diagnostic pair windows;
- queued/completed/dropped/failed metrics.

Optional pair windows may probe a high-value observer/target pair at 2-5 Hz for up to five seconds after a readiness event. They must be globally capped.

## Captured cue ledger

Captured explanations:

- target recently geometrically exposed;
- observer or known group member recently hit/killed target;
- target recently fired within a conservative distance;
- observer recently hit;
- ongoing engagement with the same target/group;
- target recently known within a memory window.

Cue labels:

- `KNOWN`;
- `PLAUSIBLE`;
- `UNEXPLAINED_IN_CAPTURED_DATA`.

The final label does not mean impossible legitimately. Footsteps, distant audio and Discord calls are not observable reliably.

## Go service MVP

Suggested packages:

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

Ingestion endpoint:

```text
POST /v1/telemetry/batches
```

Requirements:

- authenticated server;
- request size and schema validation;
- idempotency by `(server_id, batch_sequence)`;
- durable raw write before success;
- no feature calculation in request path;
- structured rejection reasons;
- health/readiness/metrics endpoints.

Storage recommendation:

- compressed append-only raw batches for exact replay;
- PostgreSQL for normalised events, observations, features and cases;
- time partitions and configurable retention.

Preserve client timestamp, server receipt/event timestamps, sequences, estimated offset and timing uncertainty.

## Initial behavioural features

### Hidden-threat readiness lift

Compare rates of weapon raise, ADS, movement stop/slowdown and route change when a hard-occluded enemy exists against matched empty/control situations.

### Correct sector selection

Measure whether deliberate camera/route changes select sectors containing hard-occluded players more often than the player’s normal scan distribution and matched controls.

### Pre-exposure readiness

Measure readiness-to-first-geometric-exposure timing and angular relation, discounting recently known targets and plausible cues.

### Hidden pursuit efficiency

Measure route convergence on concealed players, target-following route changes and bypassed search sectors. This is lower-confidence and requires larger samples.

### Hidden-threat avoidance

Measure stops, reversals and detours before concealed contact against empty matched routes. Supporting-only; never dominant.

### Post-engagement concealed-target handoff

After hit/kill events, measure immediate turning, movement or cover selection relative to another concealed and previously unexplained player.

### Squad information propagation

Use a real group/clan adapter where available. Otherwise inferred co-movement groups are clearly labelled lower-confidence. Do not claim which account owns a radar.

### Telemetry integrity

Track missing/replayed sequences, timestamp drift, combat-correlated gaps and impossible camera/player relationships. These are supporting health signals, not DMA evidence by themselves.

## Matched controls and ranking

Match controls where data permits on:

- map/area;
- distance band;
- observer movement/combat state;
- weapon/ADS baseline;
- time since combat;
- population band;
- first-/third-person;
- hotspot classification;
- recent cue/exposure state.

Use the same player in equivalent empty situations first, then trusted/contextual cohorts and synthetic empty sectors where geometry permits.

Ranking is explainable weighted evidence with minimum gates for observations, unrelated targets, distinct sessions and control quality. Missing telemetry and plausible cues reduce confidence.

Never display `cheat probability`. Use:

- review priority;
- hidden-awareness outlier percentile;
- unexplained observations;
- supporting pattern;
- data confidence.

## Review output

Each case must include:

- player/session summary;
- feature percentiles, raw and control rates;
- observations, unrelated targets and session counts;
- telemetry confidence;
- strongest incidents;
- simple 2D positions/routes over time;
- camera direction when available;
- raised/ADS state;
- visibility and cue timeline;
- combat events;
- matched-control explanation;
- uncertainty notes;
- squad context;
- admin disposition.

Admin disposition supports evaluation but is not automatically a cheat label.

## Performance and safety constraints

- no unbounded per-frame work;
- no client remote-player enumeration;
- no synchronous network/disk I/O on client;
- no `POST_now` on server;
- no database work in DayZ;
- no full pairwise raycast sweep;
- all queues, buffers, batches, probe work and diagnostic windows bounded;
- overload drops optional observations rather than delaying gameplay;
- instrument collector stages with `TickCount`;
- determine safe budgets through representative load testing.

## Milestones

### Milestone 0 — feasibility/performance spike

Prove on the target DayZ version:

- client accumulator sampling and local states;
- RPC batching and authenticated receive path;
- server player snapshots/lifecycle;
- client/dedicated-server execution context of `Weapon_Base.OnFire`;
- `EEHitBy` / `EEKilled` data;
- bone probe points across stances;
- visibility behaviour for terrain, buildings, doors, base building, vehicles and vegetation;
- async localhost `RestContext.POST`;
- rotated spool and replay;
- payload and CPU cost under representative player counts.

Exit: every required surface is proven or replaced with a verified alternative; no assumed hook remains.

### Milestone 1 — versioned collector

Client sampler/ring buffer/RPC, server validation/identity/sequences, snapshots, lifecycle/combat, bounded transitions, health metrics and NDJSON development output.

### Milestone 2 — Go ingestion/replay

Versioned endpoint, durable raw batches, PostgreSQL normalisation, idempotency/replay, Docker Compose and metrics.

### Milestone 3 — visibility scheduler

Event-gated candidates, bounded probes, pair windows and workload/degradation metrics.

### Milestone 4 — explainable analysis

Readiness lift, sector selection, pre-exposure readiness, concealed handoff, integrity, controls and evidence gates. Output JSON/CSV/basic API before polished UI.

### Milestone 5 — review and squad analysis

Route/timeline view, case workflow, optional group adapter, inferred-group analysis and admin feedback.

## Acceptance criteria

- [ ] Every DayZ integration uses a verified surface documented in this repository.
- [ ] Weapon raise, ADS and route changes are sampled transitions, not fictional callbacks.
- [ ] Client telemetry is labelled untrusted in schema and code.
- [ ] RPC attribution ignores client-supplied identity.
- [ ] DayZ performs no lifetime scoring or database queries.
- [ ] DayZ uses no blocking REST call.
- [ ] Visibility work is event-gated, queued and bounded.
- [ ] Ambiguous visibility never becomes strong hard-occlusion/exposure evidence.
- [ ] Go ingestion stores raw data durably before acknowledgement.
- [ ] Raw telemetry can be replayed after algorithm changes.
- [ ] Rankings require multiple independent observations, targets and sessions.
- [ ] Every case exposes rates, controls, counts, confidence and uncertainty.
- [ ] There is no automatic ban/kick integration.
- [ ] Representative load testing meets the operator-approved overhead tolerance.
- [ ] Trusted controlled sessions calibrate false-positive patterns before operational ranking.

## Evaluation gate

Run silently during calibration with trusted regulars, strong PvP players and controlled admin scenarios. Compare the top-ranked engagements/accounts against randomly selected cases, ideally blind.

Continue beyond the prototype only if the ranked group consistently produces more review-worthy unexplained behaviour than random review.

## Recommended implementation decision

Proceed with Milestone 0 and Milestones 1-2 only. Do not build a polished UI or sophisticated scoring until collected telemetry demonstrates that hidden-threat readiness and sector-selection features improve review efficiency.
