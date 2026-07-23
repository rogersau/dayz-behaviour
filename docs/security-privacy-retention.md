# Security, privacy and retention operations

## Network and credentials

- Ingestion and the admin explorer bind to loopback by default. Compose publishes only on `127.0.0.1`.
- Ingestion requires a bearer or DayZ query token. Browser review uses Steam OpenID plus an explicit SteamID64 admin allowlist; machine API access requires the review bearer token.
- Steam authentication proves account identity only. `DBA_STEAM_ADMIN_IDS` is the authorization boundary and must contain only current administrators. Remove departed administrators promptly and restart the service after changing the environment.
- Browser sessions are HMAC-signed, HTTP-only, SameSite=Lax and expire after 12 hours. Use an unpredictable `DBA_SESSION_SECRET` of at least 32 characters. HTTPS public URLs automatically enable the Secure cookie flag.
- DayZ's `RestContext` cannot set an arbitrary authorization header, so its query token is acceptable only for the loopback sidecar. Do not expose that transport across a network boundary; use a private authenticated proxy or mTLS termination instead.
- Rotate runtime tokens through environment/configuration management. Historical event authority is based on server/session/source IDs and does not depend on retaining an old credential.
- Raw files, DayZ profiles and database credentials must be accessible only to operators responsible for telemetry or privacy requests.
- Map assets are read locally by `reviewd`. Metadata and tiles remain behind the explorer's authenticated, path-validated routes; there is no separately exposed map process.

## Identity boundary

The restricted raw tier contains durable DayZ identifiers because the server must bind RPCs, join events and support identity deletion. Normalisation converts player sessions, durable IDs and known payload identity fields to deterministic pseudonyms before analyst/review access.

Production deployments must configure:

```text
DBA_PSEUDONYM_SECRET=<stable random value of at least 32 bytes>
DBA_PSEUDONYM_KEY_ID=<operator-managed key identifier, for example production-v1>
```

Pseudonyms use HMAC-SHA-256 with domain prefixes:

```text
dp_<digest>  durable player identity
ps_<digest>  player session identity
```

The database stores the pseudonym policy version and key ID. Startup/migration fails when the process policy differs from the database policy; a key cannot be changed silently because doing so would create a second identity namespace and break joins/deletion. Key rotation therefore requires an explicit data migration or a deliberate reset of development data.

Databases containing identities created before the keyed policy are marked `sha256-unkeyed-development-v1 / legacy-unkeyed`. Configure keyed pseudonyms before first migration on a new production database. Existing legacy development data must be intentionally migrated or cleared before enabling a key.

When no pseudonym secret is configured, commands retain the legacy unkeyed development policy and report that limitation. This mode is not suitable for production because Steam IDs can be dictionary-matched.

Pseudonyms reduce routine exposure; they are not anonymisation. Access controls and retention still apply.

## Migration integrity

Migration filenames and SHA-256 checksums are recorded in `schema_migrations`. A previously applied migration whose contents change causes migration failure. Existing migration rows without a checksum are adopted once using the repository version present during the first upgraded run; subsequent changes fail closed.

Never edit an applied migration. Add a new numbered migration.

## Retention

`cmd/retention` independently configures raw, normalized and review durations:

```text
-raw-retention         default 30 days
-normalized-retention  default 90 days
-review-retention      default 365 days
```

The command is a dry run unless `-execute` is supplied. Record operator approval and retain the output with the change record. Aggregate feature/calibration data may remain only when it no longer references a subject and operator policy permits it.

## Player deletion

Run a dry scope report first:

```powershell
go run ./cmd/privacy-delete -raw-dir ./data/raw -player-id '<durable-id>'
```

After approval:

```powershell
go run ./cmd/privacy-delete -raw-dir ./data/raw -player-id '<durable-id>' -actor '<operator>' -reason '<request-id>' -execute
```

Execution removes events where the subject is the event owner, observer, target, combat source or nested player-session reference. It deletes empty raw batches, rewrites mixed batches through a same-directory backup/replace sequence, removes subject-level normalized/review data and records only the keyed durable pseudonym in `privacy_audit_log`. Re-run normalization after deletion if mixed batches were rewritten.

Backups, database snapshots and external log retention must follow the same deletion policy; the command cannot erase copies outside the configured raw root and database.

## Clock-integrity boundary

Each client clock response must match one outstanding server-issued challenge containing the exact sequence and server-send timestamp. Challenges expire after ten seconds and are consumed once, including failed attempts. Clock quality may suppress timing-sensitive evidence but can never strengthen suspicion.

## Review safety

- No endpoint sends analysis to DayZ.
- High-priority output requires breadth, uncertainty, matched-context, stability and negative-control gates.
- Candidate language is behavioural and uncertainty-aware; it is never a statement that a player cheated.
- Review dispositions are audited but are not treated as perfect ground truth.
- The explorer reads normalized pseudonyms, not restricted raw durable identities. Expanded payloads remain sensitive operational telemetry and must not be exposed beyond the admin boundary.
- Exact route coordinates are sensitive telemetry. Coarse `area_cell` observations retain their uncertainty on the map and must not be rendered as precise locations.
