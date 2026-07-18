# Milestone 0: DayZ Feasibility and Performance Spike

## Purpose

Prove every DayZ-side surface required by the behavioural-awareness MVP on the actual target DayZ version and establish safe collection budgets before production architecture is treated as valid.

## Required experiments

### Client sampling

- Mod `MissionGameplay.OnUpdate(float timeslice)` while preserving `super.OnUpdate(timeslice)`.
- Use an accumulator to capture 2 Hz and 10 Hz samples independent of frame rate.
- Capture:
  - `g_Game.GetCurrentCameraPosition()`;
  - `g_Game.GetCurrentCameraDirection()`;
  - `IsFireWeaponRaised()`;
  - `IsInIronsights()`;
  - `IsInOptics()`;
  - `IsInThirdPerson()`.
- Measure per-sample cost and behaviour through death, unconsciousness, respawn and reconnect.

### RPC batching

- Implement a project-owned `ScriptRPC` message.
- Send bounded batches with schema version, nonce and monotonic sequence.
- Receive through the chosen object-level `OnRPC` seam.
- Confirm the server receives the correct `PlayerIdentity` independently of payload identity fields.
- Test malformed, oversized, duplicate, stale and flooded batches.

### Successful-fire marker

- Mod `Weapon_Base.OnFire(int muzzle_index)` and call `super.OnFire(muzzle_index)`.
- Log execution side, local ownership, player identity availability, weapon and muzzle index.
- Test semi-auto, automatic, burst, shotgun/multi-muzzle and jam cases.
- Determine whether a reliable dedicated-server marker exists. If not, explicitly retain only the untrusted local client marker.

### Server snapshots and lifecycle

- Mod `MissionServer.OnUpdate(float timeslice)` with an accumulator.
- Enumerate players through `g_Game.GetPlayers` at 2 Hz.
- Capture position, orientation, alive/unconscious state and item in hands where safe.
- Validate connect, ready, respawn, reconnect and disconnect handling.

### Hit and death events

- Extend `PlayerBase.EEHitBy(...)` and `PlayerBase.EEKilled(...)` while preserving vanilla behaviour.
- Record exactly which callback fields are reliable on a dedicated server.
- Resolve root shooter from source only where the object hierarchy permits it.
- Confirm that no complete original native shot packet is available and document the actual limitations.

### Bone probe points

- Validate head and torso bone names/indices and `GetBonePositionWS` behaviour for:
  - standing;
  - crouched;
  - prone;
  - leaning/raised weapon;
  - unconscious/dead exclusion.
- Define fallback position offsets only if bone access proves unreliable.

### Visibility classification

Build controlled scenarios for:

- terrain ridge;
- building wall;
- open and closed door;
- player-built wall/fence;
- vehicle obstruction;
- trees/bushes/vegetation;
- target partly exposed at head only;
- target partly exposed at torso only.

Compare `RaycastRV` and `RaycastRVProxy` with `ObjIntersectView` and define deterministic rules for:

- `HARD_OCCLUDED`;
- `GEOMETRICALLY_EXPOSED`;
- `UNKNOWN`.

Vegetation and ambiguous contacts must remain `UNKNOWN` unless testing proves a safe narrower rule.

### Export and spool

- Create a localhost `RestContext`.
- Use asynchronous `POST`, never `POST_now`.
- Test success, timeout, connection refused, sidecar restart and DayZ server shutdown.
- Implement rotated spool files with bounded disk usage.
- Test replay and idempotency against a development receiver.

## Performance matrix

Test representative populations, for example:

- 10 players;
- 30 players;
- 60 players or the target server cap.

For each population measure:

- baseline server script timing without collector;
- 2 Hz authoritative snapshots;
- client RPC batch rates;
- event-gated candidate selection;
- 1, 3 and 5 visibility probes per decision event;
- diagnostic windows at configured maximum concurrency;
- JSON/serialization and export dispatch cost;
- queue/drop behaviour under artificial overload.

Use `TickCount` around each collector stage and record server FPS/tick health through existing operational tooling.

## Deliverables

1. Development-only DayZ client/server mod.
2. NDJSON development receiver or minimal Go ingest endpoint.
3. Surface validation matrix with pass/fail/replacement.
4. Payload-size measurements.
5. Safe initial sample/RPC/probe budgets.
6. Recorded visibility fixtures and expected classifications.
7. Updated specification removing or replacing failed assumptions.
8. Go/no-go recommendation for Milestone 1.

## Exit criteria

- Every required MVP surface is proven on the target DayZ version or replaced with a verified alternative.
- No fictional or assumed hook remains.
- Blocking REST is absent.
- All queues and buffers are bounded.
- Visibility ambiguity rules are documented.
- Representative load testing shows overhead within the operator-approved tolerance.
- The project can produce a replayable controlled-session telemetry file without running behavioural scoring inside DayZ.
