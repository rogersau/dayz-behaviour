# Implementation inventory and plan mapping

Status date: 2026-07-19

| Plan boundary | Repository owner | Current implementation | Verification |
|---|---|---|---|
| Client state and transitions | `dayz/BehaviourProbe/scripts/5_Mission/DayZBehaviour/DBAMissionGameplay.c` and `DBAClientTransitionDetector` | Adaptive state samples, pure edges, bounded batches, health | Server module load only; connected-client fixture pending |
| RPC contract and trust | `DBAProbeTypes.c` / `DayZGame.OnRPC` | Server nonce, sender attribution, schema/sequence/range/rate checks | Compiles on 1.29; live client/remote execution pending |
| Authoritative collection | `DBAMissionServer.c`, `DBAPlayerProbe.c`, `DBAWeaponProbe.c` | Lifecycle, snapshots, movement, hit/kill and shot spike | Dedicated module load; callback-value fixtures pending |
| Sampling scheduler | `DBAMissionServer.c` | Separate random and enrichment queues, probabilities, timing, bounded load | Code and module load verified; population profiling pending |
| Visibility | `DBAVisibilityProbe.c` | Adaptive points, blocker ambiguity, origin policy, duration confirmation | Code and module load verified; confusion matrix pending |
| Export/spool | `DBAProbeExporter.c`, `internal/spool` | Async POST, bounded rotation, automatic import | Live success and receiver-outage replay verified |
| Raw ingestion | `internal/ingest`, `internal/storage`, `cmd/ingestd` | Auth, limits, fsync/rename, idempotency conflict, metrics | Unit and live DayZ export verified |
| Normalisation/replay | `internal/postgres`, `internal/replay`, `cmd/normalize`, `cmd/replay` | Versioned schema, pseudonyms, specialized tables, deterministic replay | PostgreSQL 17 replay and repeat run verified |
| Observation model | `internal/observations` | Sessions, encounters, episodes, windows, cue ledger, timing suppression | Unit and golden replay verified |
| Primary analytics | `internal/features`, `internal/ranking`, `cmd/analyse` | Matching, conditional logit, shrinkage, sector/pre-exposure, stability and controls | Synthetic/golden tests; calibrated effect validity pending |
| Review experience | `internal/review`, `cmd/reviewd` | Persistent API, filters, evidence structures, dispositions, basic UI | Unit and PostgreSQL-backed API smoke verified |
| Privacy/retention | `internal/privacy`, `internal/retention`, `cmd/privacy-delete`, `cmd/retention` | Pseudonymisation, independent retention, audited deletion, dry-run safety | Unit and database dry-run verified |

There is one implementation per seam. DayZ owns live-engine facts and bounded probes; Go owns persistence, reconstruction, analysis and review. No database or lifetime analysis runs in DayZ, and no analysis result is sent back to gameplay.

## Deferred by the plan

Phase-2 changepoint/Isolation Forest and phase-3 route/squad models remain research work. They are not required for the recommended first operational release and cannot independently create a highest-priority case.

## Remaining evidence work and ownership

| Work item | Owner role | Dependency |
|---|---|---|
| Connected hook/camera/RPC/combat matrix | DayZ mod engineer + server operator | Two instrumented 1.29 clients |
| Visibility confusion matrix and policy ID | DayZ mod engineer + analyst | Labelled observer/target fixtures across camera/stance/blockers |
| Representative load and latency budget | Server operator | Agreed population bands and baseline comparison |
| Trusted cohort/intervention/holdouts | Analyst + privacy-approved operator | Participant cohort and collection notice |
| Blinded review yield evaluation | Review lead | Calibrated candidate and random case mix |

These items extend the single implementations listed above; they do not require parallel replacement components.
