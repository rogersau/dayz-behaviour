# Architecture

## Purpose

DayZ Behaviour collects a bounded set of game observations and uses them to identify repeated hidden-awareness patterns for manual administrator review.

The design has four rules:

1. collection must remain bounded and lightweight inside DayZ;
2. server-authoritative facts must remain distinct from client observations;
3. missing or uncertain data must reduce confidence rather than create suspicion;
4. raw evidence and algorithm versions must remain replayable and auditable.

The system does not identify cheat software, recreate a player’s screen or audio, or automatically enforce punishments.

## System overview

```text
┌──────────────────────── DayZ process ────────────────────────┐
│                                                              │
│  Client collector                   Server collector          │
│  - camera direction                 - direct player identity  │
│  - raised/ADS/optics state          - lifecycle and combat    │
│  - transition intervals             - authoritative position  │
│  - collector health                 - audio opportunities      │
│           │                          - bounded visibility       │
│           └──────── authenticated RPC ────────┘               │
│                                               │               │
│                                  asynchronous HTTP export     │
└───────────────────────────────────────────────┼───────────────┘
                                                ▼
                                local server agent executable
                                      durable disk outbox
                                                │
                                      authenticated HTTPS
                                                ▼
                                        central ingestd
                                                │
                                  immutable raw JSON batches
                                                │
                         ┌──────────────────────┴───────────────────┐
                         ▼                                          ▼
                     normalize                                  analyse
                         │                                          │
                  PostgreSQL records                      observations/features
                         └──────────────────────┬───────────────────┘
                                                ▼
                                            reviewd
                                      API + admin explorer
```

A same-host evaluation deployment may still send the DayZ exporter directly to `ingestd`. Distributed deployments use the standalone server agent so central outages and internet failures do not block or slow the DayZ process.

## Components

### DayZ client collector

The client observes information available only on the local client:

- camera position and direction;
- weapon-raised, iron-sight, optics, and third-person state;
- bounded transition detection;
- a short pre-event sample ring and post-event burst;
- client collector health.

This is Tier B supporting context. The server attributes every RPC using its own `PlayerIdentity`; client-supplied identity is not accepted.

### DayZ server collector

The server owns the authoritative game-side context:

- direct durable player identity and player-session lifecycle;
- authoritative position, velocity, alive, and unconscious state;
- hit, kill, and fire events where the script callback is valid;
- gunshot and movement-audio opportunities;
- random prospective sampling opportunities;
- bounded observer/target visibility probes;
- clock challenges, queue health, export health, and spool state.

It performs only bounded current-world work. It does not scan every player pair every frame or calculate historical rankings.

### Server agent

The standalone server agent is the normal boundary for a DayZ server outside the central host. It:

- listens only on loopback for the existing DayZ exporter;
- authenticates and validates the local batch;
- requires the batch `server_id` to match its configuration;
- commits the batch to a bounded durable disk outbox before acknowledging DayZ;
- imports the DayZ mod emergency spool when configured;
- forwards the original batch to central `ingestd` over authenticated HTTPS;
- retries transient failures and isolates permanent upstream rejections in a dead-letter directory.

It performs no behavioural analysis and does not alter the telemetry payload.

### `ingestd`

`ingestd` is the central or same-host HTTP receiver. It:

- authenticates the same-host DayZ exporter or remote server agent;
- validates batch and event invariants;
- persists each batch as an immutable, fsynced JSON file;
- imports failed-export spool files;
- rejects an existing batch identity when its contents differ.

The DayZ query token is intended only for the loopback DayZ-to-agent or same-host boundary. Distributed agents use server-specific bearer credentials over HTTPS, and central ingest binds each credential to one configured `server_id`.

### `normalize`

`normalize` replays raw batches into PostgreSQL. It stores direct player and session identifiers exactly as collected, along with:

- normalized events;
- player sessions;
- sampling opportunities;
- visibility probe results;
- source authority and schema versions.

Normalization is idempotent. Raw files remain the source of truth.

### `analyse`

`analyse` rebuilds observations from raw telemetry, enriches them with cue facts, calculates features, applies validation gates, and optionally persists results.

The primary feature compares reactions during validated concealed-target opportunities with reactions during neutral no-relevant-target opportunities.

### `reviewd`

`reviewd` exposes:

- a Steam-authenticated browser explorer;
- bearer-authenticated review APIs;
- direct `player_id` and player-session identifiers;
- ordered timelines with authority and payload context;
- local DayZ maps and spatial evidence;
- review cases and audited dispositions.

It is read-only with respect to DayZ. It cannot ban, kick, message, or otherwise alter a player in game.

## Trust and authority

| Tier | Meaning | Examples |
|---|---|---|
| A | Server-authoritative game fact | identity, lifecycle, position, combat callback, visibility geometry |
| B | Untrusted client observation | camera sample, ADS/optics transition, client timing interval |
| C | Collector or pipeline health | queue depth, drops, export failures, clock quality |

Tier B data can describe when or how a player reacted. It cannot create strong hidden-target evidence without corresponding Tier A context.

Tier C data never strengthens a player signal. Missing telemetry, excessive uncertainty, queue delay, or collection loss can only suppress eligibility or lower confidence.

## Identity and sessions

The server records the durable identity returned by `PlayerIdentity.GetId()`. On Steam servers this is normally the SteamID64 used by other moderation systems.

Each connected lifecycle also receives a session ID:

```text
<server-session-id>:<session-sequence>:<durable-player-id>
```

Direct IDs are stored throughout raw data, PostgreSQL, analysis output, review APIs, and the explorer. There is no anonymized identity mode or compatibility alias.

Because identities are direct, access control and retention are the privacy boundary. Databases, API responses, screenshots, exports, logs, and backups must be treated as sensitive administrative data.

## Audio-cue model

The system records no microphone input and no raw game audio. It creates server-derived **audio opportunities**.

### Gunshots

`Weapon_Base.EEFired(int muzzleType, int mode, string ammoType)` records:

- shooter identity and session;
- server timestamp and position;
- weapon and ammunition type;
- muzzle type and fire mode;
- suppressor presence and type.

The Go model compares the shot position with the observer’s latest authoritative position at or before the shot and assigns:

- `CAPTURED_STRONG_CUE`;
- `LIKELY_AUDIO_CUE`;
- `POSSIBLE_AUDIO_CUE`;
- `NOT_AUDIBLE_BY_MODEL`.

Suppressed and unsuppressed shots use separate versioned range bands.

### Footsteps and movement

At a bounded server cadence, moving players generate `MOVEMENT_AUDIO_OPPORTUNITY` events containing:

- authoritative position and velocity-derived speed;
- a coarse gait band;
- stance;
- surface type from `SurfaceGetType3D`;
- the item in the `Feet` attachment slot.

Go estimates likely and maximum footstep ranges from these fields. It does not reproduce the complete DayZ sound engine, indoor acoustics, weather masking, animation-specific samples, hearing damage, or client volume settings.

### Cue safety

A cue means:

> Captured game state supports a plausible opportunity for the observer to hear the target.

It does not mean:

> The observer definitely heard and correctly localized the target.

Strong or likely cues may classify an observation as known or plausible. Possible cues remain visible to reviewers but do not automatically suppress primary analysis.

## Time model

DayZ server time is monotonic only within one `server_session_id`. Events from different server sessions are never joined by relative timestamp.

Client transitions are sampled, so the true transition occurred within an interval:

```text
event_time_lower_ms <= true event time <= event_time_upper_ms
```

The server issues single-use clock challenges and maps client intervals into server time using the latest accepted offset and uncertainty. Missing or poor alignment suppresses timing-sensitive evidence.

## Sampling model

### Random prospective opportunities

Random opportunities are the primary statistical stream. The collector records risk-set size, inclusion probability, queue admission, queue delay, load state, and policy version.

Opportunity types are:

- validated concealed target — hidden condition;
- no relevant target inside the configured risk set — neutral control;
- exposed or partially exposed target — visible positive control.

Visible targets are not the neutral baseline.

### Event enrichment

A compact client decision edge can request bounded, lower-priority probing around one observer/target pair. This helps reconstruct timing and direction but remains separate from the random stream to avoid selection bias.

### Independence

Closely spaced observations are grouped into refractory windows, target episodes, and encounters. Evidence breadth counts distinct sessions, encounters, and durable target identities rather than raw event volume.

## Visibility safety

A server head-origin ray is not automatically equivalent to what a player could see, especially in third person.

Strong concealed-target evidence requires:

- Tier A server geometry;
- a server that enforces first person;
- `server_first_person_only=true`;
- `visibility_origin_mode=VALIDATED_FIRST_PERSON_HEAD`;
- a recorded validation ID;
- repeated occlusion for the configured duration;
- acceptable queue and timing quality.

Without those conditions, visibility remains descriptive context.

## Storage and replay

Raw batches are stored under:

```text
<raw-root>/<server-id>/<server-session-id>/<batch-sequence>.json
```

Writes use a temporary file, fsync, atomic rename, and directory sync where supported. Reusing a batch identity with different contents is an error.

PostgreSQL stores normalized events, sessions, sampling opportunities, visibility evidence, observations, features, rankings, cases, and dispositions. Derived data can be rebuilt from retained raw batches.

Transport, sampling, visibility, observation, cue, feature, matching, and ranking policies carry explicit version identifiers. SQL migrations are checksum-verified.

## Failure behaviour

The system fails conservatively:

- network failure spools export batches to disk;
- queues and spool files are bounded and measured;
- stale observer/target entities are revalidated before probes;
- malformed, unauthorized, conflicting, or impossible batches are rejected;
- missing maps are not substituted with another terrain;
- missing telemetry or failed validation gates suppresses review tiers;
- no failure mode creates automatic enforcement.
