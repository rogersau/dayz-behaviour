# DayZ server agent

The DayZ Behaviour server agent is a single executable installed beside a DayZ dedicated server. It receives telemetry from the DayZ mod over loopback, stores every accepted batch in a durable local outbox, and forwards the original batch to central `ingestd` over HTTPS.

The agent does not require Docker, Go, PostgreSQL, or another runtime on the DayZ server.

Use the same release version for the agent and central images for the initial protocol. See [Releases and published images](releases.md) for downloads, checksums, image tags, upgrades, and rollback.

## Home-hosted topology

```text
DayZ dedicated server
    │
    │ HTTP on 127.0.0.1 only
    ▼
dayz-behaviour-agent.exe
    ├─ validates the local DayZ credential
    ├─ validates server_id and schema
    ├─ durably queues the batch on disk
    └─ retries HTTPS delivery
            │
            ▼
      Cloudflare public hostname
            │
            ▼
Cloudflare Tunnel running at home
            │
            ▼
central ingestd → raw storage → normalize/analyse → PostgreSQL/reviewd
```

Cloudflare Tunnel is only the route into the home network. The Go services, raw telemetry, PostgreSQL, analysis, and review interface remain on the home server.

## Download and verify

Download the Windows AMD64 ZIP and checksum file from the matching GitHub Release. GitHub CLI can fetch both:

```powershell
gh release download v0.1.0 `
  --repo rogersau/dayz-behaviour `
  --pattern "dayz-behaviour-agent_0.1.0_windows_amd64.zip" `
  --pattern "dayz-behaviour-agent_0.1.0_windows_amd64_SHA256SUMS.txt"
```

Verify the ZIP before extracting it:

```powershell
$expected = (Get-Content .\dayz-behaviour-agent_0.1.0_windows_amd64_SHA256SUMS.txt |
  Where-Object { $_ -match "\.zip$" }).Split()[0]
$actual = (Get-FileHash .\dayz-behaviour-agent_0.1.0_windows_amd64.zip -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actual -ne $expected.ToLowerInvariant()) { throw "Checksum mismatch" }
```

After extraction:

```powershell
.\dayz-behaviour-agent.exe version
```

The output should match the release version without the leading `v`.

## Agent commands

```powershell
.\dayz-behaviour-agent.exe init `
  -server-id example-chernarus-1 `
  -upstream-url https://ingest.example.com `
  -dayz-spool-dir "C:\DayZServer\profiles\DayZBehaviourProbe\spool"

.\dayz-behaviour-agent.exe install
.\dayz-behaviour-agent.exe start
.\dayz-behaviour-agent.exe status
.\dayz-behaviour-agent.exe stop
.\dayz-behaviour-agent.exe uninstall
```

Run `init` in an interactive terminal. It securely prompts for the server-specific credential issued by the central administrator and creates `dayz-behaviour-agent.json` beside the executable. For unattended setup, provide the credential through `DBA_AGENT_UPSTREAM_TOKEN` for that command only.

`install`, `start`, `stop`, and `uninstall` require an elevated Windows terminal because they manage a Windows service.

## Configure the DayZ mod

The `init` command creates a separate random credential for the loopback DayZ-to-agent connection and prints it once. Put the printed value into the DayZ mod configuration:

```json
{
  "enabled": true,
  "server_id": "example-chernarus-1",
  "endpoint": "http://127.0.0.1:8080/",
  "ingest_token": "[REDACTED_SECRET]",
  "configuration_hash": "example-chernarus-1-v1"
}
```

The mod, agent, and central credential-map `server_id` values must match.

## Agent configuration

A generated configuration contains:

```json
{
  "listen_addr": "127.0.0.1:8080",
  "server_id": "example-chernarus-1",
  "local_ingest_token": "[REDACTED_SECRET]",
  "upstream_url": "https://ingest.example.com",
  "upstream_bearer_token": "[REDACTED_SECRET]",
  "data_dir": "./data",
  "dayz_spool_dir": "C:\\DayZServer\\profiles\\DayZBehaviourProbe\\spool",
  "max_queue_bytes": 5368709120,
  "max_queue_batches": 250000,
  "upload_interval_seconds": 1,
  "request_timeout_seconds": 15,
  "minimum_retry_seconds": 1,
  "maximum_retry_seconds": 300
}
```

Relative paths are resolved from the configuration-file directory. The listener is required to remain on loopback. Remote upstream URLs are required to use HTTPS.

Restrict access to the agent folder because:

- the configuration contains credentials;
- the outbox and dead-letter directories contain directly attributable telemetry;
- the DayZ spool contains the same raw batches when local delivery fails.

The local and upstream credentials may be supplied at runtime with `DBA_AGENT_LOCAL_TOKEN` and `DBA_AGENT_UPSTREAM_TOKEN`. Environment values override the JSON fields.

## Delivery behaviour

The local receiver returns success only after the batch has been written to the agent outbox. Uploading is asynchronous.

- HTTP 2xx from central ingest removes the local queued copy.
- Network failures and HTTP 5xx responses remain queued and use bounded exponential retry.
- Invalid, conflicting, or oversized batches rejected with permanent HTTP 4xx responses move to the local `dead-letter` directory.
- Duplicate delivery is expected and safe; central ingest remains idempotent.
- The outbox is bounded by bytes and batch count. When full, the agent rejects new batches so the DayZ mod can use its own bounded emergency spool.

Local operational endpoints are available only through the loopback listener:

```text
GET http://127.0.0.1:8080/healthz
GET http://127.0.0.1:8080/readyz
GET http://127.0.0.1:8080/agent/status
GET http://127.0.0.1:8080/agent/metrics
```

Routine checks should confirm:

- `version` matches the intended release;
- service state is `running`;
- status is `ok`, or any degraded reason is understood;
- queue batches and bytes return towards zero after an outage;
- `last_success_at` advances;
- upload failures are not continuously increasing;
- dead letters remain zero unless a rejection is under investigation.

## Configure central server authentication

Central `ingestd` loads a JSON map that binds one credential to one DayZ `server_id`.

Generate a separate credential for each server:

```powershell
.\dayz-behaviour-agent.exe generate-credential
```

Put the value in the central map and provide the same value to that server operator for the interactive `init` prompt:

```json
{
  "example-chernarus-1": "[REDACTED_SECRET]",
  "example-namalsk-1": "[REDACTED_SECRET]"
}
```

Set `DBA_SERVER_AUTH_FILE` to the mounted container path. The agent sends its claimed server ID in `X-DayZ-Server-ID`; central ingest rejects a credential used for a different `server_id`.

`ingestd` reads the map at startup. Restart it after adding, rotating, or revoking a server credential. Agents safely queue during the restart.

Existing single-token same-host deployments remain supported through `DBA_QUERY_TOKEN` or `DBA_BEARER_TOKEN`. Do not use the DayZ query token across the internet.

## Run the central stack at home

Copy `deploy/server-auth.example.json` to a protected path outside the repository and replace the placeholders. Configure:

```text
DBA_SERVER_AUTH_HOST_FILE=<absolute host path to server-auth.json>
DBA_CLOUDFLARE_TUNNEL_TOKEN=<Cloudflare tunnel token>
DBA_CORE_IMAGE=ghcr.io/rogersau/dayz-behaviour:0.1.0
DBA_REVIEW_IMAGE=ghcr.io/rogersau/dayz-behaviour-review:0.1.0
```

Pull and start the pinned release images:

```powershell
docker compose `
  --env-file .env `
  -f deploy/compose.yaml `
  -f deploy/compose.release.yaml `
  -f deploy/compose.home.yaml `
  pull

docker compose `
  --env-file .env `
  -f deploy/compose.yaml `
  -f deploy/compose.release.yaml `
  -f deploy/compose.home.yaml `
  up -d --no-build postgres ingestd normalize reviewd cloudflared
```

In the Cloudflare Tunnel configuration, publish only:

```text
ingest.example.com + path ^/v1/telemetry/batches$ → http://ingestd:8080
review.example.com                              → http://reviewd:8082
catch-all                                      → HTTP 404
```

Apply Cloudflare Access or an equivalent control to the review hostname. The ingest route is authenticated by the server-specific application credentials. Its path rule prevents the public hostname from exposing `healthz`, `readyz`, or metrics.

No inbound home-router port forwarding is required. `ingestd`, PostgreSQL, and `reviewd` remain bound to loopback on the Docker host; `cloudflared` reaches them through the private Compose network.

## Upgrade and rollback

Update central services before agents. Agents queue during a brief central restart.

For each upgrade:

1. back up central raw telemetry, PostgreSQL, configurations, and the server credential map;
2. pin and deploy the new central and review images;
3. verify ingest, normalization, login, timelines, and maps;
4. update one agent and confirm its queue drains;
5. continue server by server;
6. retain the previous ZIP and image tags until validation is complete.

Do not roll back across a destructive database migration without restoring a compatible backup. See [Releases and published images](releases.md) for the full procedure.

## Build from source

For development, build the Windows executable with the Go version declared in `go.mod`:

```powershell
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags="-s -w" -o dayz-behaviour-agent.exe ./cmd/agentd
```

An ordinary source build reports version `dev`; release builds embed the SemVer tag.

## Publish a release

The `Release Agent and Docker Images` workflow publishes the Windows artifacts and GHCR images from a SemVer tag. The release commit must already be merged into `main` before the tag is pushed.

See [Releases and published images](releases.md) for the tag command, asset list, container tags, prerelease behaviour, and manual republishing.
