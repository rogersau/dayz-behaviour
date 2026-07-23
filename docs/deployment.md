# Deployment

This guide describes the supported local/sidecar topology. The DayZ collector exports to a receiver beside the dedicated server, while PostgreSQL and the review service may run on the same host or on a protected internal network.

## Prerequisites

- a DayZ dedicated server;
- the DayZ mod tools or existing build/signing process used by your server;
- Docker Engine with Compose for the standard stack;
- Go matching the version in `go.mod` for direct development;
- sufficient protected disk for immutable raw telemetry and PostgreSQL;
- an HTTPS reverse proxy if the admin explorer will be accessed over a network.

## Recommended topology

```text
DayZ dedicated server
    │
    ├─ DayZBehaviourProbe server scripts
    ├─ loopback HTTP ───────────────> ingestd :8080
    └─ failed-export spool               │
                                         ▼
                                  immutable raw volume
                                         │
                         normalize ───────┴─────── analyse
                              │                       │
                              └────── PostgreSQL ─────┘
                                          │
                                       reviewd :8082
                                          │
                                   HTTPS reverse proxy
                                          │
                                      administrators
```

`ingestd` and PostgreSQL are published on `127.0.0.1` by the supplied Compose file. Keep them private. Only the reverse-proxied `reviewd` service should be reachable by administrators over a network.

## Configure environment variables

Copy the example file:

```powershell
Copy-Item .env.example .env
```

Replace every placeholder before starting the stack.

| Variable | Required by | Purpose |
|---|---|---|
| `DBA_POSTGRES_PASSWORD` | Compose/PostgreSQL | Database password |
| `DBA_QUERY_TOKEN` | DayZ mod and `ingestd` | Loopback DayZ ingest credential |
| `DBA_REVIEW_TOKEN` | `reviewd` | Bearer token for scripts and API clients |
| `DBA_PUBLIC_BASE_URL` | `reviewd` | Exact externally visible origin |
| `DBA_STEAM_ADMIN_IDS` | `reviewd` | Comma-separated SteamID64 administrator allowlist |
| `DBA_SESSION_SECRET` | `reviewd` | Browser-session signing secret, at least 32 characters |
| `DBA_DEFAULT_MAP` | `reviewd` | Fallback terrain only when telemetry has no `map_id` |

Generate independent random values for the query token, review token, session secret, and database password. Do not reuse a secret between purposes.

The application now preserves direct DayZ/Steam identifiers. No pseudonym secret or key ID is required.

## Start the standard stack

```powershell
docker compose --env-file .env -f deploy/compose.yaml up --build postgres ingestd normalize reviewd
```

The Compose services are:

| Service | Default loopback address | Role |
|---|---|---|
| `postgres` | `127.0.0.1:5432` | Direct-identity normalized, analysis, and review data |
| `ingestd` | `127.0.0.1:8080` | DayZ telemetry receiver and spool importer |
| `normalize` | internal | Continuous raw-to-PostgreSQL normalization every 5 seconds |
| `reviewd` | `127.0.0.1:8082` | Browser explorer and review API |

Named volumes retain data across ordinary restarts and `docker compose down`:

- `telemetry-data` — immutable raw batches and DayZ spool import area;
- `postgres-data` — PostgreSQL cluster data.

Do not run `docker compose down -v` unless you intentionally want to erase both datasets.

## Existing databases created by an anonymizing release

One-way pseudonyms cannot be reversed. If PostgreSQL already contains pseudonymized players, the direct-identity policy fails closed until the derived data is rebuilt.

Back up raw and PostgreSQL data, then run a dry report:

```powershell
go run ./cmd/direct-identity-rebuild `
  -raw-dir ./data/raw `
  -database-url $env:DBA_DATABASE_URL
```

Execute only after reviewing the counts:

```powershell
go run ./cmd/direct-identity-rebuild `
  -raw-dir ./data/raw `
  -database-url $env:DBA_DATABASE_URL `
  -execute `
  -confirm REBUILD_WITH_DIRECT_IDENTITIES

go run ./cmd/analyse -raw-dir ./data/raw
```

The command preserves restricted raw batches and the privacy audit log. It clears normalized events, sessions, cues, observations, features, rankings, cases, and dispositions before replaying raw telemetry. Reviewer dispositions are not reconstructable from raw events, so export or record any that must be retained before executing.

## Verify the services

Check service state:

```powershell
docker compose --env-file .env -f deploy/compose.yaml ps
```

Open the local explorer:

```text
http://127.0.0.1:8082/
```

A Steam login succeeds only when the verified SteamID64 is present in `DBA_STEAM_ADMIN_IDS`.

At this point an empty explorer is expected because the DayZ collector has not exported data yet.

## Pack and load the DayZ mod

The source is under:

```text
dayz/BehaviourProbe
```

Use the same packing and signing process as your other DayZ mods. Load the packed mod on:

- the dedicated server;
- every client connecting to the instrumented server.

The client side captures camera/control context. The server side owns identity, lifecycle, combat, authoritative position, gunshot and movement-audio opportunities, visibility geometry, batching, export, and spool recovery. Running only one side produces incomplete telemetry and should not be treated as an operational deployment.

The project owns RPC IDs `759430` through `759436`. Check that they do not conflict with another loaded mod before deployment.

## Configure the DayZ collector

Start the server once. The mod creates:

```text
$profile:DayZBehaviourProbe/config.json
```

Minimum configuration:

```json
{
  "enabled": true,
  "server_id": "example-chernarus-1",
  "endpoint": "http://127.0.0.1:8080/",
  "ingest_token": "same-value-as-DBA_QUERY_TOKEN",
  "configuration_hash": "example-chernarus-1-v1"
}
```

The endpoint must end with `/`.

Important collection controls include:

| Setting | Default | Meaning |
|---|---:|---|
| `server_snapshot_interval_seconds` | `0.5` | Authoritative server snapshot interval |
| `export_interval_seconds` | `1.0` | Normal export cadence |
| `max_events_per_export` | `128` | Maximum events in one HTTP batch |
| `max_pending_events` | `512` | Bounded in-memory event queue |
| `enable_audio_cues` | `true` | Captures server gunshot and movement-audio opportunities |
| `audio_context_interval_seconds` | `2.0` | Server cadence for movement-audio context |
| `audio_min_movement_speed_mps` | `0.25` | Ignores nearly stationary movement for footstep inference |
| `enable_visibility_probe` | `false` | Enables observer/target geometry work |
| `max_visibility_pairs_per_tick` | `1` | Per-tick visibility work cap |
| `visibility_radius_metres` | `1000` | Target candidate radius |
| `random_opportunity_min_seconds` | `3` | Lower random-trigger interval |
| `random_opportunity_max_seconds` | `8` | Upper random-trigger interval |
| `max_probe_queue_age_ms` | `1000` | Drops stale visibility work |
| `max_spool_batches_per_file` | `256` | Failed-export spool rotation size |
| `max_spool_files` | `8` | Bounded spool file count |
| `clock_challenge_interval_seconds` | `30` | Client/server clock alignment cadence |
| `minimum_occlusion_duration_ms` | `250` | Repeated blocked-probe requirement |

Keep the default queue and work limits until representative-load testing shows the deployment can safely change them.

## Audio-cue validation

Audio collection does not record raw sound. The server emits:

- `SHOT_FIRED_SERVER` from `Weapon_Base.EEFired`, including weapon, ammunition, suppressor, shooter, time, and position;
- `MOVEMENT_AUDIO_OPPORTUNITY` from bounded server sampling, including speed, gait, stance, surface, footwear, time, and position.

Before relying on these cues operationally:

1. Confirm `EEFired` executes once per successful shot on the target dedicated-server build.
2. Test suppressed and unsuppressed weapons, burst/automatic fire, multi-muzzle weapons, and reconnects.
3. Confirm movement events appear for walk, jog, sprint, crouch, prone, barefoot, and representative footwear.
4. Confirm `SurfaceGetType3D` returns useful surface names on terrain, roads, building floors, metal, wood, and modded surfaces.
5. Compare model classifications with controlled listening tests at labelled distances.
6. Record the server build, mod set, test scene, model version, and accepted limitations.
7. Adjust Go model thresholds through a new version rather than silently changing existing semantics.

The model intentionally remains approximate. It does not reproduce exact DayZ attenuation, occlusion, indoor acoustics, weather masking, hearing damage, or client volume settings.

## Visibility configuration

Visibility probing is disabled by default because strong hidden-target evidence requires deployment-specific validation.

### Development or third-person servers

Use:

```json
{
  "enable_visibility_probe": false,
  "server_first_person_only": false,
  "visibility_origin_mode": "PLAYER_HEAD_APPROXIMATION",
  "visibility_validation_id": ""
}
```

You may enable descriptive probes in controlled tests, but a head approximation on a third-person or unknown-perspective server must not be treated as strong hidden evidence.

### Validated first-person deployment

Only after a controlled confusion-matrix fixture passes on the exact server build and mod set:

```json
{
  "enable_visibility_probe": true,
  "server_first_person_only": true,
  "visibility_origin_mode": "VALIDATED_FIRST_PERSON_HEAD",
  "visibility_validation_id": "your-recorded-validation-id"
}
```

Retain the fixture results, server build, mod list, policy configuration, and validation ID as an operational record. Revalidate after changes that could affect geometry, camera behaviour, animation/bones, or script execution.

## First collection check

After one or more clients connect:

1. Confirm `ingestd` logs accepted batches rather than authentication or validation failures.
2. Confirm raw JSON files appear in the `telemetry-data` volume under server and server-session directories.
3. Confirm `normalize` reports newly processed batches.
4. Confirm direct SteamID64/player-session values appear in the explorer.
5. Open a session and confirm lifecycle, server snapshot, client context, collector-health, movement-audio, and gunshot events appear.
6. Fire suppressed and unsuppressed weapons and verify shooter identity, weapon, suppressor state, and position.
7. Walk, sprint, crouch, and prone over representative surfaces and verify movement context.
8. Disconnect and reconnect a client and confirm the new session accepts client sequences starting again.
9. Stop `ingestd` briefly in a controlled environment, confirm DayZ spools failed exports, restart it, and confirm the spool importer archives the recovered file.

Do not enable operational review rankings until this basic collection path is stable.

## Run analysis

The Compose `analyse` service is under the `tools` profile and runs once:

```powershell
docker compose --env-file .env -f deploy/compose.yaml --profile tools run --rm analyse
```

Direct Go execution:

```powershell
go run ./cmd/analyse -raw-dir ./data/raw
```

When `DBA_DATABASE_URL` is present, analysis results are written to PostgreSQL. Otherwise JSON is printed to standard output.

The analyzer needs both eligible hidden opportunities and neutral controls. A successfully running collector may still produce no review candidate when visibility validation is absent, data is sparse, timing is uncertain, cues explain the observations, or evidence gates do not pass.

## Reverse proxy the admin explorer

For network access:

1. keep `reviewd` bound to loopback or a private container network;
2. terminate TLS at a trusted reverse proxy;
3. forward the external host and scheme correctly;
4. set `DBA_PUBLIC_BASE_URL` to the exact HTTPS origin, with no unexpected path rewriting;
5. restrict access further with network controls where practical.

Example:

```text
DBA_PUBLIC_BASE_URL=https://behaviour.example.com
```

Steam OpenID callback validation and secure-cookie behaviour are derived from this value. A mismatch commonly causes login loops or rejected callbacks.

Do not publish:

- PostgreSQL port `5432`;
- `ingestd` port `8080` using the DayZ query-token transport;
- raw telemetry files or DayZ profile configuration;
- direct-identity review API responses or exports;
- map tiles outside the authenticated `reviewd` routes.

## Direct development without Compose

Start the receiver:

```powershell
$env:DBA_QUERY_TOKEN = 'replace-with-a-long-random-token'
go run ./cmd/ingestd
```

Normalize into an existing PostgreSQL database:

```powershell
$env:DBA_DATABASE_URL = 'postgres://user:password@127.0.0.1:5432/dayz_behaviour?sslmode=disable'
go run ./cmd/normalize -raw-dir ./data/raw
```

Run the explorer with the same database. Use `.env.example` as the authoritative environment-variable checklist.

## Deployment acceptance checklist

Before calling a deployment operational, record evidence for:

- client and server script compilation on the target DayZ build;
- lifecycle, reconnect, combat, gunshot, movement-audio, client transition, and health callbacks;
- authentication, immutable raw storage, normalization, and replay;
- direct-identity display and cross-reference;
- spool recovery and bounded-loss reporting;
- controlled audio-model checks for weapons, suppressors, movement, surfaces, and footwear;
- first-person visibility confusion matrix where strong evidence is enabled;
- representative player-count CPU, frame, network, queue, and drop measurements;
- trusted-cohort calibration and negative controls;
- administrator authentication and authorization;
- backup restoration, retention dry runs, and player deletion;
- documented review workflow and uncertainty language.

Passing unit tests or an empty-server smoke test is not a substitute for these deployment-specific checks.
