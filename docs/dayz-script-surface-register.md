# Verified DayZ Script Surface Register

This register exists to stop the project from drifting into fictional hooks. Every DayZ-side implementation must reference a real script surface and be revalidated against the target game version during Milestone 0.

Source repository: `BohemiaInteractive/DayZ-Script-Diff`

## Client sampling and local state

| Surface | Source path | Use |
|---|---|---|
| `MissionGameplay.OnUpdate(float timeslice)` | `scripts/5_mission/mission/missiongameplay.c` | Accumulator-driven local sampling |
| `g_Game.GetCurrentCameraPosition()` | demonstrated in `scripts/4_world/entities/itembase/rangefinder.c` and `scripts/4_world/classes/weapondebug.c` | Local camera origin |
| `g_Game.GetCurrentCameraDirection()` | demonstrated in the same files | Local camera direction |
| `DayZPlayerImplement.IsInIronsights()` | `scripts/4_world/entities/dayzplayerimplement.c` | Sampled ADS state |
| `DayZPlayerImplement.IsInOptics()` | same | Sampled optics state |
| `DayZPlayerImplement.IsInThirdPerson()` | same | Camera mode context |
| `DayZPlayerImplement.IsFireWeaponRaised()` | same | Sampled weapon-raised state |

## RPC transport

| Surface | Source path | Use |
|---|---|---|
| `ScriptRPC.Write(...)` / `ScriptRPC.Send(...)` | examples in mission and remote debug scripts | Client-to-server batches |
| `OnRPC(PlayerIdentity sender, Object target, int rpcType, ParamsReadContext ctx)` or object-level equivalent | global and entity script surfaces | Receive and bind to server-supplied identity |

The implementation must use a project-owned RPC ID that does not collide with vanilla/mod RPC IDs and must validate schema, size, nonce and sequence.

## Combat events

| Surface | Source path | Use |
|---|---|---|
| native `TryFireWeapon(...)` followed by `m_weapon.OnFire(muzzleIndex)` on success | `scripts/4_world/entities/firearms/fsm/states/weaponfire.c` | Feasibility spike for a successful-fire marker |
| `Weapon_Base.OnFire(int muzzle_index)` | `scripts/4_world/entities/firearms/weapon_base.c` | Same-frame client shot sample; server execution must be proven |
| `PlayerBase.EEHitBy(...)` | `scripts/4_world/entities/manbase/playerbase.c` | Server hit event |
| `PlayerBase.EEKilled(...)` | same | Server death event |

`EEHitBy` does not expose the complete original native shot packet. The project must not claim otherwise.

## Server collection

| Surface | Source path | Use |
|---|---|---|
| `MissionServer.OnUpdate(float timeslice)` | `scripts/5_mission/mission/missionserver.c` | Bounded snapshot and work-queue processing |
| `g_Game.GetPlayers(array<Man>)` | demonstrated by `MissionServer.UpdatePlayersStats` | Connected-player enumeration |
| player entity position/orientation and validated bone world-position APIs | entity/player scripts | Authoritative snapshots and visibility probe points |

## Visibility

| Surface | Source path | Use |
|---|---|---|
| `DayZPhysics.RaycastRV(...)` | `scripts/3_game/global/dayzphysics.c` | Simple bounded view probes |
| `DayZPhysics.RaycastRVProxy(...)` | same | Probe variant where blocker/result details are needed |
| `ObjIntersectView` | same | View-geometry classification |

Raw ray outputs are clear, hard-blocked or ambiguous per point. Derived outputs are `EXPOSED`, `HEAD_ORIGIN_OCCLUDED`, `ROBUSTLY_OCCLUDED` or `AMBIGUOUS`. `ROBUSTLY_OCCLUDED` is unavailable by default and requires a controlled first-person validation ID plus repeated-duration confirmation. These remain geometric evidence, not statements about exact rendered perception.

## Export and persistence

| Surface | Source path | Use |
|---|---|---|
| `RestContext.POST(...)` | `scripts/3_game/http/restapi.c` | Asynchronous server-to-Go export |
| `RestContext.POST_now(...)` | same | Explicitly prohibited in gameplay code because it blocks |
| `OpenFile`, `FPrint`, `CloseFile` | core system/file APIs | Rotated spool files |
| `JsonSerializer` / `JsonFileLoader` | `scripts/3_game/tools/jsonfileloader.c` | Schema serialization/config and completed batches |
| `TickCount(...)` | `scripts/1_core/proto/ensystem.c` | Collector performance instrumentation |

## Runtime status on DayZ 1.29.0.163451

PBO packing, Game/World/Mission module loading, `MissionServer.OnUpdate`, asynchronous REST success/failure, bounded spooling and end-to-end Go ingestion are verified on the target dedicated server.

The following still require connected-player controlled fixtures:

1. Exact client and dedicated-server execution of a modded `Weapon_Base.OnFire` override.
2. Best object-level `OnRPC` receive seam for the required mod packaging.
3. Stable head/torso bone names and world positions across stance/animation states.
4. Raycast result behaviour for terrain, buildings, open/closed doors, base-building objects, vehicles and vegetation.
5. Reliable asynchronous REST behaviour and callback lifecycle under server restart/failure.
6. Safe sampling, RPC and probe budgets at representative player counts.
