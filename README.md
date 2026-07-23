# DayZ Behaviour

Behavioural telemetry and conservative review-prioritisation tooling for repeated hidden-awareness patterns in DayZ. It supports manual review only; it does not decide cheating or trigger bans, kicks or gameplay action.

## Status

The collector, durable data plane, replay/normalisation, core analysis and persistent review API are implemented. The mod packs and loads on DayZ Server 1.29.0.163451, and authenticated live export plus outage spool recovery have been exercised.

Production readiness is intentionally blocked on connected-player capability fixtures, controlled visibility validation, representative load, trusted-cohort calibration, negative controls and blinded-review yield. See [implementation status](docs/implementation-status.md) and the [capability matrix](docs/capability-matrix.md).

## Test and run locally

Go may need to be addressed by its full path on Windows if it is not yet in the current shell's `PATH`.

```powershell
go test ./...
go vet ./...

$env:DBA_QUERY_TOKEN = 'replace-with-a-long-random-token'
go run ./cmd/ingestd
```

The receiver binds to `127.0.0.1:8080` and stores fsynced raw batches under `./data/raw` by default. Authentication is mandatory unless `DBA_ALLOW_UNAUTHENTICATED_LOCAL=true` is explicitly set for development.

## Full local stack

```powershell
$env:DBA_POSTGRES_PASSWORD = 'replace-with-a-long-random-password'
$env:DBA_QUERY_TOKEN = 'replace-with-a-long-random-ingest-token'
$env:DBA_REVIEW_TOKEN = 'replace-with-a-long-random-review-token'
$env:DBA_STEAM_ADMIN_IDS = '76561198000000000' # comma-separated SteamID64 allowlist
$env:DBA_SESSION_SECRET = 'replace-with-at-least-32-random-characters'
$env:DBA_PUBLIC_BASE_URL = 'http://127.0.0.1:8082'
docker compose -f deploy/compose.yaml up --build postgres ingestd reviewd
```

Normalize and analyse immutable raw data:

```powershell
go run ./cmd/normalize -raw-dir ./data/raw
go run ./cmd/analyse -raw-dir ./data/raw
```

The admin explorer is served at `http://127.0.0.1:8082/`. Browser users sign in through Steam and are admitted only when their verified SteamID64 is in `DBA_STEAM_ADMIN_IDS`. The UI provides searchable pseudonymized sessions, a filterable context timeline and a self-contained route/location map. The review image includes only level-4 Chernarus, Livonia, Sakhal and Namalsk assets selected from DZMap; no second map container runs. Compose continuously normalizes new immutable batches, and its `telemetry-data` and `postgres-data` volumes survive ordinary container restarts and `docker compose down`. Do not use `docker compose down -v` unless the captured dataset is intentionally being erased. API clients can continue to use `Authorization: Bearer <DBA_REVIEW_TOKEN>`.

For a reverse-proxied deployment, set `DBA_PUBLIC_BASE_URL` to the exact externally visible HTTPS origin. Steam OpenID callback URLs and secure-cookie behaviour are derived from it. See [admin explorer operations](docs/admin-explorer.md).

Retention and identity deletion default to report-only operation:

```powershell
go run ./cmd/retention -raw-dir ./data/raw
go run ./cmd/privacy-delete -raw-dir ./data/raw -player-id '<durable-id>'
```

Add `-execute` only after reviewing the reported scope. Executed identity deletion also requires `-actor`, `-reason` and PostgreSQL configuration and writes a pseudonymous audit record.

## DayZ mod

Source is under `dayz/BehaviourProbe`. Pack and load it on both client and dedicated server. First launch creates `$profile:DayZBehaviourProbe/config.json`; configure the loopback endpoint and the same ingest token.

Visibility probes are disabled by default. Keep `visibility_origin_mode` as `PLAYER_HEAD_APPROXIMATION`, `server_first_person_only` as `false`, and the validation ID empty during development. Strong occlusion may only be enabled after a controlled first-person confusion matrix passes on a server that actually enforces first person; then set `server_first_person_only` to `true`, use `VALIDATED_FIRST_PERSON_HEAD`, and record the issued validation ID. Third-person or unknown-perspective head rays never become strong evidence.

## Design references

- [Implementation plan](docs/behavioural-awareness-implementation-plan.md)
- [Specification](docs/behavioural-awareness-spec.md)
- [Implementation inventory](docs/implementation-inventory.md)
- [Sampling and latency report](docs/sampling-and-latency-report.md)
- [DayZ script surface register](docs/dayz-script-surface-register.md)
