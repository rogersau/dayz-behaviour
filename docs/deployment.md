# Deployment

This guide describes the supported local/sidecar topology. The DayZ collector exports to a receiver beside the dedicated server, while PostgreSQL and the review service may run on the same host or a protected internal network.

## Prerequisites

- a DayZ dedicated server;
- the normal DayZ mod packing and signing toolchain;
- Docker Engine with Compose for the standard stack;
- Go matching `go.mod` for direct development;
- protected disk for raw telemetry and PostgreSQL;
- an HTTPS reverse proxy for remote explorer access.

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

The supplied Compose file publishes `ingestd` and PostgreSQL only on `127.0.0.1`. Keep them private. Only reverse-proxied `reviewd` should be reachable remotely.

## Environment variables

Copy the example:

```powershell
Copy-Item .env.example .env
```

Replace every placeholder.

| Variable | Used by | Purpose |
|---|---|---|
| `DBA_POSTGRES_PASSWORD` | PostgreSQL/Compose | Database password |
| `DBA_QUERY_TOKEN` | DayZ mod and `ingestd` | Loopback ingest credential |
| `DBA_REVIEW_TOKEN` | `reviewd` | Bearer token for API clients |
| `DBA_PUBLIC_BASE_URL` | `reviewd` | Exact externally visible origin |
| `DBA_STEAM_ADMIN_IDS` | `reviewd` | Comma-separated SteamID64 allowlist |
| `DBA_SESSION_SECRET` | `reviewd` | Browser-session signing secret |
| `DBA_DEFAULT_MAP` | `reviewd` | Fallback terrain when telemetry lacks `map_id` |

Generate independent random values for credentials. The application stores direct DayZ/Steam IDs and does not require an identity secret or compatibility mode.

## Start the standard stack

```powershell
docker compose --env-file .env -f deploy/compose.yaml up --build postgres ingestd normalize reviewd
```

| Service | Default address | Role |
|---|---|---|
| `postgres` | `127.0.0.1:5432` | Normalized, analysis, and review data |
| `ingestd` | `127.0.0.1:8080` | Telemetry receiver and spool importer |
| `normalize` | internal | Continuous raw-to-PostgreSQL normalization |
| `reviewd` | `127.0.0.1:8082` | Explorer and review API |

Named volumes persist through ordinary restarts:

- `telemetry-data` — raw batches and spool import area;
- `postgres-data` — PostgreSQL cluster.

Do not run `docker compose down -v` unless both datasets should be erased.

## Verify the services

```powershell
docker compose --env-file .env -f deploy/compose.yaml ps
```

Open:

```text
http://127.0.0.1:8082/
```

Steam login succeeds only when the verified SteamID64 is in `DBA_STEAM_ADMIN_IDS`. An empty explorer is expected before the collector exports data.

## Pack and load the DayZ mod

Source:

```text
dayz/BehaviourProbe
```

Use the normal packing and signing process. Load the mod on:

- the dedicated server;
- every connecting client.

The client captures camera/control context. The server owns identity, lifecycle, combat, authoritative position, gunshot and movement-audio opportunities, visibility geometry, batching, export, and spool recovery.

The project uses RPC IDs `759430` through `759436`. Confirm they do not conflict with another loaded mod.

## Configure the collector

First launch creates:

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

Important settings:

| Setting | Default | Meaning |
|---|---:|---|
| `server_snapshot_interval_seconds` | `0.5` | Authoritative snapshot interval |
| `export_interval_seconds` | `1.0` | Normal export cadence |
| `max_events_per_export` | `128` | Maximum events per HTTP batch |
| `max_pending_events` | `512` | Bounded in-memory event queue |
| `enable_audio_cues` | `true` | Captures gunshot and movement-audio opportunities |
| `audio_context_interval_seconds` | `2.0` | Movement-audio sampling cadence |
| `audio_min_movement_speed_mps` | `0.25` | Ignores nearly stationary movement |
| `enable_visibility_probe` | `false` | Enables observer/target geometry work |
| `max_visibility_pairs_per_tick` | `1` | Per-tick visibility work cap |
| `visibility_radius_metres` | `1000` | Candidate radius |
| `random_opportunity_min_seconds` | `3` | Minimum random interval |
| `random_opportunity_max_seconds` | `8` | Maximum random interval |
| `max_probe_queue_age_ms` | `1000` | Drops stale probe work |
| `max_spool_batches_per_file` | `256` | Spool rotation size |
| `max_spool_files` | `8` | Spool file cap |
| `clock_challenge_interval_seconds` | `30` | Clock alignment cadence |
| `minimum_occlusion_duration_ms` | `250` | Repeated occlusion requirement |

Keep default budgets until representative-load tests justify changes.

## Audio-cue validation

No raw sound is recorded. The server emits:

- `SHOT_FIRED_SERVER` from `Weapon_Base.EEFired`, including shooter, position, weapon, ammunition, muzzle type, fire mode, and suppressor context;
- `MOVEMENT_AUDIO_OPPORTUNITY`, including position, speed, gait, stance, surface, and footwear.

Before relying on audio cues operationally:

1. confirm `EEFired` runs once per successful shot on the target dedicated-server build;
2. test suppressed and unsuppressed weapons, burst/automatic fire, multi-muzzle weapons, and modded weapons;
3. verify shooter attribution across reconnects;
4. verify movement events for walk, jog, sprint, crouch, prone, barefoot, and representative footwear;
5. inspect `SurfaceGetType3D` values on terrain, roads, floors, wood, metal, and modded surfaces;
6. compare model classifications with controlled listening tests at labelled distances;
7. measure event volume and script/export cost at representative player count;
8. change thresholds only through a new model version.

The model does not reproduce exact DayZ attenuation, indoor acoustics, weather masking, hearing damage, or client volume settings.

## Visibility configuration

Visibility probing is disabled by default because strong hidden evidence requires deployment-specific validation.

For development or third-person servers:

```json
{
  "enable_visibility_probe": false,
  "server_first_person_only": false,
  "visibility_origin_mode": "PLAYER_HEAD_APPROXIMATION",
  "visibility_validation_id": ""
}
```

For a validated first-person deployment, only after a controlled confusion-matrix fixture passes:

```json
{
  "enable_visibility_probe": true,
  "server_first_person_only": true,
  "visibility_origin_mode": "VALIDATED_FIRST_PERSON_HEAD",
  "visibility_validation_id": "your-recorded-validation-id"
}
```

Retain the fixture, DayZ build, mod list, settings, and validation ID. Revalidate after changes that may affect camera, geometry, animation, bones, or script execution.

## First collection check

After clients connect:

1. confirm `ingestd` accepts batches;
2. confirm raw JSON appears under the expected server/session path;
3. confirm `normalize` processes new batches;
4. confirm direct player and session IDs appear in the explorer;
5. inspect lifecycle, snapshot, client-context, and health events;
6. fire suppressed and unsuppressed weapons and inspect shot context;
7. move over representative surfaces and inspect movement-audio events;
8. disconnect/reconnect and verify a new session accepts restarted client sequences;
9. test spool recovery by briefly interrupting `ingestd` in a controlled environment.

Do not treat review rankings as operational until this path is stable.

## Run analysis

Compose tool profile:

```powershell
docker compose --env-file .env -f deploy/compose.yaml --profile tools run --rm analyse
```

Direct execution:

```powershell
go run ./cmd/analyse -raw-dir ./data/raw
```

With `DBA_DATABASE_URL`, results persist to PostgreSQL. Without it, JSON is printed to standard output.

No candidate may be the correct result when visibility validation is absent, data is sparse, timing is uncertain, cues explain observations, or evidence gates do not pass.

## Reverse proxy the explorer

For remote access:

1. keep `reviewd` on loopback or a private network;
2. terminate TLS at a trusted proxy;
3. preserve forwarded host and scheme;
4. set `DBA_PUBLIC_BASE_URL` to the exact HTTPS origin;
5. apply additional network restrictions where practical.

Do not publish PostgreSQL, `ingestd`, raw telemetry, direct-identity API responses, or unauthenticated map assets.

## Direct development

Receiver:

```powershell
$env:DBA_QUERY_TOKEN = 'replace-with-a-long-random-token'
go run ./cmd/ingestd
```

Normalizer:

```powershell
$env:DBA_DATABASE_URL = 'postgres://user:password@127.0.0.1:5432/dayz_behaviour?sslmode=disable'
go run ./cmd/normalize -raw-dir ./data/raw
```

Use `.env.example` as the environment-variable checklist.

## Deployment acceptance checklist

Before calling a deployment operational, retain evidence for:

- client and server script compilation on the target DayZ build;
- lifecycle, reconnect, combat, gunshot, movement-audio, client transition, and health callbacks;
- authenticated ingest, immutable storage, normalization, and replay;
- direct-ID display and cross-reference;
- spool recovery and bounded-loss reporting;
- audio calibration and representative-load measurements;
- visibility confusion matrix where strong evidence is enabled;
- trusted-cohort calibration and negative controls;
- administrator authentication and authorization;
- backup restoration, retention, and deletion exercises;
- documented review workflow and uncertainty language.
