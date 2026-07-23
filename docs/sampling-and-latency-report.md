# Sampling and latency report

Status date: 2026-07-23

## Implemented design

- The client runs a lightweight readiness/camera detector at 10 Hz while alive and conscious.
- Routine export remains 2 Hz. A compact 50-sample ring retains approximately five seconds of detector history.
- A meaningful edge exports the unsent high-rate ring across bounded existing RPC batches, then retains 10 Hz burst export for ten seconds.
- Decision edges carry the sampling interval in which the transition occurred rather than pretending the transition happened exactly at one timestamp.
- Validated clock alignment maps that client interval into a server-time lower/upper interval. Missing alignment falls back to a conservative interval ending at server receipt.
- Random prospective opportunities use randomized 3–8 second jitter per eligible observer by default.
- Event enrichment is triggered by compact decision edges and is labelled separately.
- Event-enrichment pairs may run a bounded five-second diagnostic window at a 250 ms interval. Random work keeps queue priority and all work remains subject to queue, age and per-tick limits.
- Random opportunity contexts are separated into:
  - `ROBUSTLY_OCCLUDED` hidden-target opportunities for the exposure condition;
  - `NO_RELEVANT_TARGET` neutral opportunities for the primary readiness baseline;
  - `EXPOSED` and `PARTIALLY_EXPOSED` opportunities retained as visible-player positive controls, not the primary baseline.
- When no eligible target is inside the configured radius, the server records a neutral `SAMPLING_OPPORTUNITY` rather than omitting the denominator.
- Random opportunities rejected because the queue is full produce lightweight `SAMPLING_OPPORTUNITY_DROPPED` records with observer context, eligible counts, load state and drop reason.
- Every probed visibility event records eligible counts, target/observer inclusion probabilities, queue-admission probability, risk-set definition/completeness, queue delay and probe timestamps.
- Client/server clock alignment records four timestamps, round-trip delay, offset estimate and uncertainty. Responses must match one outstanding server-issued challenge and are single use.
- Primary observations use only random opportunities. Timing-sensitive use is suppressed above the versioned 250 ms queue-delay or 500 ms event-uncertainty development limits.
- Strong occlusion additionally requires server-enforced first person, the validated first-person-head policy, a controlled-validation ID and a repeated blocked probe meeting the minimum duration.

## Statistical interpretation

The primary readiness comparison is:

```text
validated concealed target
    versus
no relevant target inside the configured risk-set radius
```

Visible-player opportunities remain useful for checking that ordinary legitimate responsiveness is captured, but they are not a neutral baseline. Exact matching currently omits target distance because the first neutral-control implementation has no target. Matched-control quality is therefore capped at `0.5` until geometry-matched empty sectors or equivalent target-free distance context is implemented and validated.

Decision timing is represented as:

```text
event_time_lower_ms
    <= true transition time <=
event_time_upper_ms
```

A readiness/exposure ordering is definite only when the readiness upper bound is before the exposure lower bound. Overlapping intervals remain ambiguous.

## Pre-exposure status

Pre-exposure output is `EXPERIMENTAL_SUPPORTING_ONLY`. It reports:

- definite pre-exposure count;
- ambiguous timing count;
- lower and upper rate bounds;
- median lower and upper lead-time bounds.

It cannot promote review priority. It remains blocked on controlled pair-window execution, exposure-cause labelling and the first-/third-person visibility matrix.

## Measurements completed

| Check | Result |
|---|---|
| Dedicated startup/module compile | Pass on 1.29.0.163451 before the temporal/neutral-control extensions; recompile required |
| Authenticated export | 2 requests, 2 accepted, 0 rejected, 0 storage failures |
| Receiver outage | Async callback returned `error:7`; batch spooled without blocking |
| Spool replay | Batch durably imported and source spool archived |
| Empty-server queue health | 0 pending/dropped probe work; exporter success counter advanced |

## Not yet a performance claim

An empty-server smoke test cannot establish a safe player-count budget. The following must be measured with connected clients at representative population:

- detector CPU cost and RPC bytes by baseline, ring-export and burst state;
- edge interval width, edge-to-server receive and clock-uncertainty distributions;
- queue-delay and probe-duration percentiles;
- diagnostic-window admission, completion and starvation rates;
- script-frame CPU, memory and network deltas versus an uninstrumented control;
- neutral, hidden, visible and dropped opportunity rates by player-count and context band;
- drop/admission rates and effective sample size under load;
- gameplay tolerance approved by the server operator.

The current 250 ms queue limit, 500 ms event-uncertainty limit and default queue/probe budgets are development policy values. They must be replaced by a versioned calibration decision after that run.
