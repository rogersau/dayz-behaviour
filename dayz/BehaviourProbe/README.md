# DayZ Behaviour Probe

Development-only Milestone 0 mod. It is intentionally instrumented and is not a production anti-cheat.

## What it proves

- local `MissionGameplay.OnUpdate` camera/state sampling at 2 Hz or 10 Hz;
- bounded primitive `ScriptRPC` batches attributed through server-supplied `PlayerIdentity`;
- server `MissionServer.OnUpdate` snapshots via `GetGame().GetPlayers`;
- `Weapon_Base.OnFire` execution side and local shot markers;
- `PlayerBase.EEHitBy` and `PlayerBase.EEKilled` event fields;
- optional bounded `RaycastRVProxy` visibility probes;
- asynchronous `RestContext.POST` to the Go receiver;
- append-only failed-export spooling.

## Server configuration

On first launch the mod creates:

```text
$profile:DayZBehaviourProbe/config.json
```

Set `server_id`, `endpoint`, and `ingest_token`. The endpoint must end with `/`.

`ingest_token` is sent as a query parameter because the exposed DayZ `RestContext` API does not provide a general arbitrary-header setter. Keep the Go receiver bound to loopback. This is a Milestone 0 transport constraint, not the final internet-facing security design.

Visibility probes are disabled by default. Enable them only in controlled fixtures and keep `max_visibility_pairs_per_tick` low.

## Expected deployment

Pack `dayz/BehaviourProbe` as `DayZBehaviourProbe`, load it on both client and server for the spike, and run the Go receiver beside the dedicated server.
