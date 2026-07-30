# Releases and published images

DayZ Behaviour publishes a standalone Windows agent and central Docker images from version tags such as `v0.1.0`.

Use a pinned version for an operational deployment. The `latest` tag is convenient for evaluation but makes rollback and incident review harder.

## Release contents

A release contains:

- `dayz-behaviour-agent_<version>_windows_amd64.exe` — standalone server agent;
- `dayz-behaviour-agent_<version>_windows_amd64.zip` — agent, setup guide, and licence notices;
- `dayz-behaviour-agent_<version>_windows_amd64_SHA256SUMS.txt` — download checksums;
- `ghcr.io/rogersau/dayz-behaviour:<version>` — central Go runtime;
- `ghcr.io/rogersau/dayz-behaviour-review:<version>` — review runtime with supported map assets.

The central image contains `ingestd`, `normalize`, `analyse`, `reviewd`, `retention`, `privacy-delete`, and `replay`. Compose selects the required executable with each service's entrypoint.

## Platforms

| Artifact | Platforms |
|---|---|
| Windows agent | Windows AMD64 |
| Central runtime image | Linux AMD64 and ARM64 |
| Review image | Linux AMD64 |

The review image is AMD64-only because the pinned map-asset source image is AMD64-only.

## Version tags

For a stable release such as `v0.1.3`, the workflow publishes:

```text
ghcr.io/rogersau/dayz-behaviour:0.1.3
ghcr.io/rogersau/dayz-behaviour:0.1
ghcr.io/rogersau/dayz-behaviour:latest

ghcr.io/rogersau/dayz-behaviour-review:0.1.3
ghcr.io/rogersau/dayz-behaviour-review:0.1
ghcr.io/rogersau/dayz-behaviour-review:latest
```

A prerelease such as `v0.2.0-rc.1` receives only its exact tag. It does not replace `latest` or the stable minor tag.

## Download and verify the Windows agent

Download the ZIP and checksum file from the matching GitHub Release. GitHub CLI can do this directly:

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

The result should match the release version without the leading `v`.

See [DayZ server agent](server-agent.md) for configuration and Windows service installation.

## Deploy central services from published images

The supplied Compose files support either local source builds or published images. Set both image variables to the same pinned version:

```powershell
$env:DBA_CORE_IMAGE = "ghcr.io/rogersau/dayz-behaviour:0.1.0"
$env:DBA_REVIEW_IMAGE = "ghcr.io/rogersau/dayz-behaviour-review:0.1.0"
```

Pull the release images:

```powershell
docker compose `
  --env-file .env `
  -f deploy/compose.yaml `
  -f deploy/compose.release.yaml `
  pull
```

Start them without falling back to a local build:

```powershell
docker compose `
  --env-file .env `
  -f deploy/compose.yaml `
  -f deploy/compose.release.yaml `
  up -d --no-build postgres ingestd normalize reviewd
```

For the home-hosted Cloudflare Tunnel topology, include both overrides:

```powershell
docker compose `
  --env-file .env `
  -f deploy/compose.yaml `
  -f deploy/compose.release.yaml `
  -f deploy/compose.home.yaml `
  up -d --no-build postgres ingestd normalize reviewd cloudflared
```

If the GHCR packages are private, authenticate before pulling:

```powershell
$env:GHCR_TOKEN | docker login ghcr.io -u <github-user> --password-stdin
```

The token needs package read permission. Public packages can be pulled anonymously.

## Upgrade order

For the initial protocol, deploy the same release version across the central services and agents.

Recommended order:

1. back up raw telemetry, PostgreSQL, deployment configuration, and server credential maps;
2. pull the new pinned central and review images;
3. stop or pause scheduled analysis work;
4. update `ingestd`, `normalize`, and `reviewd` together;
5. verify migrations, ingest, normalization, login, timelines, and maps;
6. update agents one server at a time;
7. confirm each agent's queue drains and its reported version is correct;
8. resume analysis and compare health counters with the pre-upgrade baseline.

Agents queue locally during a brief central restart, so the DayZ process does not need to wait for central availability.

## Rollback

Do not roll back across a destructive database migration without a tested restoration plan.

For a compatible rollback:

1. set `DBA_CORE_IMAGE` and `DBA_REVIEW_IMAGE` back to the previous exact version;
2. run Compose with `deploy/compose.release.yaml` and `--no-build`;
3. roll agents back to the matching previous executable if the release changed the transport contract;
4. verify queued batches drain without permanent rejections;
5. document the reason, affected period, image digests, and restored version.

Keep the previous agent ZIP and exact image tags until the new release has completed operational validation.

## Publishing a release

The `Release Agent and Docker Images` GitHub workflow runs when a SemVer tag is pushed:

```powershell
git tag v0.1.0
git push origin v0.1.0
```

Before tagging, ensure the release commit is already merged into `main`. The workflow checks out the tag, runs the race-enabled test suite and `go vet`, builds the Windows agent, publishes both GHCR images with provenance and SBOM data, and then creates the GitHub Release.

A manual workflow run can republish an existing tag. It replaces downloadable assets and republishes the versioned images without creating a second GitHub Release.
