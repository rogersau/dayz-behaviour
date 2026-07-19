# DayZ Behaviour

Behavioural telemetry and review-prioritisation tooling for identifying repeated hidden-awareness patterns in DayZ.

This project is designed for manual review support only. It does not make authoritative cheat or ban decisions.

## Current status

Milestone 0 is in progress. The first implementation slice contains:

- a Go telemetry ingestion service with versioned validation, authentication, durable raw storage and idempotent replay inputs;
- a development-only DayZ client/server probe for validating camera sampling, RPC attribution, authoritative snapshots, combat callbacks, visibility raycasts and asynchronous export;
- explicit documentation of which DayZ surfaces are implemented versus still requiring dedicated-server verification.

See:

- [Specification](docs/behavioural-awareness-spec.md)
- [DayZ script surface register](docs/dayz-script-surface-register.md)
- [Milestone 0 spike](docs/milestone-0-spike.md)
- [Implementation status](docs/implementation-status.md)
- [Issue #1](https://github.com/rogersau/dayz-behaviour/issues/1)

## Run the Go receiver

```bash
go test ./...
go vet ./...

export DBA_QUERY_TOKEN='replace-with-a-long-random-token'
go run ./cmd/ingestd
```

The receiver listens on `127.0.0.1:8080` by default and durably stores accepted raw batches under `./data/raw`.

Docker Compose is also available:

```bash
export DBA_QUERY_TOKEN='replace-with-a-long-random-token'
docker compose -f deploy/compose.yaml up --build
```

## Run the DayZ feasibility probe

The development mod source is under [`dayz/BehaviourProbe`](dayz/BehaviourProbe).

Pack and load it on both the client and dedicated server used for the Milestone 0 spike. On first launch it creates:

```text
$profile:DayZBehaviourProbe/config.json
```

Configure the loopback receiver endpoint, server identifier and the same query token used by the Go receiver. Visibility probes are disabled by default and should initially be enabled only in controlled fixtures.

The Enforce Script code has not yet been compiled or executed against a DayZ dedicated server. Treat it as a feasibility probe until the checks in `docs/milestone-0-spike.md` pass.
