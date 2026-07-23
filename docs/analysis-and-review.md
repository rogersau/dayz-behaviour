# Analysis and review

## What the analysis is trying to measure

The analysis asks a narrow question:

> Does this player repeatedly enter a combat-ready state more often when another player is validated as concealed than they do during comparable neutral situations?

It does not estimate the probability that a player is cheating. It produces evidence summaries and review-priority tiers so administrators can decide which sessions deserve manual inspection.

A plausible legitimate explanation may exist for every highlighted incident. Examples include sound, team communication, prior visual contact, map knowledge, prediction, or information that the collector did not capture.

## From events to observations

Raw telemetry is not counted directly as evidence. The observation builder creates versioned decision windows from random prospective sampling opportunities.

Each observation records:

- the observer and, where applicable, target session;
- server and server-session identity;
- map, coarse area, movement, stance, weapon state, and population context;
- sampling stream, inclusion probability, admission probability, and queue delay;
- visibility class and authority;
- captured cues before the opportunity;
- whether a qualifying readiness transition occurred inside the decision window;
- lower and upper timing bounds;
- independence, timing, hidden, neutral-control, and positive-control eligibility;
- every source event used to construct the observation;
- builder and policy versions.

Observations close together are grouped into refractory windows, observer/target episodes, and encounters. This prevents a burst of highly correlated samples from being treated as many independent incidents.

## Conditions and controls

### Hidden condition

A primary hidden observation requires `ROBUSTLY_OCCLUDED` visibility. This classification is accepted only when the deployment has a validated, server-enforced first-person policy and the probe meets its duration and timing requirements.

### Neutral control

The primary control is a random opportunity where there is **no relevant target inside the configured risk-set radius**.

This measures the observer’s ordinary tendency to raise or aim a weapon in similar server, map, movement, stance, population, camera-policy, and sampling contexts.

### Visible positive control

Exposed or partially exposed players are retained as a positive control. They help confirm that the collector captures normal legitimate reactions, but they are not the neutral baseline for hidden-awareness lift.

## Readiness outcome

The primary outcome is a client transition such as:

- weapon raised;
- ADS entered;
- optics entered.

The client samples state at intervals. The event therefore has a lower and upper time bound. An outcome is eligible only when the event interval overlaps the versioned decision window and its uncertainty is within policy limits.

The transition remains Tier B client context even after clock alignment. The hidden or neutral condition is derived from Tier A server context.

## Cue ledger

Before calling an opportunity unexplained, the builder searches captured prior events for plausible cues.

Examples include:

- recent exposed or partially exposed visibility;
- recent combat contact attributed to the target;
- other captured context that a policy version marks as known or plausible.

The resulting cue classes are:

- `KNOWN` — captured data contains a direct explanation;
- `PLAUSIBLE` — captured data contains a reasonable indirect explanation;
- `UNEXPLAINED_IN_CAPTURED_DATA` — the collector did not record an explanation.

“Unexplained” does not mean impossible or illegitimate. It means only that the retained telemetry did not capture the explanation.

Primary readiness features use independent observations in the unexplained class. Known and plausible observations remain available in the timeline for review.

## Primary feature: hidden-threat readiness

For each durable player identity, the analyzer counts:

- hidden successes and hidden trials;
- neutral-control successes and control trials;
- independent sessions, encounters, and durable target identities.

A beta-binomial estimate shrinks sparse rates towards a prior instead of treating a few successes as a stable player characteristic.

The main reported quantity is:

```text
readiness lift = hidden posterior rate - neutral-control posterior rate
```

The ranking policy uses the lower confidence bound, not just the point estimate.

## Matched model

The analyzer also creates exact-context strata within a player. A stratum must contain both hidden and neutral-control observations.

Current matching context includes available variables such as:

- server and map;
- coarse area cell;
- movement band and stance;
- baseline weapon state;
- camera/server policy;
- server population band;
- cue class;
- sampling-policy version.

A one-coefficient conditional logistic model estimates the hidden-versus-neutral odds ratio while conditioning out the stratum intercept.

Sparse, non-converged, or separated models are reported as limitations and cannot satisfy the high-priority matched-model gate.

## Stability and negative controls

### Leave-one-session-out stability

The primary estimate is recalculated while excluding each session in turn. A high-priority result requires the effect direction to remain stable rather than being driven by one unusual session.

### Negative controls

Preregistered negative-control checks are run against the matched data. High-priority output is suppressed when negative controls are incomplete or fail.

Negative controls are safety checks for the analysis pipeline, not evidence against a player.

## Supporting feature families

### Concealed-sector selection

Event-enrichment windows can compare camera-turn direction with the bearing to a concealed target. A circular permutation test measures whether concealed sectors are selected more often than expected under shifted target bearings.

This feature is supporting context. It does not bypass the primary readiness breadth and validation gates.

### Pre-exposure readiness

The analyzer compares a readiness interval with the first later exposed/partially exposed probe interval for the same target.

An ordering is definite only when:

```text
readiness_upper_ms < exposure_lower_ms
```

Overlapping intervals are counted as ambiguous. Censored windows remain censored.

Pre-exposure output is marked `EXPERIMENTAL_SUPPORTING_ONLY`. It cannot promote the review tier.

## Review tiers

The current transparent ranking policy produces four tiers:

| Tier | Meaning |
|---|---|
| `INSUFFICIENT_DATA` | Evidence breadth requirements were not met |
| `MONITOR` | Breadth requirements passed, but the review-effect gate did not |
| `REVIEW` | The readiness-lift lower bound passed the review gate |
| `HIGH_PRIORITY_REVIEW` | Stronger lift, matched-model, stability, and negative-control gates passed |

Default breadth gates are:

- at least 20 eligible hidden opportunities;
- at least 3 independent sessions;
- at least 5 independent encounters;
- at least 5 independent durable target identities.

Default lift gates are:

- review when the readiness-lift lower bound is at least `0.05`;
- possible high-priority review when it is at least `0.15` and all additional gates pass.

These values are policy, not universal scientific constants. A deployment should calibrate them using trusted cohorts and documented review outcomes before treating the queue as operationally useful.

## What an administrator should review

A reviewer should not look only at the tier or the strongest incident. Review the full evidence package:

1. **Data quality** — hidden, neutral, visible-positive, and dropped-opportunity counts; clock uncertainty; collector losses.
2. **Breadth** — sessions, encounters, targets, and whether one period dominates.
3. **Effect uncertainty** — point estimates, lower bounds, matched-model convergence, and stability diagnostics.
4. **Cue history** — prior exposure, combat, nearby activity, and uncaptured-information limitations.
5. **Timeline** — authoritative positions and combat facts, client transitions, visibility probes, and source authority.
6. **Spatial context** — route, related players, camera context, and coarse visibility cells.
7. **Alternative explanations** — audio, squad communication, prediction, map knowledge, stream sniping, or telemetry gaps.
8. **Conventional evidence** — server logs, existing anti-cheat evidence, player reports, and video where available.

The review result should describe observable behaviour and uncertainty. It should not convert the model output into a cheat probability.

## Admin explorer

The browser explorer lists pseudonymized sessions and reconstructs ordered timelines from normalized PostgreSQL data.

Spatial evidence is rendered with different provenance:

- exact server positions for the selected player’s route;
- related-player server positions separately;
- untrusted client camera positions separately;
- visibility `area_cell` as a coarse 100 m cell rather than a precise point.

Captured `map_id` takes precedence. If the required map is unavailable, the explorer fails closed instead of plotting coordinates on a different terrain.

Timeline requests are capped at 2,000 entries. Larger sessions should be divided with `from_ms` and `to_ms` while preserving ordering context.

## Review API concepts

The review service stores:

- versioned candidate rankings;
- review cases for eligible tiers;
- audited reviewer dispositions;
- evidence component values and source incident IDs.

A reviewer disposition is operational feedback, not perfect ground truth. It should not be fed back into model calibration without a documented labeling and quality-control process.

## Interpreting missing or weak output

No review candidate may mean:

- visibility probing is disabled;
- first-person validation is not configured;
- no neutral controls were collected;
- timing or queue delays exceeded policy;
- the player did not meet breadth gates;
- the matched model did not converge;
- stability or negative controls failed;
- the selected raw dataset is too small;
- retention or privacy deletion removed required history.

These outcomes are preferable to manufacturing a confident result from incomplete evidence.

## Non-goals

The analysis does not:

- detect or name cheat software;
- prove that an individual incident was impossible legitimately;
- recreate audio or every rendered visibility condition;
- infer communications outside the collected game state;
- replace human investigation;
- trigger a ban, kick, warning, or other gameplay action.
