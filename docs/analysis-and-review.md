# Analysis and review

## What the analysis measures

The analysis asks:

> Does this player repeatedly enter a combat-ready state more often when another player is validated as concealed than during comparable neutral situations?

It does not estimate the probability that a player is cheating. It produces evidence summaries and review-priority tiers for manual investigation.

A legitimate explanation may exist for every highlighted incident, including sound, team communication, prior visual contact, map knowledge, prediction, stream sniping, or information the collector did not capture.

## From events to observations

Raw telemetry is not counted directly as evidence. The observation builder creates versioned decision windows from random prospective opportunities.

Each observation records:

- direct observer identity and target session where applicable;
- server and server-session identity;
- map, area, movement, stance, weapon state, and population context;
- sampling probabilities, admission, and queue delay;
- visibility class and authority;
- cue facts captured before the opportunity;
- whether a readiness transition occurred inside the decision window;
- lower and upper timing bounds;
- eligibility and independence flags;
- every source event used;
- builder and policy versions.

Closely spaced observations are grouped into refractory windows, observer/target episodes, and encounters so correlated samples are not treated as independent evidence.

## Conditions and controls

### Hidden condition

A primary hidden observation requires `ROBUSTLY_OCCLUDED` visibility from a validated, server-enforced first-person policy with acceptable duration and timing.

### Neutral control

The primary control is a random opportunity where no relevant target exists inside the configured risk-set radius.

This measures the observer’s ordinary tendency to raise or aim a weapon in similar server, map, movement, stance, population, camera-policy, and sampling contexts.

### Visible positive control

Exposed and partially exposed targets are retained as a positive control to confirm that normal legitimate responsiveness is captured. They are not the neutral baseline.

## Readiness outcome

The primary client transitions are:

- weapon raised;
- ADS entered;
- optics entered.

Because client state is sampled, each transition has lower and upper time bounds. It is eligible only when its interval overlaps the decision window and uncertainty remains within policy limits.

The transition remains Tier B client context. The hidden or neutral condition is Tier A server context.

## Cue ledger

Before calling an opportunity unexplained, analysis searches prior captured events for plausible explanations:

- recent exposed or partially exposed visibility;
- recent combat contact attributed to the target;
- server-derived gunshot opportunities;
- server-derived movement and footstep opportunities.

Cue classes are:

- `KNOWN` — captured data contains a direct strong explanation;
- `PLAUSIBLE` — captured data contains a reasonable indirect explanation;
- `UNEXPLAINED_IN_CAPTURED_DATA` — no qualifying explanation was captured.

“Unexplained” does not mean impossible or illegitimate. It describes only the retained telemetry.

Primary readiness features use independent observations in the unexplained class. Known and plausible incidents remain available for review.

## Audio cues without raw audio

No microphone or raw game audio is recorded.

### Gunshots

For a server-recorded shot, analysis uses:

- shooter and observer positions at the time of the shot;
- distance and bearing;
- weapon and ammunition type;
- muzzle type and fire mode;
- suppressor state and type;
- a versioned gunshot range model.

Suppressed and unsuppressed shots use separate conservative bands.

### Footsteps

For a moving target, analysis uses:

- authoritative position;
- velocity-derived speed and coarse gait;
- stance;
- terrain or surface type;
- footwear attachment type;
- observer distance and bearing;
- a versioned footstep model.

This is less certain than a gunshot because it does not reproduce exact animation sound events, building acoustics, weather masking, ambient noise, hearing damage, or client settings.

### Audibility classifications

| Classification | Interpretation |
|---|---|
| `CAPTURED_STRONG_CUE` | Strong server-derived opportunity, such as a nearby unsuppressed shot |
| `LIKELY_AUDIO_CUE` | Likely audible under ordinary conditions |
| `POSSIBLE_AUDIO_CUE` | Could have been audible, but not strong enough to explain the observation automatically |
| `NOT_AUDIBLE_BY_MODEL` | Outside the configured model range or missing usable context |

Strong gunshots may produce `KNOWN`. Likely gunshots or footsteps may produce `PLAUSIBLE`. Possible cues remain visible but do not automatically suppress analysis.

The system can say:

> A plausible audible opportunity existed.

It cannot say:

> The player definitely heard and localized the cue.

## Primary feature: hidden-threat readiness

For each direct durable player ID, the analyzer counts:

- hidden successes and trials;
- neutral-control successes and trials;
- independent sessions, encounters, and target identities.

A beta-binomial estimate shrinks sparse rates toward a prior. The main reported quantity is:

```text
readiness lift = hidden posterior rate - neutral-control posterior rate
```

Ranking uses the lower confidence bound rather than only the point estimate.

## Matched model

Exact-context strata must contain both hidden and neutral observations. Matching context includes available variables such as:

- server and map;
- coarse area cell;
- movement band and stance;
- baseline weapon state;
- camera/server policy;
- population band;
- cue class;
- sampling-policy version.

A conditional logistic model estimates the hidden-versus-neutral odds ratio. Sparse, non-converged, or separated models cannot satisfy the high-priority gate.

## Stability and negative controls

### Leave-one-session-out stability

The primary estimate is recalculated while excluding each session. High-priority output requires the effect direction to remain stable rather than depend on one period.

### Negative controls

Preregistered negative-control checks validate the analysis pipeline. Incomplete or failed controls suppress high-priority output. They are not evidence against a player.

## Supporting features

### Concealed-sector selection

Event-enrichment windows compare camera-turn direction with concealed-target bearing. A circular permutation test measures whether concealed sectors are selected more often than expected.

This is supporting context and does not bypass primary gates.

### Pre-exposure readiness

The analyzer compares a readiness interval with the first later exposure interval for the same target.

A definite ordering requires:

```text
readiness_upper_ms < exposure_lower_ms
```

Overlapping intervals remain ambiguous. Pre-exposure is `EXPERIMENTAL_SUPPORTING_ONLY` and cannot promote a review tier.

## Review tiers

| Tier | Meaning |
|---|---|
| `INSUFFICIENT_DATA` | Evidence breadth requirements were not met |
| `MONITOR` | Breadth passed, but the review-effect gate did not |
| `REVIEW` | Readiness-lift lower bound passed the review gate |
| `HIGH_PRIORITY_REVIEW` | Stronger lift plus matched-model, stability, and negative-control gates passed |

Default breadth gates are:

- at least 20 eligible hidden opportunities;
- at least 3 independent sessions;
- at least 5 independent encounters;
- at least 5 independent target identities.

Default lift gates are:

- `REVIEW` at a readiness-lift lower bound of at least `0.05`;
- possible `HIGH_PRIORITY_REVIEW` at at least `0.15` with all other gates passing.

These are versioned policy values, not universal scientific constants. Each deployment should calibrate them using trusted cohorts and review outcomes.

## What an administrator should review

Review the complete evidence package:

1. **Identity** — direct `player_id` and affected sessions.
2. **Data quality** — hidden, neutral, visible, audio, dropped, clock, and collector-loss counts.
3. **Breadth** — sessions, encounters, targets, and whether one period dominates.
4. **Uncertainty** — lower bounds, matched-model convergence, and stability.
5. **Cue history** — prior exposure, combat, gunshots, footsteps, and uncaptured information.
6. **Timeline** — authoritative positions, combat, client transitions, probes, and audio opportunities.
7. **Spatial context** — routes, related players, cue direction/distance, and coarse cells.
8. **Alternative explanations** — squad communication, prediction, map knowledge, or telemetry gaps.
9. **Conventional evidence** — server logs, anti-cheat evidence, reports, and video.

The conclusion should describe observable behaviour and uncertainty, not convert a tier into a cheat probability.

## Admin explorer and API

The explorer lists direct player and session IDs and reconstructs ordered timelines from PostgreSQL. The direct ID can be copied into Steam, BattleMetrics, ban, or ticket systems.

Spatial evidence keeps provenance separate:

- exact server positions for the selected route;
- related-player positions separately;
- untrusted client camera positions separately;
- visibility `area_cell` as a coarse 100 m cell.

Timeline requests are capped at 2,000 entries. Use `from_ms` and `to_ms` for larger sessions.

The review API exposes the direct identity as `player_id`. There is no legacy alias.

Screenshots and exports contain directly attributable player data and must remain inside the administrator boundary.

## Interpreting weak or missing output

No candidate may mean:

- visibility probing or first-person validation is unavailable;
- no neutral controls were collected;
- known or plausible cues explained eligible incidents;
- timing or queue limits were exceeded;
- breadth gates were not met;
- the matched model failed;
- stability or negative controls failed;
- the dataset is too small;
- retention or deletion removed required history.

Failing closed is preferable to manufacturing confidence from incomplete evidence.

## Non-goals

The analysis does not:

- detect or name cheat software;
- prove that one incident was impossible legitimately;
- record or recreate raw audio;
- guarantee that an inferred cue was heard;
- infer external communications;
- replace human investigation;
- trigger a ban, kick, warning, or gameplay action.
