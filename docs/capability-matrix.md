# DayZ 1.29 capability matrix

Status date: 2026-07-19  
Build: 1.29.0.163451  
Mission: local diagnostic `Test.chernarusplus`

`Loaded` means the exact mod compiled in the real dedicated-server script runtime. It does not imply a callback was exercised by a player.

| Surface | Local client | Remote entity | Dedicated server | Authority | Evidence |
|---|---:|---:|---:|---|---|
| `MissionGameplay.OnUpdate` camera/state | Executed | N/A | N/A | B | Camera samples accepted and normalized in one-client DayZDiag fixture |
| raised / ADS / optics / third person | Pending | Not used | Server raised state recorded as context only | B/A-context | Controlled state fixture required |
| `DayZGame.OnRPC` nonce/batch/edge/clock/health | Executed | N/A | Executed | B joined to server identity | Automated diagnostic PASS plus normalized camera, edge, clock and health events |
| `MissionServer.OnUpdate` and `GetPlayers` | N/A | N/A | Loaded and executed | A | Startup health events exported |
| connect/ready/respawn/reconnect/disconnect hooks | N/A | N/A | Partially executed | A | Connect, new-character ready and disconnect verified; respawn/reconnect pending |
| `EEHitBy` / `EEKilled` | N/A | Loaded code | Loaded code | A | Combat callback fixture pending |
| `Weapon_Base.OnFire` | Pending | Pending | Loaded code | diagnostic until proven | Execution-side fixture pending |
| head/torso/additional bone positions | N/A | Loaded code | Loaded code | A | Stance/animation fixture pending |
| `RaycastRVProxy` view geometry | N/A | Loaded code | Loaded code | A | Controlled confusion matrix pending |
| `RestContext.POST` success | N/A | N/A | Executed | A transport | Live diagnostic events persisted through the continuous normalizer |
| `RestContext` failure callback/spool | N/A | N/A | Executed | A transport | Receiver-down `error:7` spooled and replayed |

## Safe interpretation

- Default visibility origin is `PLAYER_HEAD_APPROXIMATION`; blocked rays are `HEAD_ORIGIN_OCCLUDED`, never strong evidence.
- `ROBUSTLY_OCCLUDED` requires all of: a server that enforces first person, `server_first_person_only=true`, `visibility_origin_mode=VALIDATED_FIRST_PERSON_HEAD`, a non-empty controlled-validation ID, and a repeated blocked probe meeting the configured minimum duration.
- Third-person or unknown-perspective head rays cannot become `ROBUSTLY_OCCLUDED`.
- Queued observer and target entities are revalidated for identity, life state, consciousness and range immediately before probing.
- Missing or implausible client telemetry can suppress evidence but cannot strengthen it.

## Next controlled run

Use at least two connected clients and labelled observer/target placements. Capture the execution side, values and timing for every pending row before issuing a visibility validation ID or increasing collection budgets. The reconnect fixture must also confirm that client sequences restarting from one are accepted after the player session is rotated.
