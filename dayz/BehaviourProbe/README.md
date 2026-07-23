# DayZ Behaviour Probe

This directory contains the client/server DayZ mod used by [DayZ Behaviour](../../README.md).

The mod captures bounded game telemetry and sends it to the external Go ingest service. It does not analyze long-term player behaviour, assign review tiers, or perform enforcement inside DayZ.

## Load both sides

Pack and sign this directory using the normal mod process for your server. Load the resulting mod on:

- the dedicated server;
- every connecting client.

The client side captures camera and control-state context. The server side owns identity, lifecycle, combat, authoritative position, visibility geometry, batching, export, and spool recovery. A one-sided deployment is incomplete.

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
  "ingest_token": "same-value-as-DBA_QUERY_TOKEN",
  "configuration_hash": "your-versioned-config-id"
}
```

The endpoint must end with `/`.

`ingest_token` is sent as a query parameter because DayZ `RestContext` does not expose a general arbitrary-header setter. Keep the receiver on loopback or behind an equivalent private sidecar boundary. Do not expose this transport directly to the internet.

## Collection behaviour

The mod captures:

- bounded client camera/control samples and transition intervals;
- authoritative server lifecycle, movement, combat, and position events;
- randomized prospective sampling opportunities;
- neutral no-relevant-target controls;
- optional bounded visibility and event-enrichment probes;
- client/server clock alignment;
- collector, queue, export, and spool health.

Failed asynchronous exports are written to bounded NDJSON spool files for later import by `ingestd`.

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

## Further documentation

- [Deployment](../../docs/deployment.md)
- [Architecture](../../docs/architecture.md)
- [Operations, security, and privacy](../../docs/operations.md)
