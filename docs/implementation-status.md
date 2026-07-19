# Implementation Status

## Feature branch

`feature/milestone-0-foundation`

## Implemented

### External Go service

- versioned telemetry batch and event schema;
- strict request and payload validation;
- bearer-token authentication for conventional producers;
- query-token authentication for the DayZ `RestContext` constraint;
- loopback-first HTTP server with health and readiness endpoints;
- immutable, fsynced, idempotent raw batch storage;
- graceful shutdown;
- Docker and Compose deployment;
- unit tests covering schema validation, authentication, idempotency and storage.

### DayZ development probe

- adaptive local camera/state sampling from `MissionGameplay.OnUpdate`;
- bounded primitive `ScriptRPC` batches;
- server nonce and monotonic per-player batch validation;
- RPC attribution from server-supplied `PlayerIdentity`;
- authoritative player snapshots from `MissionServer.OnUpdate` and `GetGame().GetPlayers`;
- `Weapon_Base.OnFire` execution-side instrumentation and local shot markers;
- `PlayerBase.EEHitBy` and `PlayerBase.EEKilled` capture;
- optional bounded head/torso `RaycastRVProxy` visibility observations;
- asynchronous `RestContext.POST` export;
- append-only failed-export spool.

## Explicitly unverified until run in DayZ

The Go code has been tested locally. Enforce Script cannot be compiled in this environment, so the following remain Milestone 0 runtime checks rather than completed claims:

- exact `DayZGame.OnRPC` behaviour for global project-owned RPC IDs with the target mod set;
- client and dedicated-server execution contexts of `Weapon_Base.OnFire`;
- actual callback values from `EEHitBy` and `EEKilled`;
- head and `Spine3` positions across all stances;
- `RaycastRVProxy` classification around terrain, doors, vehicles, base-building objects and vegetation;
- `RestContext.POST` response behaviour and timeout handling on the target dedicated server;
- safe sample, RPC, export and visibility-probe budgets at representative population sizes.

## Known transport constraint

The exposed DayZ `RestContext` API provides content-type configuration but no general arbitrary-header setter. The development probe therefore supports a URL query token loaded from the server profile and requires the Go receiver to remain bound to loopback.

This is acceptable for the local Milestone 0 sidecar. It is not the final design for traffic crossing a network boundary.

## Not implemented yet

- automatic replay of DayZ spool files;
- PostgreSQL normalisation;
- timeline and engagement reconstruction;
- matched controls and behavioural feature calculation;
- squad analysis;
- review API or dashboard;
- production visibility scheduler and cooldowns;
- automatic enforcement of any kind.
