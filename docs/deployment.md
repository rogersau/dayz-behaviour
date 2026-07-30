# Deployment

This guide covers the central stack, DayZ collector, and initial validation. A DayZ server outside the central host uses the standalone Windows agent described in [DayZ server agent](server-agent.md); the agent durably forwards telemetry without requiring Docker or PostgreSQL on the game server.

Use [Releases and published images](releases.md) for release contents, checksums, exact image tags, upgrades, rollback, and release publication.

## Prerequisites

- a DayZ dedicated server;
- the normal DayZ mod packing and signing toolchain;
- Docker Engine with Compose for the central stack;
- Windows AMD64 for the standalone game-server agent;
- protected disk for raw telemetry and PostgreSQL;
- an HTTPS reverse proxy or Cloudflare Tunnel for remote access;
- Go matching `go.mod` only for direct development or source builds.

## Deployment methods

Use one of these central deployment methods:

- **Published release images — recommended for operators.** Pin the central and review images to the same exact version and use `deploy/compose.release.yaml`.
- **Source build — intended for development and evaluation.** Use the base Compose file with `--build`.

Use the standalone agent for every DayZ server outside the central host. Keep the DayZ mod pointed at the agent on `127.0.0.1`; do not point the mod directly at the public central hostname.

## Recommended topology

```text
DayZ dedicated server
    │
    ├─ DayZBehaviourProbe client/server mod
    │          │
    │          └─ loopback HTTP
    │                     ▼
    └──────── dayz-behaviour-agent.exe
               ├─ durable local outbox
               ├─ DayZ spool importer
               └─ authenticated HTTPS
                            │
                            ▼
                  Cloudflare public hostname
                            │
                            ▼
                 Cloudflare Tunnel at home
                            │
                            ▼
                         ingestd
                            │
                  immutable raw volume
                            │
             normalize ─────┴───── analyse
                  │                    │
                  └──── PostgreSQL ────┘
                            │
                         reviewd
                            │
                    authenticated admins
```

Cloudflare Tunnel is only the route into the home network. The Go services, PostgreSQL, raw telemetry, analysis, and review interface remain on the central host.

The supplied Compose files publish `ingestd`, PostgreSQL, and `reviewd` only on `127.0.0.1`. Keep them private. The home-hosted override adds `cloudflared`, which reaches the services through the private Compose network without opening an inbound home-router port.

A same-host evaluation deployment may omit the agent and send the DayZ collector directly to loopback `ingestd`. This is not the recommended distributed topology.

## Environment variables

Copy the example:

```powershell
Copy-Item .env.example .env
```

Replace every placeholder.

| Variable | Used by | Purpose |
|---|---|---|
| `DBA_POSTGRES_PASSWORD` | PostgreSQL/Compose | Database password |
| `DBA_QUERY_TOKEN` | same-host DayZ mod and `ingestd` | Loopback ingest credential; not for internet transport |
| `DBA_SERVER_AUTH_FILE` | central `ingestd` | Container path to the JSON map binding one remote agent credential to each `server_id` |
| `DBA_SERVER_AUTH_HOST_FILE` | home Compose override | Host path to the protected server-authentication JSON file |
| `DBA_CLOUDFLARE_TUNNEL_TOKEN` | `cloudflared` | Token for the outbound Cloudflare Tunnel connection |
| `DBA_CORE_IMAGE` | release Compose override | Exact central runtime image tag |
| `DBA_REVIEW_IMAGE` | release Compose override | Exact review image tag with map assets |
| `DBA_REVIEW_TOKEN` | `reviewd` | Bearer token for API clients |
| `DBA_PUBLIC_BASE_URL` | `reviewd` | Exact externally visible HTTPS origin |
| `DBA_STEAM_ADMIN_IDS` | `reviewd` | Comma-separated SteamID64 allowlist |
| `DBA_SESSION_SECRET` | `reviewd` | Browser-session signing secret |
| `DBA_DEFAULT_MAP` | `reviewd` | Fallback terrain when telemetry lacks `map_id` |

Generate independent random values for credentials. The application stores direct DayZ/Steam IDs and does not require an identity secret or compatibility mode.

## Start central services from published images

Pin both images to the same exact release:

```powershell
$env:DBA_CORE_IMAGE = "ghcr.io/rogersau/dayz-behaviour:0.1.0"
$env:DBA_REVIEW_IMAGE = "ghcr.io/rogersau/dayz-behaviour-review:0.1.0"
```

Pull the release:

```powershell
docker compose `
  --env-file .env `
  -f deploy/compose.yaml `
  -f deploy/compose.release.yaml `
  pull
```

Start the central services without falling back to local builds:

```powershell
docker compose `
  --env-file .env `
  -f deploy/compose.yaml `
  -f deploy/compose.release.yaml `
  up -d --no-build postgres ingestd normalize reviewd
```

For the home-hosted Cloudflare Tunnel topology:

```powershell
docker compose `
  --env-file .env `
  -f deploy/compose.yaml `
  -f deploy/compose.release.yaml `
  -f deploy/compose.home.yaml `
  up -d --no-build postgres ingestd normalize reviewd cloudflared
```

Use exact versions rather than `latest` for operational deployments. Keep the previous image tags available until the upgrade has completed validation.

## Build central services from source

From a checked-out source tree:

```powershell
docker compose --env-file .env -f deploy/compose.yaml up --build postgres ingestd normalize reviewd
```

| Service | Default address | Role |
|---|---|---|
| `postgres` | `127.0.0.1:5432` | Normalized, analysis, and review data |
| `ingestd` | `127.0.0.1:8080` | Telemetry receiver and central raw storage boundary |
| `normalize` | internal | Continuous raw-to-PostgreSQL normalization |
| `reviewd` | `127.0.0.1:8082` | Explorer and review API |
| `cloudflared` | outbound only | Tunnel route from Cloudflare to private Compose services |

Named volumes persist through ordinary restarts:

- `telemetry-data` — raw batches and central spool import area;
- `postgres-data` — PostgreSQL cluster.

Do not run `docker compose down -v` unless both datasets should be erased.

## Verify the services

For a release deployment:

```powershell
docker compose `
  --env-file .env `
  -f deploy/compose.yaml `
  -f deploy/compose.release.yaml `
  ps
```

Open the local review service:

```text
http://127.0.0.1:8082/
```

Steam login succeeds only when the verified SteamID64 is in `DBA_STEAM_ADMIN_IDS`. An empty explorer is expected before the collector exports data.

## Install the server agent

Download the Windows ZIP and checksum file from the matching GitHub Release. Verify the checksum before extracting it, then confirm the embedded version:

```powershell
.\dayz-behaviour-agent.exe version
```

Use the same release version as the central images for the initial protocol. Configure and install the Windows service as described in [DayZ server agent](server-agent.md).

The agent must:

- listen only on loopback;
- use a local DayZ query credential distinct from its upstream credential;
- use an upstream credential bound to its configured `server_id`;
- queue accepted batches durably before acknowledging DayZ;
- point to the public HTTPS ingest hostname;
- have protected local storage for its outbox, dead-letter files, and configuration.

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

Minimum configuration when using the agent:

```json
{
  "enabled": true,
  "server_id": "example-chernarus-1",
  "endpoint": "http://127.0.0.1:8080/",
  "ingest_token": [REDACTED_SECRET],
  "configuration_hash": "example-chernarus-1-v1"
}
```

The endpoint must end with `/`. The `server_id` must match the agent configuration and the central credential map.

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

1. confirm the agent reports the expected version and `status=ok`;
2. confirm the agent queue remains bounded and uploads succeed;
3. confirm central `ingestd` accepts the server credential and `server_id`;
4. confirm raw JSON appears under the expected server/session path;
5. confirm `normalize` processes new batches;
6. confirm direct player and session IDs appear in the explorer;
7. inspect lifecycle, snapshot, client-context, and health events;
8. fire suppressed and unsuppressed weapons and inspect shot context;
9. move over representative surfaces and inspect movement-audio events;
10. disconnect/reconnect and verify a new session accepts restarted client sequences;
11. test agent retry by briefly interrupting central ingest connectivity;
12. test DayZ spool recovery by briefly stopping the local agent in a controlled environment.

Do not treat review rankings as operational until this path is stable.

## Run analysis

Release-image Compose tool profile:

```powershell
docker compose `
  --env-file .env `
  -f deploy/compose.yaml `
  -f deploy/compose.release.yaml `
  --profile tools `
  run --rm --no-deps analyse
```

Source-build Compose tool profile:

```powershell
docker compose --env-file .env -f deploy/compose.yaml --profile tools run --rm analyse
```

Direct execution:

```powershell
go run ./cmd/analyse -raw-dir ./data/raw
```

With `DBA_DATABASE_URL`, results persist to PostgreSQL. Without it, JSON is printed to standard output.

No candidate may be the correct result when visibility validation is absent, data is sparse, timing is uncertain, cues explain observations, or evidence gates do not pass.

## Publish through Cloudflare Tunnel

Configure the Tunnel with the narrowest routes possible:

```text
ingest.example.com + path ^/v1/telemetry/batches$ → http://ingestd:8080
review.example.com                              → http://reviewd:8082
catch-all                                      → HTTP 404
```

Apply Cloudflare Access or an equivalent administrator control to the review hostname. The ingest route still requires the server-specific application credential.

Set `DBA_PUBLIC_BASE_URL` to the exact external review origin. Do not expose PostgreSQL, raw telemetry, direct-identity API output, `ingestd` health endpoints, or unauthenticated map assets.

## Direct development

Receiver:

```powershell
$env:DBA_QUERY_TOKEN = [REDACTED_SECRET]
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

- verified release checksums and recorded agent/image versions and digests;
- client and server script compilation on the target DayZ build;
- lifecycle, reconnect, combat, gunshot, movement-audio, client transition, and health callbacks;
- authenticated agent delivery, immutable central storage, normalization, and replay;
- agent outage retry, outbox bounds, dead-letter handling, and DayZ spool recovery;
- direct-ID display and cross-reference;
- audio calibration and representative-load measurements;
- visibility confusion matrix where strong evidence is enabled;
- trusted-cohort calibration and negative controls;
- administrator authentication and authorization;
- backup restoration, retention, deletion, upgrade, and rollback exercises;
- documented review workflow and uncertainty language.
