# Implementation status

Status date: 2026-07-19
Target verified build: DayZ Server 1.29.0.163451 on Windows

## Current position

The implementation is structurally aligned with the production plan through the first operational-release architecture. A one-client diagnostic fixture now validates the client camera/RPC path and basic server lifecycle, but controlled multiplayer visibility, combat hook execution, representative load, trusted-cohort calibration and blinded-review yield still require additional connected players and labelled scenarios.

No ranking can trigger gameplay action. Strong hidden evidence fails closed unless a validated first-person policy ID, minimum occlusion duration, timing limits, repeated breadth, matched-model stability and negative controls all pass.

## Implemented and automated

### DayZ collector

- adaptive local camera/readiness sampling with a pure transition detector;
- compact decision-edge, clock-alignment and collector-health RPCs;
- nonce, schema, sender identity, monotonic sequence/time, payload, whitelist and rate validation;
- server-authenticated player-session lifecycle across connect, ready, respawn, reconnect and disconnect;
- authoritative snapshots, movement state/transitions, hit and kill capture;
- separate random-opportunity and event-enrichment queues with bounded work, admission probabilities, drop counters and queue timing;
- adaptive head/torso/pelvis/upper-body visibility evidence and ambiguous-blocker handling;
- safe head-origin semantics, explicit validated-first-person opt-in and minimum-duration confirmation;
- asynchronous REST export, bounded in-memory queues, bounded rotating spool and periodic health events.

### Go data plane

- strict versioned schema and authority/sampling validation;
- authenticated loopback-first ingestion, request limits, durable fsync-before-ack storage, conflict detection and metrics;
- automatic DayZ spool import and deterministic replay;
- PostgreSQL migrations, idempotent raw/event/session/sampling/visibility normalisation and analyst-facing pseudonyms;
- independent raw, normalized and review retention; dry-run defaults; auditable durable-identity deletion;
- deterministic golden replay fixture.

### Analysis and review

- versioned encounter, observer-target episode, refractory and prospective decision-window construction;
- cue facts and conservative `UNEXPLAINED_IN_CAPTURED_DATA` language;
- exact within-player context matching and conditional logistic fitting with separation suppression;
- beta-binomial shrinkage fallback, circular permutation, pre-exposure summary, robust cohort scoring and Benjamini-Hochberg adjustment;
- leave-one-session-out stability and negative-control fail-closed gates;
- persistent feature results, rankings, cases, algorithm runs and dispositions;
- authenticated review API, required endpoint aliases and filters;
- Steam OpenID admin authentication with an explicit SteamID64 allowlist and signed, expiring browser sessions;
- a read-only browser explorer with searchable pseudonymized sessions, authority-labelled contextual timelines, related-player context and expandable event evidence.
- a self-contained map adapter with Chernarus, Livonia, Sakhal and Namalsk tiles, exact/coarse location semantics, selected/related routes and map/layer controls.

## Evidence completed locally

- all Go tests and `go vet ./...` pass;
- Docker image and Compose configuration build successfully;
- PostgreSQL migrations and replay are idempotent on PostgreSQL 17;
- the PBO packs and all Game, World and Mission modules compile/load on DayZ 1.29;
- a matched DayZDiag client/server run passed the automated client check: nonce receipt, camera sampling and authenticated sample-batch acceptance;
- the one-client run exercised connect, new-character ready and disconnect lifecycle events plus clock alignment, decision edges, client health and authoritative snapshots;
- authenticated DayZ-to-Go export persisted 1,389 normalized development events across 520 immutable batches, with the admin explorer reading the resulting sessions;
- receiver-down testing produced a bounded spool file and automatic import archived it after durable storage;
- persistent review API returned health, candidates and a completed algorithm run.

## Runtime evidence still required

These are evidence gates, not hidden claims of completion:

1. Add a second instrumented client and verify remote-entity behavior, raised/ADS/optics transitions, `OnFire`, respawn/reconnect, hit and kill paths. The one-client camera, RPC and connect/new-ready/disconnect paths are validated.
2. Run labelled first- and third-person visibility fixtures across stance, geometry, motion and blocker classes; issue a validation ID only if the false-occlusion limit passes.
3. Measure clock uncertainty, edge-to-receive latency, decision windows and safe sampling/probe budgets under representative population.
4. Run silent trusted-player calibration, controlled information interventions, all position/identity/sector negative controls and held-out stability.
5. Demonstrate operator-approved gameplay tolerance, false-priority tolerance, blinded-review yield and lift over random selection.

Until those gates pass, the system can collect and analyse development data but must not be presented as a production-capable detector.
