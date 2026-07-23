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
| `DBA_PSEUDONYM_SECRET` | normalization, analysis, review | Stable HMAC secret, at least 32 characters |
| `DBA_PSEUDONYM_KEY_ID` | normalization, analysis, review | Stable operator-managed key identifier |
| `DBA_DEFAULT_MAP` | `reviewd` | Fallback terrain only when telemetry has no `map_id` |

The pseudonym secret and key ID form a database identity policy. Set them before the database first contains player data. Changing either later requires an explicit identity migration or a deliberate reset of development data.

Generate independent, random values for the query token, review token, session secret, pseudonym secret, and database password. Do not reuse a secret between purposes.

## Start the standard stack

```powershell
docker compose --env-file .env -f deploy/compose.yaml up --build postgres ingestd normalize reviewd
```

The Compose services are:

| Service | Default loopback address | Role |
|---|---|---|
| `postgres` | `127.0.0.1:5432` | Normalized, analysis, and review data |
| `ingestd` | `127.0.0.1:8080` | DayZ telemetry receiver and spool importer |
| `normalize` | internal | Continuous raw-to-PostgreSQL normalization every 5 seconds |
| `reviewd` | `127.0.0.1:8082` | Browser explorer and review API |

Named volumes retain data across ordinary restarts and `docker compose down`:

- `telemetry-data` — immutable raw batches and DayZ spool import area;
- `postgres-data` — PostgreSQL cluster data.

Do not run `docker compose down -v` unless you intentionally want to erase both datasets.

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

The client side captures camera/control context. The server side owns identity, lifecycle, combat, visibility geometry, batching, export, and spool recovery. Running only one side produces incomplete telemetry and should not be treated as an operational deployment.

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
4. Confirm pseudonymized sessions appear in the explorer.
5. Open a session and confirm lifecycle, server snapshot, client context, and collector-health events appear.
6. Disconnect and reconnect a client and confirm the new session accepts client sequences starting again.
7. Stop `ingestd` briefly in a controlled environment, confirm DayZ spools failed exports, restart it, and confirm the spool importer archives the recovered file.

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

The analyzer needs both eligible hidden opportunities and neutral controls. A successfully running collector may still produce no review candidate when visibility validation is absent, data is sparse, timing is uncertain, or evidence gates do not pass.

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
$env:DBA_PSEUDONYM_SECRET = 'replace-with-at-least-32-random-characters'
$env:DBA_PSEUDONYM_KEY_ID = 'development-v1'
go run ./cmd/normalize -raw-dir ./data/raw
```

Run the explorer with the same database and pseudonym policy. Use `.env.example` as the authoritative environment-variable checklist.

## Deployment acceptance checklist

Before calling a deployment operational, record evidence for:

- client and server script compilation on the target DayZ build;
- lifecycle, reconnect, combat, client transition, and health callbacks;
- authentication, immutable raw storage, normalization, and replay;
- spool recovery and bounded-loss reporting;
- first-person visibility confusion matrix where strong evidence is enabled;
- representative player-count CPU, frame, network, queue, and drop measurements;
- trusted-cohort calibration and negative controls;
- administrator authentication and authorization;
- backup restoration, retention dry runs, and player deletion;
- documented review workflow and uncertainty language.

Passing unit tests or an empty-server smoke test is not a substitute for these deployment-specific checks.
