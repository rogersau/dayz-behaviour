# Sampling and latency report

Status date: 2026-07-19

## Implemented design

- Random prospective opportunities use randomized 3–8 second jitter per eligible observer by default.
- Event enrichment is triggered by compact decision edges and is labelled separately.
- Random work has queue priority; both queues, age and work per tick are bounded.
- Every visibility event records eligible counts, target/observer inclusion probabilities, queue-admission probability, risk-set definition/completeness, queue delay and probe timestamps.
- Client/server clock alignment records four timestamps, round-trip delay, offset estimate and uncertainty.
- Primary observations use only random opportunities. Timing-sensitive use is suppressed above the versioned 250 ms queue-delay development limit.
- Strong occlusion additionally requires a controlled-validation ID and a repeated blocked probe meeting the minimum duration.

## Measurements completed

| Check | Result |
|---|---|
| Dedicated startup/module compile | Pass on 1.29.0.163451 |
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
- drop/admission rates and effective sample size under load;
- gameplay tolerance approved by the server operator.

The current 250 ms timing limit and default queue/probe budgets are development policy values. They must be replaced by a versioned calibration decision after that run.
