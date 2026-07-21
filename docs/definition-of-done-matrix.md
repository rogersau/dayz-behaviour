# Production definition-of-done matrix

Status date: 2026-07-19

| # | Production criterion | Status | Evidence or remaining gate |
|---:|---|---|---|
| 1 | Every MVP DayZ hook verified on target build | Partial | All modules load on 1.29; connected client, lifecycle and combat execution fixtures remain |
| 2 | Bounded collectors and no blocking HTTP | Implemented | Bounded arrays/queues/spool; only async `RestContext.POST` |
| 3 | Identity ignores client-supplied identity | Implemented | RPC sender `PlayerIdentity` is the binding source |
| 4 | Durable raw before acknowledgement | Verified | fsync, atomic placement and live ingestion test |
| 5 | Replay after schema/algorithm change | Verified | deterministic replay and golden fixture |
| 6 | Separate labelled random/enrichment streams | Implemented | separate queues and primary denominator filter |
| 7 | Inclusion/admission/drop metadata | Implemented | schema validation and collector health/drop fields |
| 8 | Strong evidence limited to validated semantics | Implemented, validation pending | fail-closed validation ID/origin/duration gate; confusion matrix not yet passed |
| 9 | Third-person head occlusion cannot masquerade | Implemented | default result is `HEAD_ORIGIN_OCCLUDED`; Go gate requires validated first person |
| 10 | Client telemetry is supporting only | Implemented | Tier B/C and server authority rules |
| 11 | Observation independence | Implemented | episode, encounter, refractory and prospective windows |
| 12 | Matched context, shrinkage and uncertainty | Implemented, calibration pending | conditional logit, separation handling, beta-binomial fallback and bounds |
| 13 | Breadth across sessions/encounters/targets | Implemented | transparent ranking gates and lifecycle session rotation |
| 14 | Negative controls show no material signal | Pending data | controls fail closed; full trajectory/sector controls require calibrated multiplayer replay |
| 15 | Trusted-player false-priority tolerance | Pending data | silent trusted cohort not yet run |
| 16 | Candidate exposes rates/controls/counts/versions | Implemented | persistent candidate and incident API structures |
| 17 | No automated punishment | Verified by architecture | no DayZ-bound analysis path or enforcement endpoint |
| 18 | Representative load meets gameplay tolerance | Pending data | empty-server smoke is not representative |
| 19 | Review yield beats random selection | Pending data | blinded review/calibration campaign required |
| 20 | Retention, access and deletion | Implemented | loopback/auth, independent policies, dry-run tools and audited deletion |

Code completion does not waive evidence criteria. The production release remains blocked while any row marked `Pending data` or `Partial` remains.
