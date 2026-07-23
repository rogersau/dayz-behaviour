# Operations, security, and privacy

This guide covers day-to-day operation after the stack and DayZ mod are deployed. The system contains direct player identities, exact routes, combat context, inferred audibility, and review data. Operate it as a restricted administrative service.

## Operational boundaries

- `ingestd` and PostgreSQL should remain on loopback or a private service network.
- The DayZ query token is suitable only for the sidecar boundary created by the server host.
- Remote administrator access should reach only `reviewd` through HTTPS.
- Raw telemetry, DayZ profiles, environment files, database backups, spool files, API exports, and screenshots must be accessible only to authorized operators.
- No service sends analysis or enforcement commands to DayZ.
- Audio cues are inferred from game state; no raw audio is captured.

## Service responsibilities

| Service/tool | Operational responsibility |
|---|---|
| `ingestd` | Authenticate, validate, and durably store raw batches; import DayZ spool files |
| `normalize` | Continuously convert new raw batches into direct-identity PostgreSQL records |
| `analyse` | Build cue facts, feature results, and review rankings |
| `reviewd` | Serve authenticated evidence and record review dispositions |
| `retention` | Report or enforce independent raw/normalized/review retention windows |
| `privacy-delete` | Report or delete all references to one durable player identity |
| `direct-identity-rebuild` | Replace older pseudonymized derived data by replaying retained raw batches with direct identities |

## Routine checks

### Daily or after a server restart

Check:

- DayZ collector health events continue to arrive;
- raw batch files are increasing under the expected server/session path;
- `normalize` is processing new batches;
- no repeated authentication, validation, conflict, or storage failures appear in `ingestd` logs;
- event, probe, and export queues remain bounded;
- dropped-event and dropped-opportunity counters are understood;
- PostgreSQL and filesystem capacity remain healthy;
- the explorer can list recent direct player/session identities;
- gunshot and movement-audio events are present when expected.

A low-volume server may legitimately produce few visibility opportunities. Distinguish low opportunity counts from a stopped collector by checking lifecycle, snapshots, audio opportunities, client health, and server collector health.

### Before running or trusting analysis

Confirm the selected data contains:

- validated strong-hidden observations;
- neutral no-relevant-target controls;
- visible positive controls;
- plausible audio-cue coverage;
- acceptable queue delay and clock uncertainty;
- multiple independent sessions, encounters, and target identities;
- no unexplained collection outage dominating the period.

The analyzer reports data-quality limitations. Do not discard those messages when copying results into a case or ticket.

## Logs and health signals

### Ingest health

`ingestd` should distinguish:

- accepted batches;
- idempotent duplicate batches;
- conflicting batch identities;
- unauthorized requests;
- schema or payload validation failures;
- raw-storage failures;
- spool files and batches imported.

A duplicate with identical content is expected during retry or spool recovery. A duplicate batch identity with different content is a correctness incident and should be investigated.

### Collector health

Server and client health events contain counters such as:

- samples captured, batches attempted, and decision edges attempted;
- client samples dropped or queued;
- accepted and rejected RPC counts;
- random and event-enrichment queue depth;
- random and enrichment opportunities dropped;
- pending and dropped export events;
- export success and failure count;
- spool overwrite count.

Loss counters are context for evidence quality. They must not be interpreted as player behaviour.

### Audio collection health

During controlled checks, confirm:

- `SHOT_FIRED_SERVER` appears once for each successful server-side shot callback;
- shooter ID, weapon, ammunition, suppressor state, position, and timestamp are populated;
- `MOVEMENT_AUDIO_OPPORTUNITY` appears at the configured bounded cadence only when speed exceeds the threshold;
- stance, surface, footwear, position, and speed are populated;
- audio opportunity volume is consistent with the configured interval and player count.

A missing audio event means the system did not capture the cue. It does not prove the game produced no sound.

### Time quality

Clock challenges are single-use, expire, and must match the server-issued sequence and send time. High round-trip delay or uncertainty suppresses timing-sensitive observations.

Repeated rejected clock responses may indicate a broken or incompatible client collector, reconnect-state issue, or malicious RPC input. They do not strengthen a candidate ranking.

## DayZ export spool

When asynchronous HTTP export fails, the DayZ collector writes append-only NDJSON spool files in its profile area. File count and batches per file are bounded by configuration.

The standard Compose deployment exposes the DayZ spool directory to `ingestd` as `/data/dayz-spool`. The importer:

1. waits for a spool file to become stable;
2. atomically claims it;
3. decodes and validates every batch;
4. stores batches idempotently in the raw tier;
5. moves the source into an archive directory;
6. restores the original file name if importing fails.

### Recovery procedure

1. Restore `ingestd` connectivity and authentication.
2. Confirm the DayZ profile spool directory is mounted or copied to the importer directory.
3. Watch `ingestd` logs for imported files, batches, and duplicates.
4. Confirm recovered raw batch files exist.
5. Confirm `normalize` processes the recovered batches.
6. Retain failed spool files for investigation when decoding or validation fails.

A rising spool-overwrite counter means the bounded spool filled before recovery. Record the affected time and treat analysis coverage as incomplete.

## Data locations

### Compose volumes

- `telemetry-data` stores raw batches and the imported DayZ spool area.
- `postgres-data` stores the PostgreSQL cluster.

### Raw layout

```text
<raw-root>/<server-id>/<server-session-id>/<20-digit-batch-sequence>.json
```

Raw batches are immutable. Do not edit them manually. Corrections are made through versioned normalization/analysis code or the privacy-deletion workflow.

### PostgreSQL

PostgreSQL contains derived and operational tables for:

- raw batch metadata;
- normalized events and direct player sessions;
- sampling opportunities and visibility probes;
- encounters, episodes, decision windows, cues, and observations;
- feature results and algorithm runs;
- candidate rankings and review cases;
- dispositions and privacy audit records.

Most PostgreSQL data can be rebuilt from retained raw telemetry. Review dispositions and other administrator-entered information do not exist in raw game events and must be backed up separately.

## Direct identity policy

The application stores the durable DayZ identity and full player-session identity directly:

```text
76561198000000000
<server-session-id>:<session-sequence>:76561198000000000
```

This allows direct cross-reference with Steam, BattleMetrics, bans, tickets, and other moderation systems.

It also removes pseudonymization as a privacy control. Protect:

- PostgreSQL access;
- API responses;
- browser exports and screenshots;
- log aggregation;
- backups;
- analytics output files;
- ticket attachments.

Do not expose direct identities in public reports unless your policy and legal basis permit it.

### Existing pseudonymized databases

Older versions used one-way HMAC or deterministic hashes. They cannot be reversed.

The database will fail closed when its recorded identity policy does not match `direct-identifiers-v1`. Use the explicit rebuild workflow instead of changing policy rows manually.

Dry run:

```powershell
go run ./cmd/direct-identity-rebuild `
  -raw-dir ./data/raw `
  -database-url $env:DBA_DATABASE_URL
```

Execute after backup and approval:

```powershell
go run ./cmd/direct-identity-rebuild `
  -raw-dir ./data/raw `
  -database-url $env:DBA_DATABASE_URL `
  -execute `
  -confirm REBUILD_WITH_DIRECT_IDENTITIES

go run ./cmd/analyse -raw-dir ./data/raw
```

The command:

1. reports exact derived-table counts;
2. preserves restricted raw files, raw batch metadata, and the privacy audit log;
3. clears normalized events, sessions, cues, observations, features, rankings, cases, and dispositions;
4. switches the database identity policy;
5. replays raw batches with direct identities;
6. removes the normalization checkpoint so continuous normalization can resume safely.

Export any reviewer dispositions that must be retained before execution. The raw replay cannot recreate them.

## Audio-model operations

### What the model does

The model assigns a versioned audibility class using captured game facts:

- gunshots: distance, weapon, ammunition, suppressor state, positions, and time;
- footsteps: distance, speed/gait, stance, surface, footwear, positions, and time.

### What the model does not do

It does not reproduce:

- raw game audio;
- exact sound-set attenuation;
- building-room acoustics or door/window transmission;
- weather and ambient masking;
- infected, item, reload, impact, or voice sounds not yet modelled;
- hearing damage, headsets, equalizers, or client volume settings;
- external voice communications.

### Calibration procedure

Maintain a deployment-specific fixture covering:

- suppressed and unsuppressed weapons at labelled distances;
- pistols, rifles, shotguns, automatic/burst fire, and modded weapons;
- standing, crouched, and prone movement;
- walk, jog, and sprint;
- barefoot and representative footwear;
- grass, dirt, road, concrete, wood, metal, building floors, and modded surfaces;
- indoor/outdoor and obstructed scenarios, even when the first model cannot represent them.

Record model version, DayZ build, mod list, environment, expected classification, observed player experience, and deviations.

Change thresholds by publishing a new audio-model version. Do not silently reinterpret historical results under the same version.

## Backups

Back up both the raw and PostgreSQL tiers.

A usable backup plan should include:

- filesystem-consistent copies or snapshots of `telemetry-data`;
- PostgreSQL logical or physical backups appropriate to the deployment;
- deployment configuration and DayZ collector configuration;
- administrator-entered review dispositions;
- documented restoration tests;
- the same retention and deletion obligations as the live system.

Because player IDs are direct, backups contain directly attributable personal data. Encrypt and restrict them accordingly.

Test restoration into an isolated environment. Confirm migrations, normalization, direct session search, timelines, cue facts, review cases, and map access before declaring the backup valid.

## Database migrations

Migration files are embedded into the Go binary and applied in filename order.

Rules:

- never edit an applied migration;
- add a new numbered migration;
- review destructive statements separately;
- back up before schema changes;
- test upgrades against a copy of representative data;
- retain the migration checksum failure as a hard stop.

The migration table records a SHA-256 checksum. A changed migration file fails closed instead of silently mutating the history of the database schema.

## Administrator authentication

Browser administrators authenticate through Steam OpenID. Steam proves the account identity; it does not grant authorization.

Authorization requires the verified SteamID64 to appear in:

```text
DBA_STEAM_ADMIN_IDS=76561198000000000,76561198000000001
```

To add or remove an administrator:

1. update the allowlist;
2. restart `reviewd`;
3. verify the expected account can or cannot access the explorer.

Every browser request is checked against the current allowlist after restart. Rotate `DBA_SESSION_SECRET` to invalidate every existing browser session.

Machine clients use:

```http
Authorization: Bearer <DBA_REVIEW_TOKEN>
```

Keep the browser-session secret, review token, and Steam administrator allowlist independent.

## Network and TLS

For remote access:

- terminate TLS at a trusted reverse proxy;
- set `DBA_PUBLIC_BASE_URL` to the exact external HTTPS origin;
- preserve forwarded host and scheme;
- use firewall, VPN, identity-aware proxy, or equivalent additional controls where available;
- avoid caching authenticated API, map, and timeline responses.

The browser cookie is HTTP-only, SameSite=Lax, signed, and short-lived. HTTPS enables the Secure flag through the configured public base URL.

Do not expose the query-token ingest transport to the internet. If ingest must cross a host boundary, place it behind a private authenticated tunnel, mTLS proxy, or equivalent protected transport rather than relying on the query parameter alone.

## Map operations

The review image contains selected local map assets for Chernarus, Livonia, Sakhal, and Namalsk. `reviewd` serves map metadata and tiles through authenticated routes; there is no separate map service.

The captured event `map_id` wins. `DBA_DEFAULT_MAP` is used only when a session contains no map identity. When the required terrain is unavailable, the explorer must fail closed rather than reinterpret coordinates on another map.

Exact routes and related-player positions are sensitive telemetry. Coarse visibility cells must remain coarse.

## Retention

`cmd/retention` manages three independent classes:

```text
-raw-retention         default 30 days
-normalized-retention  default 90 days
-review-retention      default 365 days
```

Run a report first:

```powershell
go run ./cmd/retention -raw-dir ./data/raw
```

Review the counts, obtain the required operational approval, and then execute:

```powershell
go run ./cmd/retention -raw-dir ./data/raw -execute
```

Use explicit duration flags when policy differs from the defaults. Record the command, operator, approval, counts, and execution result.

Retention of a database row does not remove an external backup or copied log. Backup retention must implement the same policy.

## Player identity deletion

The deletion workflow removes events where the player is:

- the event owner;
- an observer or target;
- a combat or audio source;
- a durable or nested session identity reference.

### Dry run

```powershell
go run ./cmd/privacy-delete `
  -raw-dir ./data/raw `
  -player-id '<durable-dayz-id>'
```

### Execute

```powershell
go run ./cmd/privacy-delete `
  -raw-dir ./data/raw `
  -player-id '<durable-dayz-id>' `
  -actor '<operator>' `
  -reason '<request-or-ticket-id>' `
  -execute
```

Execution:

1. deletes raw files containing only matching events;
2. safely rewrites mixed raw batches;
3. removes normalized, cue, observation, feature, ranking, case, and disposition references;
4. removes raw batch metadata for affected batches;
5. writes a privacy audit record containing the direct subject identity and affected counts.

After rewriting mixed raw batches, rerun normalization as documented by your deployment process.

The command cannot erase database snapshots, copied raw directories, exported reports, ticket attachments, or other systems. Those copies must be located and handled under the same request.

## Review-case handling

- Keep the model output and administrator conclusion separate.
- Record observable behaviour, cue facts, data limitations, conventional evidence, and the final operational decision.
- Do not describe a tier as a cheat probability.
- Do not state that an audio cue was definitely heard.
- Do not use one reviewer’s disposition as unquestioned training ground truth.
- Restrict expanded payloads, direct identities, and exact route coordinates to the admin boundary.
- Close or annotate stale cases when later replay changes the analysis version or eligibility.

## Updating the software

Before updating:

1. back up raw and PostgreSQL data;
2. record the current application commit, DayZ mod build, schema/migration state, sampling policy, visibility policy, audio policy, and identity policy;
3. review migration and configuration changes;
4. test against a copy of representative data;
5. repack/re-sign the DayZ mod when script files changed;
6. schedule validation for changed DayZ callbacks, audio context, or visibility behaviour.

After updating:

1. confirm migrations complete;
2. verify ingest, normalization, explorer login, direct identities, timelines, and maps;
3. verify DayZ lifecycle, shot, movement-audio, and client RPC events;
4. compare health/drop counters with the previous version;
5. replay a known fixture and confirm the intended algorithm versions;
6. document any result changes caused by a new builder, cue, feature, or ranking policy.

## Troubleshooting

### No raw files are appearing

Check:

- the mod is loaded on the dedicated server;
- `enabled` is true;
- the endpoint ends in `/`;
- the token matches `DBA_QUERY_TOKEN`;
- `ingestd` is listening on the expected loopback address;
- the DayZ host can reach that address;
- `ingestd` logs do not show authentication or validation rejection;
- the DayZ profile contains spool files indicating export failure.

### Raw files exist but the explorer is empty

Check:

- `normalize` is running and can connect to PostgreSQL;
- the database identity policy is `direct-identifiers-v1`;
- an older pseudonymized database has been rebuilt;
- migrations completed;
- normalization logs show processed batches rather than errors;
- the explorer is using the same PostgreSQL database.

### Gunshots are missing

Check:

- `enable_audio_cues` is true;
- the server-side `Weapon_Base.EEFired` override compiled and loaded;
- the target weapon calls the expected base method;
- server logs and raw events show `SHOT_FIRED_SERVER`;
- the source player is the weapon hierarchy root;
- another mod did not replace the callback without chaining `super`.

### Footstep cues are missing or excessive

Check:

- `audio_context_interval_seconds` and `audio_min_movement_speed_mps`;
- authoritative velocity values on the target build;
- stance and `Feet` attachment values;
- `SurfaceGetType3D` results;
- whether movement events are being dropped under export load;
- whether the cue is outside the Go model’s configured range.

### Steam login loops or callback fails

Check:

- `DBA_PUBLIC_BASE_URL` exactly matches the externally visible origin;
- HTTPS and forwarded headers are preserved by the proxy;
- the SteamID64 is in the administrator allowlist;
- system time is correct;
- the browser accepts the signed cookie.

### A session has no map

Check:

- captured events contain the expected `map_id`;
- the review image includes that map;
- the configured fallback is valid only for sessions without captured map identity;
- tile requests remain authenticated and are not blocked by the proxy.

### Analysis produces only `INSUFFICIENT_DATA`

Check:

- visibility probing and first-person validation are configured where required;
- hidden and neutral opportunities are both present;
- clock and queue timing meet policy;
- enough independent sessions, encounters, and targets exist;
- known or plausible cues explain most observations;
- collection loss or retention has not removed required history.

This is not necessarily an error. Failing closed on insufficient evidence is intended behaviour.

## Incident response

Treat the following as operational incidents:

- raw batch identity conflicts;
- direct identity or route exposure outside the restricted boundary;
- unauthorized access or leaked tokens;
- sustained spool overwrites or unexplained collector loss;
- migration checksum mismatch;
- incorrect audio model described as proof of hearing;
- incorrect map substitution or coordinate precision;
- evidence used for automatic enforcement;
- deletion requests not propagated to backups or exports.

Preserve relevant logs and configuration without spreading player telemetry further. Rotate affected credentials, isolate services when required, and document the scope and remediation.

## Production-readiness evidence

An operator should maintain deployment-specific records for:

- exact DayZ build and loaded mod set;
- script compilation and runtime callback fixtures;
- gunshot and movement-audio callback validation;
- audio-model calibration and known limitations;
- visibility confusion matrix and validation ID;
- clock and timing distributions;
- representative player-count performance and queue loss;
- trusted-cohort calibration and negative controls;
- blinded or quality-controlled review yield;
- backup restoration;
- direct-identity rebuild, retention, and privacy-deletion exercises;
- administrator-access review;
- change history for sampling, cue, analysis, and ranking policies.

The repository’s implementation alone cannot establish these environment-specific properties.
