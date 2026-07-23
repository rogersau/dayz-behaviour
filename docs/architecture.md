# Architecture

## Purpose

DayZ Behaviour collects a deliberately limited set of game observations and uses them to identify repeated hidden-awareness patterns for manual administrator review.

The system is designed around four constraints:

1. DayZ collection must remain bounded and should not perform historical analysis in the game loop.
2. Server-authoritative facts and untrusted client context must remain distinguishable.
3. Uncertainty and missing information must reduce confidence rather than create suspicion.
4. Historical data and algorithm versions must remain replayable and auditable.

It is not designed to identify cheat software, prove the presence of DMA hardware, reproduce a player’s rendered screen or audio, or automatically enforce a punishment.

## System overview

```text
┌──────────────────────── DayZ process ────────────────────────┐
│                                                              │
│  Client collector                   Server collector          │
│  - camera direction                 - identity and sessions   │
│  - raised/ADS/optics state          - lifecycle and combat    │
│  - transition intervals             - authoritative position  │
│  - collector health                 - bounded visibility       │
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
                         │                                          │
                  PostgreSQL records                      observations/features
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

- player identity and player-session lifecycle;
- authoritative position, movement, alive, and unconscious state;
- hit, kill, and server-side fire events where the script seam is valid;
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

- converts known identity fields into deterministic pseudonyms;
- stores events, sessions, sampling opportunities, visibility runs, and analysis inputs;
- preserves source authority and algorithm/schema versions;
- applies database migrations and verifies migration checksums.

Normalization is idempotent. Raw files remain the source of truth for replay.

### `analyse`

`analyse` rebuilds observations from raw telemetry, calculates features, applies validation gates, and optionally persists results to PostgreSQL.

The primary feature family compares reactions during validated concealed-target opportunities with reactions during neutral no-relevant-target opportunities. Other feature families are supporting context and do not independently establish a review tier.

### `reviewd`

`reviewd` exposes:

- a Steam-authenticated browser explorer;
- bearer-authenticated review APIs;
- pseudonymized session search;
- ordered timelines with authority and payload context;
- local DayZ map tiles and spatial evidence;
- review cases and audited dispositions.

It is read-only with respect to DayZ. No endpoint can ban, kick, message, or alter a player in the game.

## Trust and authority

Events retain an authority tier throughout ingestion, normalization, analysis, and review.

| Tier | Meaning | Examples |
|---|---|---|
| A | Server-authoritative game fact | lifecycle, server position, combat callback, visibility geometry |
| B | Untrusted client observation | camera sample, ADS/optics transition, local timing interval |
| C | Collector or pipeline health | queue depth, drops, export failures, clock quality |

Tier B data can help explain when or how a player reacted. It cannot create strong hidden-target evidence without corresponding Tier A geometry and acceptable timing.

Tier C data never strengthens a player signal. Missing data, excessive timing uncertainty, queue delay, or collection loss can only suppress eligibility or lower confidence.

## Identity and sessions

The raw tier contains durable DayZ identity because the server needs it for attribution, joining, reconnect handling, and deletion.

The server assigns a distinct `player_session_id` for each connected character/session lifecycle. Client sequence and rate-limit state is reset when the player disconnects or reconnects, so a restarted collector can safely begin its sequences again.

Normalized and review data uses two pseudonym domains:

```text
dp_<HMAC-SHA256>  durable player identity
ps_<HMAC-SHA256>  player-session identity
```

The pseudonym policy and key ID are stored in PostgreSQL. A process using a different key fails closed because silently changing identity namespaces would break joins and deletion.

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

PostgreSQL stores normalized events, sessions, sampling opportunities, visibility evidence, observations, feature results, review rankings, cases, and review dispositions.

Database data is derived and can be rebuilt from retained raw batches, subject to retention and privacy deletion.

### Versioning

- breaking transport changes increment `schema_version`;
- sampling, visibility, observation, feature, matching, and ranking policies carry explicit version identifiers;
- applied SQL migrations are checksum-verified;
- historical raw data can be replayed with a newer analysis implementation without rewriting the original evidence.

## Failure behaviour

The system is intended to fail conservatively:

- network failure causes DayZ export batches to be spooled to disk;
- queue and spool limits are bounded and reported through health events;
- stale or dead observer/target entities are revalidated before a visibility probe;
- malformed, unauthorized, conflicting, or impossible batches are rejected;
- unavailable map data is not substituted with a different terrain;
- missing telemetry or failed negative controls suppresses higher review tiers;
- no failure mode creates automatic gameplay enforcement.

## Repository layout

```text
cmd/                    runnable Go services and tools
dayz/BehaviourProbe/    DayZ client/server mod source
deploy/                 Docker Compose deployment
internal/               ingestion, replay, normalization, features, review UI
pkg/schema/             transport schema and validation
docs/                   newcomer and operator guides
```
