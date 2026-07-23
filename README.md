# DayZ Behaviour

DayZ Behaviour is a telemetry and analysis system for finding repeated **hidden-awareness behaviour** on DayZ servers. It collects bounded client and server observations, compares a player’s reactions with neutral situations, and gives administrators explainable cases to review.

It is not an automatic anti-cheat verdict system. It does not ban, kick, punish, or send any gameplay action back to DayZ.

![How DayZ Behaviour works](docs/images/how-dayz-behaviour-works.svg)

## What problem it is trying to solve

Conventional anti-cheat tools are good at detecting known software, impossible inputs, or invalid game state. They are less useful when a player appears to know where concealed opponents are but leaves no direct technical signature.

This project looks for repeated behavioural patterns such as:

- raising or aiming a weapon while another player is validated as concealed;
- turning towards concealed-player sectors more often than matched controls;
- reacting before a target becomes visible, while preserving timing uncertainty;
- repeating these patterns across different sessions, encounters, and target players.

A single incident is not treated as proof. The system requires repeated, independent observations and preserves legitimate explanations such as prior visual contact, gunshots, inferred footsteps, team communications, map knowledge, prediction, and information the collector did not capture.

![What the system looks at](docs/images/what-the-system-looks-at.svg)

## How it works

![System architecture](docs/images/system-architecture.svg)

The DayZ server remains the authority for identity, lifecycle, combat, position, movement-audio opportunities, and visibility geometry. Client camera and control-state telemetry is retained as untrusted supporting context. Analysis runs outside the game server so collection stays bounded and historical data can be replayed with newer algorithms.

The system does **not** record raw audio. It derives audibility opportunities from authoritative gunshot events and movement context such as speed, stance, surface, footwear, position, distance, and suppressor state.

Read [Architecture](docs/architecture.md) for the complete data flow and trust model.

## Main components

| Component | Purpose |
|---|---|
| `dayz/BehaviourProbe` | Client/server DayZ mod that captures bounded telemetry, gunshot/movement audio opportunities, and visibility observations |
| `cmd/ingestd` | Authenticated loopback receiver and DayZ spool importer |
| `cmd/normalize` | Converts immutable raw batches into PostgreSQL records using direct DayZ/Steam identities |
| `cmd/analyse` | Builds observations, audio cue facts, matched controls, feature estimates, and review tiers |
| `cmd/reviewd` | Steam-authenticated evidence browser and review API |
| `cmd/retention` | Dry-run-first raw, normalized, and review retention |
| `cmd/privacy-delete` | Audited deletion of one durable player identity |

## Safety model

- Results are review priorities, not cheat probabilities.
- Strong hidden-target evidence requires server-authoritative geometry and a validated first-person visibility policy.
- Client data can suppress confidence but cannot strengthen an allegation by itself.
- Hidden observations are compared with neutral no-relevant-target opportunities, not merely with visible opponents.
- Captured gunshots and likely footsteps can explain an incident; possible audio cues remain visible without automatically suppressing analysis.
- Audio cues describe plausible audibility, not proof that a player heard a sound.
- Evidence breadth, uncertainty, matched-model stability, and negative-control gates are required before higher review tiers are produced.
- Pre-exposure timing is experimental supporting evidence and cannot promote a review tier.

![How a case gets flagged](docs/images/how-a-case-gets-flagged.svg)

### How the review tier is calculated

The system does not produce a single opaque cheat score. It builds a review tier from transparent evidence measurements and validation gates.

![How scoring works](docs/images/how-scoring-works.svg)

Read [Analysis and review](docs/analysis-and-review.md) for the statistical model, thresholds, and interpretation rules.

## Quick start

### Requirements

- Go matching `go.mod`;
- Docker with Compose;
- a DayZ dedicated server and a way to pack/sign the mod for live collection;
- PostgreSQL 17 when running services outside Compose.

### Run tests

```powershell
go test ./...
go vet ./...
```

### Start the local data and review stack

Copy `.env.example` to `.env`, replace every placeholder, then run:

```powershell
docker compose --env-file .env -f deploy/compose.yaml up --build postgres ingestd normalize reviewd
```

The services bind to loopback by default:

- ingest receiver: `http://127.0.0.1:8080`;
- admin explorer: `http://127.0.0.1:8082`;
- PostgreSQL: `127.0.0.1:5432`.

Do not expose the DayZ query-token ingest endpoint or PostgreSQL directly to a network.

### Configure the DayZ mod

Pack `dayz/BehaviourProbe`, load it on both the client and dedicated server, and start the server once. The mod creates:

```text
$profile:DayZBehaviourProbe/config.json
```

Set at least:

```json
{
  "server_id": "your-server-name",
  "endpoint": "http://127.0.0.1:8080/",
  "ingest_token": "the-same-value-as-DBA_QUERY_TOKEN"
}
```

Audio opportunity capture is enabled by default. Visibility probing is disabled by default. Do not enable strong hidden-target evidence until the server is first-person-only and its visibility policy has passed a controlled validation fixture.

See [Deployment](docs/deployment.md) for the full setup, environment variables, network topology, audio validation, and first-run checklist.

## Running analysis manually

`normalize` requires `DBA_DATABASE_URL`. `analyse` persists results when it is configured and otherwise prints JSON to standard output.

```powershell
$env:DBA_DATABASE_URL = 'postgres://user:password@127.0.0.1:5432/dayz_behaviour?sslmode=disable'
go run ./cmd/normalize -raw-dir ./data/raw
go run ./cmd/analyse -raw-dir ./data/raw
```

Player and player-session identifiers remain directly cross-referenceable with DayZ, Steam, and external moderation systems. Treat normalized databases, API responses, exports, logs, and backups as sensitive personal data.

## Operations and privacy

Retention requires PostgreSQL configuration. Privacy deletion can report matching raw events without a database, but execution requires PostgreSQL, an actor, and a reason.

```powershell
go run ./cmd/retention -raw-dir ./data/raw
go run ./cmd/privacy-delete -raw-dir ./data/raw -player-id '<durable-dayz-id>'
```

Both tools are report-only unless `-execute` is supplied.

See [Operations, security, and privacy](docs/operations.md) for backups, spool recovery, direct-identity handling, retention, deletion, administrator access, and troubleshooting.

## Documentation

- [Documentation index](docs/README.md)
- [Architecture](docs/architecture.md)
- [Analysis and review](docs/analysis-and-review.md)
- [Deployment](docs/deployment.md)
- [Operations, security, and privacy](docs/operations.md)

## Current maturity

The collector, authenticated ingest path, immutable raw storage, normalization, analysis, persistent review data, Steam-authenticated explorer, maps, spool recovery, retention, deletion workflows, direct identity output, and a first conservative gunshot/footstep cue model are implemented.

The project should still be treated as an evaluation system until each deployment has completed its own multiplayer callback validation, audio-model calibration, visibility validation, representative-load testing, trusted-cohort calibration, negative-control checks, and review-yield assessment.
