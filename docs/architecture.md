# Architecture

## Purpose

DayZ Behaviour collects a deliberately limited set of game observations and uses them to identify repeated hidden-awareness patterns for manual administrator review.

The system is designed around five constraints:

1. DayZ collection must remain bounded and should not perform historical analysis in the game loop.
2. Server-authoritative facts and untrusted client context must remain distinguishable.
3. Plausible legitimate explanations must be retained alongside suspicious-looking behaviour.
4. Uncertainty and missing information must reduce confidence rather than create suspicion.
5. Historical data and algorithm versions must remain replayable and auditable.

It is not designed to identify cheat software, prove the presence of DMA hardware, reproduce a player’s rendered screen or audio, or automatically enforce a punishment.

## System overview

```text
┌──────────────────────── DayZ process ────────────────────────┐
│                                                              │
│  Client collector                   Server collector          │
│  - camera direction                 - direct player identity  │
│  - raised/ADS/optics state          - lifecycle and combat    │
│  - transition intervals             - authoritative position  │
│  - collector health                 - gunshot opportunities   │
│                                     - movement audio context  │
│                                     - bounded visibility       │
│           │                                   │               │
│           └──────── authenticated RPC ────────┘               │
│                                               │               │
│                                  asynchronous HTTP export     │
└───────────────────────────────────────────────┼───────────────┘
                                                ▼
                                      ingestd loopback sidecar
                                                │
                                  immutable raw JSON batches
                                                │
                         ┌──────────────────────┴───────────────────┐
                         ▼                                          ▼
                     normalize                                  analyse
                         │                                  - observations
                  PostgreSQL records                       - audio cues
                         │                                  - features
                         └──────────────────────┬───────────────────┘
                                                ▼
                                            reviewd
                                      API + admin explorer
```

## Components

### DayZ client collector

The client collector observes information available only on the local client, including:

- camera position and direction;
- weapon-raised, iron-sight, optics, and third-person state;
- bounded local transition detection;
- a short pre-event sample ring and post-event burst samples;
- client collector health.

This data is useful for reconstructing behaviour, but it is not trusted as proof. The server attributes RPCs using its own `PlayerIdentity`; client-supplied identity is not accepted.

### DayZ server collector

The server collector owns the authoritative game-side context:

- direct DayZ/Steam identity and player-session lifecycle;
- authoritative position, movement, alive, and unconscious state;
- hit, kill, and server-side fire events where the script seam is valid;
- movement-audio opportunities containing speed, stance, surface, footwear, and position;
- random prospective sampling opportunities;
- bounded observer/target visibility probes;
- clock challenges, queue health, export health, and spool state.

The collector performs only bounded current-world work. It does not scan every player pair every frame and does not calculate long-term player rankings.

### `ingestd`

`ingestd` is the local HTTP receiver. It:

- requires an ingest credential unless explicitly placed in unauthenticated development mode;
- validates the batch envelope and event invariants;
- persists each batch as an immutable, fsynced JSON file;
- imports DayZ failed-export spool files;
- rejects an existing batch identity when its contents differ.

The DayZ query token exists because `RestContext` cannot set a general authorization header. That endpoint is intended for loopback or a private sidecar boundary, not direct internet exposure.

### `normalize`

`normalize` replays immutable raw batches into PostgreSQL. It:

- preserves direct durable-player and player-session identifiers;
- stores events, sessions, sampling opportunities, visibility runs, and analysis inputs;
- preserves source authority and algorithm/schema versions;
- applies database migrations and verifies migration checksums.

Normalization is idempotent. Raw files remain the source of truth for replay.

### `analyse`

`analyse` rebuilds observations from raw telemetry, enriches them with captured cue facts, calculates features, applies validation gates, and optionally persists results to PostgreSQL.

The primary feature family compares reactions during validated concealed-target opportunities with reactions during neutral no-relevant-target opportunities. Audio cues and other legitimate explanations are applied before the primary unexplained-observation filters.

### `reviewd`

`reviewd` exposes:

- a Steam-authenticated browser explorer;
- bearer-authenticated review APIs;
- direct player and player-session identifiers for cross-reference;
- ordered timelines with authority and payload context;
- local DayZ map tiles and spatial evidence;
- review cases and audited dispositions.

It is read-only with respect to DayZ. No endpoint can ban, kick, message, or alter a player in the game.

## Trust and authority

Events retain an authority tier throughout ingestion, normalization, analysis, and review.

| Tier | Meaning | Examples |
|---|---|---|
| A | Server-authoritative game fact | lifecycle, position, movement, combat callback, visibility geometry |
| B | Untrusted client observation | camera sample, ADS/optics transition, local timing interval |
| C | Collector or pipeline health | queue depth, drops, export failures, clock quality |

Tier B data can help explain when or how a player reacted. It cannot create strong hidden-target evidence without corresponding Tier A geometry and acceptable timing.

Tier C data never strengthens a player signal. Missing data, excessive timing uncertainty, queue delay, or collection loss can only suppress eligibility or lower confidence.

## Identity and sessions

The server records the durable DayZ identity returned by `PlayerIdentity.GetId()`. For Steam deployments this is normally the SteamID64 used by other server and moderation systems.

The server also assigns a distinct `player_session_id` for each connected character/session lifecycle:

```text
<server-session-id>:<session-sequence>:<durable-player-id>
```

Client sequence and rate-limit state is reset when the player disconnects or reconnects, so a restarted collector can safely begin its sequences again.

Direct identities are preserved in normalized and review data. This makes operational cross-reference straightforward, but it also means the following are sensitive personal data:

- PostgreSQL tables;
- review API responses;
- explorer screenshots and exports;
- raw batches and logs;
- backups and copied reports.

Access control and retention are therefore the privacy boundary; pseudonymization is no longer used.

### Upgrading from pseudonymized data

One-way historical pseudonyms cannot be converted back to Steam IDs. The supported transition is:

1. retain and back up restricted raw batches;
2. dry-run `cmd/direct-identity-rebuild`;
3. clear derived normalized/review data through the command;
4. switch the database identity policy to `direct-identifiers-v1`;
5. replay raw batches;
6. rerun analysis.

Review dispositions stored only in cleared derived tables are not recreated by raw replay.

## Audio-cue model

The system does not record microphone, game-mix, or raw audio samples.

Instead, it creates server-derived **audio opportunities**.

### Gunshots

The weapon callback records:

- shooter identity and player session;
- server timestamp and position;
- weapon and ammunition type;
- muzzle index;
- suppressor presence and type.

The Go audio model compares the shot position with the observer’s nearest authoritative position sample and assigns one of:

- `CAPTURED_STRONG_CUE`;
- `LIKELY_AUDIO_CUE`;
- `POSSIBLE_AUDIO_CUE`;
- `NOT_AUDIBLE_BY_MODEL`.

The initial ranges are conservative policy values and are versioned. Suppressed and unsuppressed shots use different bands.

### Footsteps and movement

At a bounded server cadence, moving players generate `MOVEMENT_AUDIO_OPPORTUNITY` events containing:

- authoritative position and velocity-derived speed;
- a coarse gait band;
- stance;
- surface type from `SurfaceGetType3D`;
- footwear attachment type.

Go estimates a likely and maximum footstep range from those fields. It does not claim to reproduce DayZ’s complete sound engine, building acoustics, weather masking, animation-specific samples, or the player’s actual volume settings.

### Cue safety

A cue means:

> The captured game state supports a plausible opportunity for the observer to hear the target.

It does not mean:

> The observer definitely heard and correctly localized the target.

Strong or likely cues can classify an observation as known or plausible and remove it from the primary unexplained-feature set. Possible cues remain visible to reviewers but do not automatically suppress the primary analysis.

## Time model

DayZ server time is monotonic only within one `server_session_id`. Events from different server sessions are never joined using their relative timestamps.

Client transitions are sampled, so the true transition occurred within an interval rather than at an exact point:

```text
event_time_lower_ms <= true event time <= event_time_upper_ms
```

The server issues single-use clock challenges and maps client intervals into server time using the latest accepted offset and uncertainty. When alignment is missing or uncertainty is too large, timing-sensitive evidence is suppressed.

## Sampling model

### Random prospective opportunities

Random opportunities are the primary statistical stream. Each eligible observer receives randomized triggers. The collector records:

- observer and target risk-set sizes;
- inclusion probabilities;
- queue-admission probability;
- queue delay and scheduler state;
- sampling policy version and reason.

The possible opportunity types are:

- validated concealed target — the hidden condition;
- no relevant target inside the configured risk set — the neutral control;
- exposed or partially exposed target — a positive control used to verify ordinary responsiveness.

Visible players are not used as the neutral baseline for hidden-awareness lift.

### Event enrichment

A compact client decision edge can request bounded, lower-priority diagnostic probing around one observer/target pair. This stream helps reconstruct exposure timing and concealed-sector behaviour, but it is kept separate from random prospective opportunities to avoid selection bias in the primary estimate.

### Independence

Repeated samples close together are grouped into windows, target episodes, and encounters. Evidence breadth is counted across distinct sessions, encounters, and durable target identities, not raw event volume.

## Visibility safety

A head-origin ray is not automatically equivalent to what a player could see, especially in third person.

Strong concealed-target evidence therefore requires all of the following:

- Tier A server-side geometry;
- a server that actually enforces first person;
- `server_first_person_only=true`;
- `visibility_origin_mode=VALIDATED_FIRST_PERSON_HEAD`;
- a non-empty validation ID from a controlled fixture;
- repeated occlusion for at least the configured duration;
- acceptable queue delay and event timing.

Without those conditions, visibility remains descriptive or supporting context and cannot become strong hidden evidence.

## Storage and replay

### Raw tier

Each batch is stored as an immutable JSON file under:

```text
<raw-root>/<server-id>/<server-session-id>/<batch-sequence>.json
```

Writes use a temporary file, fsync, atomic rename, and a directory sync where supported. Reusing a batch identity with different contents is treated as a conflict.

### PostgreSQL tier

PostgreSQL stores normalized events, direct identities, sessions, sampling opportunities, visibility evidence, cue facts, observations, feature results, review rankings, cases, and review dispositions.

Database data is derived and can be rebuilt from retained raw batches, except for operational information such as reviewer dispositions that exists only in PostgreSQL.

### Versioning

- breaking transport changes increment `schema_version`;
- sampling, visibility, audio, observation, feature, matching, and ranking policies carry explicit version identifiers;
- applied SQL migrations are checksum-verified;
- historical raw data can be replayed with a newer analysis implementation without rewriting the original evidence.

## Failure behaviour

The system is intended to fail conservatively:

- network failure causes DayZ export batches to be spooled to disk;
- queue and spool limits are bounded and reported through health events;
- stale or dead observer/target entities are revalidated before a visibility probe;
- malformed, unauthorized, conflicting, or impossible batches are rejected;
- missing position context prevents an audio cue from being asserted;
- possible audio is retained without automatically explaining an observation;
- unavailable map data is not substituted with a different terrain;
- missing telemetry or failed negative controls suppresses higher review tiers;
- no failure mode creates automatic gameplay enforcement.

## Repository layout

```text
cmd/                    runnable Go services and tools
dayz/BehaviourProbe/    DayZ client/server mod source
deploy/                 Docker Compose deployment
internal/audio/         audibility policy model
internal/cues/          cue-ledger enrichment
internal/               ingestion, replay, normalization, features, review UI
pkg/schema/             transport schema and validation
docs/                   newcomer and operator guides
```
