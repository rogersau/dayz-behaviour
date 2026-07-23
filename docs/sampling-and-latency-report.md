# Sampling and latency report

Status date: 2026-07-23

## Implemented design

- Random prospective opportunities use randomized 3–8 second jitter per eligible observer by default.
- Event enrichment is triggered by compact decision edges and is labelled separately.
- Random work has queue priority; both queues, age and work per tick are bounded.
- Random opportunity contexts are now separated into:
  - `ROBUSTLY_OCCLUDED` hidden-target opportunities for the exposure condition;
  - `NO_RELEVANT_TARGET` neutral opportunities for the primary readiness baseline;
  - `EXPOSED` and `PARTIALLY_EXPOSED` opportunities retained as visible-player positive controls, not the primary baseline.
- When no eligible target is inside the configured radius, the server records a neutral `SAMPLING_OPPORTUNITY` rather than omitting the denominator.
- Random opportunities rejected because the queue is full produce lightweight `SAMPLING_OPPORTUNITY_DROPPED` records with observer context, eligible counts, load state and drop reason.
- Every probed visibility event records eligible counts, target/observer inclusion probabilities, queue-admission probability, risk-set definition/completeness, queue delay and probe timestamps.
- Client/server clock alignment records four timestamps, round-trip delay, offset estimate and uncertainty.
- Primary observations use only random opportunities. Timing-sensitive use is suppressed above the versioned 250 ms queue-delay development limit.
- Strong occlusion additionally requires server-enforced first person, the validated first-person-head policy, a controlled-validation ID and a repeated blocked probe meeting the minimum duration.

## Statistical interpretation

The primary readiness comparison is now:

```text
validated concealed target
    versus
no relevant target inside the configured risk-set radius
```

Visible-player opportunities remain useful for checking that ordinary legitimate responsiveness is captured, but they are not a neutral baseline. Exact matching currently omits target distance because the first neutral-control implementation has no target. Matched-control quality is therefore capped at `0.5` until geometry-matched empty sectors or equivalent target-free distance context is implemented and validated.

## Measurements completed

| Check | Result |
|---|---|
| Dedicated startup/module compile | Pass on 1.29.0.163451 before the neutral-control extension; recompile required |
| Authenticated export | 2 requests, 2 accepted, 0 rejected, 0 storage failures |
| Receiver outage | Async callback returned `error:7`; batch spooled without blocking |
| Spool replay | Batch durably imported and source spool archived |
| Empty-server queue health | 0 pending/dropped probe work; exporter success counter advanced |

## Not yet a performance claim

An empty-server smoke test cannot establish a safe player-count budget. The following must be measured with connected clients at representative population:

- client sample interval and RPC bytes by active/idle state;
- edge-to-server receive and clock-uncertainty distributions;
- queue-delay and probe-duration percentiles;
- script-frame CPU, memory and network deltas versus an uninstrumented control;
- neutral, hidden, visible and dropped opportunity rates by player-count and context band;
- drop/admission rates and effective sample size under load;
- gameplay tolerance approved by the server operator.

The current 250 ms timing limit and default queue/probe budgets are development policy values. They must be replaced by a versioned calibration decision after that run.
