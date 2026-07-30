# Documentation

Start with the root [README](../README.md). It explains the problem, safety model, components, and quickest way to run the project.

The remaining guides are organized by what a new contributor or operator needs to understand:

1. [Architecture](architecture.md) — components, data flow, trust boundaries, sampling, storage, and resilience.
2. [Analysis and review](analysis-and-review.md) — how observations and controls are constructed, how review tiers are calculated, and what results do and do not mean.
3. [Deployment](deployment.md) — local stack, environment variables, DayZ mod setup, networking, and initial validation.
4. [DayZ server agent](server-agent.md) — standalone Windows agent, durable forwarding, home-hosted central services, and Cloudflare Tunnel routing.
5. [Operations, security, and privacy](operations.md) — monitoring, spool recovery, backups, migrations, administrator access, retention, and player deletion.

## Reading paths

### I want to understand the project

Read the root README, then Architecture and Analysis and review.

### I want to run it

Read Deployment, then the first-run and validation sections in Operations.

### I need to review a player

Read Analysis and review before using the admin explorer. A review tier is an evidence-prioritisation result, not a finding that the player cheated.

### I need to operate it in production

Read all four guides. Pay particular attention to the network boundary, pseudonym-key policy, retention, deletion, backups, and deployment-specific visibility validation.