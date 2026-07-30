# DayZ server agent

The DayZ Behaviour server agent is a single executable installed beside a DayZ dedicated server. It receives telemetry from the DayZ mod over loopback, stores every accepted batch in a durable local outbox, and forwards the original batch to the central ingest service over HTTPS.

The agent does not require Docker, Go, PostgreSQL, or another runtime on the DayZ server.

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

The mod and agent `server_id` values must match.

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

Relative paths are resolved from the configuration-file directory. The listener is required to remain on loopback. Remote upstream URLs are required to use HTTPS. Restrict access to the agent folder because its configuration contains credentials and its outbox contains directly attributable telemetry.

The local credential and upstream credential may be supplied at runtime with `DBA_AGENT_LOCAL_TOKEN` and `DBA_AGENT_UPSTREAM_TOKEN`. Environment values override the JSON fields.

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

## Configure central server authentication

Central `ingestd` can load a JSON map that binds one credential to one DayZ `server_id`:

```powershell
.\dayz-behaviour-agent.exe generate-credential
```

Generate a separate value for each server. Put the value in the central map and provide the same value to that server operator for the interactive `init` prompt.

```json
{
  "example-chernarus-1": "[REDACTED_SECRET]",
  "example-namalsk-1": "[REDACTED_SECRET]"
}
```

Set `DBA_SERVER_AUTH_FILE` to the mounted path of this file. The agent sends its claimed server ID in `X-DayZ-Server-ID`; central ingest rejects a credential used for a different `server_id`. `ingestd` reads the map at startup, so restart that service after adding, rotating, or revoking a server credential; agents safely queue during the restart.

Existing single-token local deployments remain supported through `DBA_QUERY_TOKEN` or `DBA_BEARER_TOKEN`.

## Run the central stack at home with Cloudflare Tunnel

Copy `deploy/server-auth.example.json` to a protected path outside the repository and replace the placeholders. Set:

```text
DBA_SERVER_AUTH_HOST_FILE=<absolute host path to server-auth.json>
DBA_CLOUDFLARE_TUNNEL_TOKEN=<Cloudflare tunnel token>
```

Start the home stack with the base Compose file and the home-hosted override:

```powershell
docker compose `
  --env-file .env `
  -f deploy/compose.yaml `
  -f deploy/compose.home.yaml `
  up --build postgres ingestd normalize reviewd cloudflared
```

In the Cloudflare Tunnel configuration, publish:

```text
ingest.example.com + path ^/v1/telemetry/batches$ → http://ingestd:8080
review.example.com                              → http://reviewd:8082
catch-all                                      → HTTP 404
```

Apply Cloudflare Access to the review hostname. The ingest route is authenticated by the server-specific application credentials. Its path rule prevents the public hostname from exposing `healthz`, `readyz`, or metrics.

No inbound home-router port forwarding is required. `ingestd`, PostgreSQL, and `reviewd` remain bound to loopback on the Docker host; `cloudflared` reaches them through the private Compose network.

## Build the Windows executable

From a machine with the Go version declared in `go.mod`:

```powershell
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags="-s -w" -o dayz-behaviour-agent.exe ./cmd/agentd
```

The output is a standalone Windows executable.

## Publish a GitHub Release

The `Release Windows Agent` workflow publishes releases from SemVer tags. Create and push a tag after the release commit is on the intended branch:

```powershell
git tag v0.1.0
git push origin v0.1.0
```

The workflow:

- runs the full Go test suite with the race detector and runs `go vet`;
- embeds `0.1.0` from the tag into the agent's `version` command;
- builds the standalone Windows AMD64 executable;
- publishes the executable, a ZIP package containing the documentation and notices, and SHA-256 checksums;
- publishes signed-provenance and SBOM-enabled container images to GitHub Container Registry;
- creates generated GitHub release notes;
- marks tags such as `v0.2.0-rc.1` as prereleases.

Published images for `v0.1.0` are:

```text
ghcr.io/rogersau/dayz-behaviour:0.1.0
ghcr.io/rogersau/dayz-behaviour-review:0.1.0
```

The core image contains `ingestd`, `normalize`, `analyse`, `retention`, `privacy-delete`, `replay`, and `reviewd`, and is published for Linux AMD64 and ARM64. The review image adds the bundled supported map assets and is Linux AMD64 because its pinned map source is AMD64-only.

Stable releases also update the minor tag, such as `0.1`, and `latest`. Prereleases publish only their exact version, such as `0.2.0-rc.1`, and do not replace `latest`.

After the first publication, check each package's GitHub visibility setting. Public GHCR packages can be pulled anonymously; private packages require registry authentication.

The workflow may also be run manually for an existing tag. Rerunning it replaces the downloadable release assets and republishes the versioned container images without creating a duplicate GitHub Release.
