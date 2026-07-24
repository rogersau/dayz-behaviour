# Operations, security, and privacy

This guide covers day-to-day operation after the stack and DayZ mod are deployed. The system contains direct player identities and sensitive telemetry and must remain a restricted administrative service.

## Operational boundaries

- Keep `ingestd` and PostgreSQL on loopback or a private service network.
- Use the DayZ query token only across the server-host sidecar boundary.
- Expose only `reviewd` to administrators, through HTTPS. The standard Compose deployment gives it read-only access to raw telemetry for continuous normalization.
- Restrict raw telemetry, DayZ profiles, environment files, database backups, spool files, API output, and screenshots.
- No service sends enforcement commands to DayZ.

## Service responsibilities

| Service/tool | Responsibility |
|---|---|
| `ingestd` | Authenticate, validate, and store raw batches; import spool files |
| `reviewd` | Continuously normalize raw batches, serve evidence, and record dispositions |
| `normalize` | Run one-shot/manual normalization for recovery or direct development |
| `analyse` | Produce features and review rankings |
| `retention` | Report or enforce raw, normalized, and review retention |
| `privacy-delete` | Report or delete all references to one direct player ID |

## Routine checks

After a restart or during routine operation, check:

- collector health events continue arriving;
- raw batch files are increasing under the expected server/session path;
- the `reviewd` normalizer processes new batches;
- `ingestd` has no repeated authentication, validation, conflict, or storage failures;
- queues and spool files remain bounded;
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

When asynchronous HTTP export fails, the collector writes bounded NDJSON spool files in the DayZ profile area.

The importer:

1. waits for a file to become stable;
2. atomically claims it;
3. validates every batch;
4. stores batches idempotently;
5. archives the imported file;
6. restores the original name when import fails.

Recovery procedure:

1. restore `ingestd` connectivity and authentication;
2. confirm the spool directory is mounted or copied to the importer path;
3. watch logs for imported files, batches, and duplicates;
4. confirm recovered raw files exist;
5. confirm the `reviewd` normalizer processes them;
6. retain malformed spool files for investigation.

A rising spool-overwrite count means data was lost after the bounded spool filled. Record the period and treat analysis coverage as incomplete.

## Data locations

Compose volumes:

- `telemetry-data` — raw batches and imported spool area;
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
- deployment and collector configuration;
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

- terminate TLS at a trusted reverse proxy;
- set `DBA_PUBLIC_BASE_URL` to the exact external HTTPS origin;
- preserve forwarded host and scheme;
- use firewall, VPN, or identity-aware proxy controls where practical;
- avoid caching authenticated API, map, and timeline responses.

Do not expose the query-token ingest endpoint to the internet. If ingest crosses hosts, use a private authenticated tunnel or mTLS boundary.

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

The command cannot erase snapshots, copied directories, exported reports, or ticket attachments. Locate and handle those separately.

## Review-case handling

- Keep model output and administrator conclusions separate.
- Record behaviour, limitations, conventional evidence, and the final decision.
- Do not describe a tier as a cheat probability.
- Do not treat one disposition as unquestioned training ground truth.
- Restrict expanded payloads and exact routes to the admin boundary.
- Revisit stale cases after algorithm changes.

## Updating software

Before updating:

1. back up raw and PostgreSQL data;
2. record the current commit, DayZ build, mod set, schema, sampling, visibility, and audio model versions;
3. review migration and configuration changes;
4. test representative data;
5. repack/re-sign the mod when scripts change;
6. schedule validation for changed DayZ callbacks, audio, or visibility behaviour.

After updating:

1. confirm migrations complete;
2. verify ingest, normalization, login, timelines, and maps;
3. verify lifecycle and client RPC events;
4. compare health/drop counters;
5. replay a known fixture;
6. document result changes caused by new policies.

## Troubleshooting

### No raw files

Check mod loading, `enabled`, endpoint trailing `/`, token match, `ingestd` listener, connectivity, rejection logs, and DayZ spool files.

### Raw files but empty explorer

Check `reviewd` normalizer logs, its read-only `telemetry-data` mount, PostgreSQL connectivity, and migrations. For recovery, run `cmd/normalize` once against the same raw directory and database.

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
- unauthorized access or leaked credentials;
- player data exposed outside the admin boundary;
- sustained spool overwrites;
- migration checksum mismatch;
- incorrect map substitution;
- evidence used for automatic enforcement;
- deletion not propagated to backups or exports.

Preserve relevant logs without spreading player data, rotate affected credentials, isolate services when required, and document scope and remediation.
