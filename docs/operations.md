# Operations, security, and privacy

This guide covers day-to-day operation after the stack and DayZ mod are deployed. The system contains direct player identities and sensitive telemetry and must remain a restricted administrative service.

## Operational boundaries

- Keep `ingestd` and PostgreSQL on loopback or a private service network.
- Use the DayZ query token only across the loopback DayZ-to-agent or same-host boundary.
- Give every remote agent a separate upstream credential bound to one `server_id`.
- Expose only the telemetry batch path and the authenticated review service through HTTPS.
- Restrict agent configuration, outbox and dead-letter files, raw telemetry, DayZ profiles, environment files, database backups, spool files, API output, and screenshots.
- No service sends enforcement commands to DayZ.

## Service responsibilities

| Service/tool | Responsibility |
|---|---|
| server agent | Authenticate the local DayZ exporter, durably queue batches, import the DayZ spool, and retry central delivery |
| `ingestd` | Authenticate remote agents or a same-host exporter, validate batches, and store immutable raw data |
| `normalize` | Convert raw batches into PostgreSQL records |
| `analyse` | Produce features and review rankings |
| `reviewd` | Serve evidence and record dispositions |
| `retention` | Report or enforce raw, normalized, and review retention |
| `privacy-delete` | Report or delete all references to one direct player ID |

## Routine checks

After a restart or during routine operation, check:

- deployed agent and image versions match the intended pinned release;
- every agent service is running and its local status endpoint is healthy;
- agent queue bytes and batches return towards zero after an outage;
- agent upload success timestamps advance and dead letters remain zero;
- collector health events continue arriving;
- raw batch files are increasing under the expected server/session path;
- `normalize` processes new batches;
- `ingestd` has no repeated authentication, validation, conflict, capacity, or storage failures;
- agent outboxes and DayZ spool files remain bounded;
- dropped-event and dropped-opportunity counters are understood;
- PostgreSQL and filesystem capacity remain healthy;
- the explorer can list recent direct player sessions.

A low-volume server may legitimately produce few visibility or audio opportunities. Confirm lifecycle, snapshots, client health, and server collector health before diagnosing a stopped collector.

Before trusting analysis, confirm the selected data includes:

- validated strong-hidden observations;
- neutral no-relevant-target controls;
- visible positive controls;
- acceptable queue and clock quality;
- multiple independent sessions, encounters, and target IDs;
- no collection outage dominating the period.

Do not omit analyzer limitations when copying results into a ticket or case.

## Logs and health signals

### Agent health

The loopback agent status and metrics endpoints expose:

- running version and configured `server_id`;
- queued batch and byte totals;
- configured queue limits;
- uploaded batch and upload-failure totals;
- last upload attempt and success timestamps;
- the last upload error;
- dead-letter count.

A non-empty queue during a central outage is expected. A queue that never drains after connectivity returns, continuously increasing upload failures, a full outbox, or any dead letter requires investigation.

### Ingest health

`ingestd` distinguishes:

- accepted batches;
- idempotent duplicates;
- conflicting batch identities;
- unauthorized requests;
- schema or payload failures;
- storage failures;
- imported spool files and batches.

An identical duplicate is expected during retries. Reusing a batch identity with different content is a correctness incident.

### Collector health

Server and client health events include:

- samples captured and batches attempted;
- decision edges attempted;
- client samples dropped or queued;
- accepted and rejected RPC counts;
- random and enrichment queue depth;
- sampling opportunities dropped;
- pending and dropped export events;
- export success and failure counts;
- spool overwrite count.

Loss counters describe evidence quality, not player behaviour.

### Time quality

Clock challenges are single-use and must match the server-issued sequence and timestamp. High latency or uncertainty suppresses timing-sensitive observations.

Rejected clock responses may indicate an incompatible client, reconnect-state problem, or invalid RPC input. They never strengthen a ranking.

## DayZ export spool

When the DayZ mod cannot reach the local agent, it writes bounded NDJSON spool files in the DayZ profile area. This is the emergency layer behind the agent's own durable outbox.

The agent importer:

1. waits for a file to become stable;
2. atomically claims it;
3. validates every batch;
4. stores batches idempotently in the agent outbox;
5. archives the imported file;
6. restores the original name when import fails.

Recovery procedure:

1. restore the local agent service and its loopback listener;
2. confirm the configured DayZ spool directory is correct and accessible to the service account;
3. watch agent logs for imported files, batches, and duplicates;
4. confirm the agent queue accepts and forwards the recovered batches;
5. confirm central raw files appear and `normalize` processes them;
6. retain malformed spool and dead-letter files for investigation.

A rising spool-overwrite count means data was lost after the bounded spool filled. Record the period and treat analysis coverage as incomplete.

## Data locations

Agent directory:

- `dayz-behaviour-agent.json` — local and upstream credentials plus queue settings;
- `data/outbox` — accepted batches waiting for central acknowledgement;
- `data/dead-letter` — permanently rejected batches and reason metadata;
- the configured DayZ profile spool — emergency batches created when the local agent is unavailable.

Central Compose volumes:

- `telemetry-data` — immutable raw batches and central spool import area;
- `postgres-data` — PostgreSQL cluster.

Raw layout:

```text
<raw-root>/<server-id>/<server-session-id>/<20-digit-batch-sequence>.json
```

Raw batches are immutable. Do not edit them manually. Corrections belong in versioned normalization/analysis code or the deletion workflow.

PostgreSQL contains:

- raw batch metadata;
- normalized events and direct player sessions;
- sampling opportunities and visibility probes;
- encounters, episodes, windows, cues, and observations;
- features and algorithm runs;
- rankings, cases, dispositions, and privacy audit records.

Most derived data can be rebuilt from retained raw telemetry. Administrator-entered dispositions require database backups.

## Direct identities

The application stores:

```text
76561198000000000
<server-session-id>:<session-sequence>:76561198000000000
```

These values are designed for direct cross-reference with Steam, BattleMetrics, bans, Discord tickets, and other moderation systems.

There is no pseudonym mode, identity policy table, compatibility alias, or identity migration tool.

Protect:

- agent configuration, outbox, dead-letter, and DayZ spool files;
- central raw telemetry;
- PostgreSQL access;
- review API responses;
- explorer exports and screenshots;
- logs and analytics output;
- backups;
- ticket attachments.

Do not expose direct identities in public reports unless operational policy and applicable law permit it.

## Audio-model operations

### What the model uses

- gunshots: source/observer position, distance, weapon, ammunition, muzzle type, fire mode, suppressor state, and time;
- footsteps: source/observer position, distance, speed, gait, stance, surface, footwear, and time.

### What the model does not reproduce

- raw game audio;
- exact sound-set attenuation;
- room acoustics or door/window transmission;
- weather and ambient masking;
- reload, impact, item, infected, or voice sounds not modelled;
- hearing damage, headsets, equalizers, or volume settings;
- external communications.

### Calibration

Maintain a fixture covering:

- suppressed and unsuppressed shots at labelled distances;
- pistols, rifles, shotguns, automatic/burst, multi-muzzle, and modded weapons;
- standing, crouched, and prone movement;
- walk, jog, and sprint;
- barefoot and representative footwear;
- grass, dirt, roads, concrete, wood, metal, floors, and modded surfaces;
- indoor, outdoor, and obstructed scenes.

Record the model version, DayZ build, mod list, scene, expected class, observed experience, and deviations.

Change thresholds only through a new model version.

## Backups

Back up both raw telemetry and PostgreSQL.

A usable plan includes:

- filesystem-consistent raw-data copies or snapshots;
- PostgreSQL logical or physical backups;
- central deployment configuration and exact image tags/digests;
- protected server credential maps and agent configurations;
- collector configuration;
- administrator-entered review dispositions;
- documented restoration tests;
- the same retention and deletion obligations as live data.

Direct IDs make backups directly attributable personal data. Encrypt and restrict them accordingly.

Test restoration in isolation. Confirm migrations, normalization, session search, timelines, audio cues, review cases, and map access.

## Database migrations

Migration files are embedded and applied in filename order.

Rules:

- never edit a migration after it has been deployed;
- add a new numbered migration for future schema changes;
- review destructive statements separately;
- back up before schema changes;
- test upgrades against representative data;
- retain checksum mismatch as a hard stop.

The current repository is greenfield, so the initial schema directly uses the final player-ID fields without historical compatibility migrations.

## Administrator authentication

Browser administrators authenticate through Steam OpenID. Steam proves identity; authorization still requires membership in:

```text
DBA_STEAM_ADMIN_IDS=76561198000000000,76561198000000001
```

To change administrators:

1. update the allowlist;
2. restart `reviewd`;
3. verify access.

Rotate `DBA_SESSION_SECRET` to invalidate browser sessions.

Machine clients use:

```http
Authorization: Bearer <DBA_REVIEW_TOKEN>
```

Keep the browser-session secret, API token, and Steam allowlist independent.

## Network and TLS

For remote access:

- terminate TLS at Cloudflare Tunnel or another trusted reverse proxy;
- expose the ingest hostname only for `POST /v1/telemetry/batches`;
- authenticate each agent with its own credential bound to one `server_id`;
- set `DBA_PUBLIC_BASE_URL` to the exact external review origin;
- preserve forwarded host and scheme;
- protect the review hostname with Steam authorization plus an identity-aware proxy, VPN, or firewall controls where practical;
- avoid caching authenticated API, map, and timeline responses.

Do not expose the DayZ query-token endpoint to the internet. The query token is only for loopback. A distributed agent sends to central ingest over HTTPS using the server-specific upstream credential.

## Map operations

`reviewd` serves selected local map assets through authenticated routes. Captured `map_id` takes precedence. If the terrain is unavailable, fail closed instead of plotting on another map.

Exact routes and related-player positions are sensitive. Coarse visibility cells must remain coarse.

## Retention

`cmd/retention` manages:

```text
-raw-retention         default 30 days
-normalized-retention  default 90 days
-review-retention      default 365 days
```

Dry run:

```powershell
go run ./cmd/retention -raw-dir ./data/raw
```

Execute after review and approval:

```powershell
go run ./cmd/retention -raw-dir ./data/raw -execute
```

Record the command, operator, approval, counts, and result. External backups and exports need equivalent retention handling.

The central retention tool does not manage remote agent outboxes, dead-letter files, archived DayZ spool files, or copied agent directories. Include those locations in operational retention and deletion procedures. Healthy outbox files should be transient; dead letters require investigation before removal.

## Player deletion

Deletion matches a player when they are:

- event owner;
- observer or target;
- combat source;
- durable or nested session reference.

Dry run:

```powershell
go run ./cmd/privacy-delete `
  -raw-dir ./data/raw `
  -player-id '<durable-dayz-id>'
```

Execute:

```powershell
go run ./cmd/privacy-delete `
  -raw-dir ./data/raw `
  -player-id '<durable-dayz-id>' `
  -actor '<operator>' `
  -reason '<ticket-id>' `
  -execute
```

Execution deletes or rewrites matching raw batches, removes normalized and review references, updates raw metadata, and writes an audit record containing the direct `subject_player_id` and affected counts.

The command cannot erase remote agent outboxes, dead-letter files, DayZ spool archives, snapshots, copied directories, exported reports, or ticket attachments. Locate and handle those separately. Stop or isolate affected agents before deletion when they may still hold queued copies for the subject.

## Review-case handling

- Keep model output and administrator conclusions separate.
- Record behaviour, limitations, conventional evidence, and the final decision.
- Do not describe a tier as a cheat probability.
- Do not treat one disposition as unquestioned training ground truth.
- Restrict expanded payloads and exact routes to the admin boundary.
- Revisit stale cases after algorithm changes.

## Updating software

Use exact release versions for both central images and the Windows agents. Record image digests and agent versions with the change record. See [Releases and published images](releases.md) for the complete upgrade and rollback procedure.

Before updating:

1. back up raw telemetry, PostgreSQL, deployment configuration, and the server credential map;
2. record the current release tags, image digests, agent versions, DayZ build, mod set, schema, sampling, visibility, and audio model versions;
3. review migration, configuration, collector, and transport changes;
4. test representative data and restoration procedures;
5. retain the previous agent ZIP and exact image tags;
6. repack/re-sign the mod when scripts change;
7. schedule validation for changed DayZ callbacks, audio, or visibility behaviour.

Roll out in this order:

1. pull the new central and review images;
2. update central `ingestd`, `normalize`, and `reviewd` together;
3. confirm migrations and central health;
4. update one agent and confirm its queue drains;
5. continue agent by agent;
6. resume scheduled analysis after the collection path is stable.

After updating:

1. verify ingest, normalization, login, timelines, and maps;
2. verify agent versions, queue depth, upload success, and zero dead letters;
3. verify lifecycle and client RPC events;
4. compare health/drop counters;
5. replay a known fixture;
6. document result changes caused by new policies.

Do not roll back across a destructive database migration without restoring a compatible backup.

## Troubleshooting

### No raw files

Check mod loading, `enabled`, the loopback endpoint trailing `/`, the local token match, agent service state, agent status, queue depth, upstream HTTPS connectivity, central credential-map membership, `server_id` agreement, central rejection logs, and DayZ spool files.

### Agent queue does not drain

Check the public ingest hostname, Tunnel health, TLS and DNS, the upstream credential, the claimed `server_id`, central capacity, and the last agent upload error. HTTP 5xx and network failures should retry; permanent 4xx rejections move the batch to dead letter.

### Agent has dead letters

Preserve the batch and reason metadata. Check schema compatibility, batch identity conflicts, request size, and whether the agent and central services use the intended matching release. Do not delete the evidence until the rejection is understood.

### Raw files but empty explorer

Check `normalize`, PostgreSQL connectivity, migrations, normalizer logs, and that `reviewd` uses the same database.

### Steam login loop

Check `DBA_PUBLIC_BASE_URL`, HTTPS forwarded headers, Steam admin allowlist, system time, and browser cookies.

### No audio cues

Check `enable_audio_cues`, `EEFired` execution, movement threshold/cadence, shot and movement events in raw data, and observer snapshots before the sound.

### Only `INSUFFICIENT_DATA`

Check visibility validation, hidden and neutral opportunities, cues, timing, breadth, collection loss, and retention. Insufficient evidence is an intended outcome.

### No high-priority results

Inspect matched-model convergence, useful strata, odds-ratio lower bound, session stability, negative controls, and readiness-lift lower bound. Do not weaken gates merely to fill the queue.

## Incident response

Treat these as incidents:

- batch identity conflicts;
- unauthorized access or leaked local, agent, Tunnel, review, or database credentials;
- an agent credential accepted for the wrong `server_id`;
- player data exposed outside the admin boundary;
- a full agent outbox, unexplained dead letters, or sustained DayZ spool overwrites;
- unexpected agent or central image versions/digests;
- migration checksum mismatch;
- incorrect map substitution;
- evidence used for automatic enforcement;
- deletion not propagated to backups or exports.

Preserve relevant logs without spreading player data, rotate affected credentials, isolate services when required, and document scope and remediation.
