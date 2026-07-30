# DayZ Behaviour Probe

This directory contains the client/server DayZ mod used by [DayZ Behaviour](../../README.md).

The mod captures bounded game telemetry and sends it to the local Go server agent or a same-host ingest service. It does not analyze long-term player behaviour, assign review tiers, record raw audio, or perform enforcement inside DayZ.

## Load both sides

Pack and sign this directory using the normal mod process for your server. Load the resulting mod on:

- the dedicated server;
- every connecting client.

The client side captures camera and control-state context. The server side owns direct identity, lifecycle, combat, authoritative position, gunshot and movement-audio opportunities, visibility geometry, batching, export, and spool recovery. A one-sided deployment is incomplete.

The mod uses project-owned RPC IDs `759430` through `759436`. Confirm they do not conflict with another loaded mod.

## Server configuration

On first launch the mod creates:

```text
$profile:DayZBehaviourProbe/config.json
```

Set at least:

```json
{
  "enabled": true,
  "server_id": "your-server-name",
  "endpoint": "http://127.0.0.1:8080/",
  "ingest_token": [REDACTED_SECRET],
  "configuration_hash": "your-versioned-config-id"
}
```

The endpoint must end with `/`.

`ingest_token` is sent as a query parameter because DayZ `RestContext` does not expose a general arbitrary-header setter. Keep this connection on loopback between the DayZ server and the standalone agent, or between DayZ and same-host `ingestd` for evaluation. Do not expose the query-token transport directly to the internet.

## Collection behaviour

The mod captures:

- bounded client camera/control samples and transition intervals;
- authoritative server lifecycle, movement, combat, and position events;
- successful server-side shot opportunities through `Weapon_Base.EEFired`;
- suppressor, weapon, ammunition, shooter, time, and position context;
- bounded movement-audio opportunities containing speed, gait, stance, surface, footwear, and position;
- randomized prospective sampling opportunities;
- neutral no-relevant-target controls;
- optional bounded visibility and event-enrichment probes;
- client/server clock alignment;
- collector, queue, export, and spool health.

Failed asynchronous exports are written to bounded NDJSON spool files. In a distributed deployment the local agent imports them into its durable outbox; same-host evaluation deployments may import them through `ingestd`.

## Audio opportunity settings

Audio opportunity capture is enabled by default:

```json
{
  "enable_audio_cues": true,
  "audio_context_interval_seconds": 2.0,
  "audio_min_movement_speed_mps": 0.25
}
```

The server does not decide whether a player heard a cue. It only exports the authoritative facts needed by Go analysis to estimate audibility.

The initial implementation relies on Enfusion/DayZ script surfaces used by the base game:

- `Weapon_Base.EEFired(int muzzleType, int mode, string ammoType)` for successful fire events;
- `Weapon_Base.GetAttachedSuppressor()` for suppressor context;
- `GetVelocity(player)` and `HumanMovementState` for movement context;
- `GetGame().SurfaceGetType3D(...)` for surface type;
- `FindAttachmentBySlotName("Feet")` for footwear type.

These callbacks and values must be verified on the exact dedicated-server build and loaded mod set. Another mod that replaces the same callback without chaining `super` can prevent collection.

The movement interval is bounded server work. Lowering it increases event volume for every moving player. Do not change it without measuring script time, export size, queue loss, and analysis value at representative population.

## Direct identities

The server uses `PlayerIdentity.GetId()` and includes the durable player ID in player-session identifiers. Normalization and review preserve those values so administrators can cross-reference Steam and external moderation systems.

This means raw, normalized, and review data is directly attributable. Restrict access to the data plane, explorer, API, logs, exports, and backups.

## Visibility safety

Visibility probing is disabled by default.

Strong concealed-target evidence may be enabled only when:

- the server actually enforces first person;
- a controlled visibility confusion matrix has passed on the exact build/mod set;
- `server_first_person_only` is true;
- `visibility_origin_mode` is `VALIDATED_FIRST_PERSON_HEAD`;
- `visibility_validation_id` records the approved fixture;
- queue, timing, and repeated-occlusion requirements are met.

Third-person or unknown-perspective head-origin rays remain descriptive context and must not become strong hidden evidence.

## Runtime checks

Before using the new audio cues:

1. Verify the scripts compile and load on the target DayZ build.
2. Fire suppressed and unsuppressed weapons and confirm exactly one `SHOT_FIRED_SERVER` event per expected server callback.
3. Test automatic/burst, multi-muzzle, modded weapons, reconnects, and weapon attachments.
4. Walk, jog, sprint, crouch, and prone over representative surfaces and verify movement events.
5. Confirm footwear and surface values are sensible.
6. Measure event and byte volume at representative player count.
7. Compare the external audibility model with controlled listening tests.

## Further documentation

- [DayZ server agent](../../docs/server-agent.md)
- [Releases and published images](../../docs/releases.md)
- [Deployment](../../docs/deployment.md)
- [Architecture](../../docs/architecture.md)
- [Analysis and review](../../docs/analysis-and-review.md)
- [Operations, security, and privacy](../../docs/operations.md)
