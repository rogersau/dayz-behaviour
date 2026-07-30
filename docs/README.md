# Documentation

Start with the root [README](../README.md). It explains the problem, safety model, components, and quickest way to run the project.

The remaining guides are organized by what a new contributor or operator needs to understand:

1. [Architecture](architecture.md) — components, data flow, trust boundaries, sampling, storage, and resilience.
2. [Analysis and review](analysis-and-review.md) — how observations and controls are constructed, how review tiers are calculated, and what results do and do not mean.
3. [Deployment](deployment.md) — central stack, environment variables, DayZ mod setup, networking, and initial validation.
4. [DayZ server agent](server-agent.md) — standalone Windows agent, durable forwarding, home-hosted central services, and Cloudflare Tunnel routing.
5. [Releases and published images](releases.md) — release assets, GHCR images, checksums, upgrades, rollback, and release publication.
6. [Operations, security, and privacy](operations.md) — monitoring, agent queues, spool recovery, backups, migrations, administrator access, retention, and player deletion.

## Reading paths

### I want to understand the project

Read the root README, then Architecture and Analysis and review.

### I want to run it

Read Releases and published images, then Deployment and the first-run and validation sections in Operations.

### I need to review a player

Read Analysis and review before using the admin explorer. A review tier is an evidence-prioritisation result, not a finding that the player cheated.

### I need to operate it in production

Read all six guides. Pay particular attention to pinned versions, the agent and central network boundaries, direct-identity handling, retention, deletion, backups, rollback, and deployment-specific visibility validation.
