# Event context V1

## Purpose

`event_context` is the stable, server-authoritative snapshot attached to sampling opportunities at the time the opportunity is evaluated. It preserves the low-level state needed to build future behavioural features without adding another DayZ hook or relying on a nearby periodic snapshot.

The collector still performs no behavioural scoring. The external services may replay the retained context under later feature, matching, cue, and ranking policies.

## Where it is emitted

- `SAMPLING_OPPORTUNITY` for neutral no-target opportunities;
- `SAMPLING_OPPORTUNITY_DROPPED` so load-related missingness keeps observer context;
- `VISIBILITY_OBSERVATION` from both random prospective and event-enrichment streams.

A targeted visibility observation contains the current `event_context` and `visibility_ray_evidence`. When validated robust occlusion requires a second probe, the final event also contains `occlusion_start_event_context` and `occlusion_start_ray_evidence` from the first qualifying probe.

## Event context shape

```json
{
  "event_context": {
    "version": "v1",
    "captured_ms": 123456,
    "observer": {
      "player_id": "...",
      "player_session_id": "...",
      "position": "...",
      "velocity": "...",
      "orientation": "...",
      "movement_heading": "...",
      "movement_speed_mps": 0,
      "stance_id": 0,
      "movement_state": "STOPPED",
      "alive": true,
      "unconscious": false,
      "weapon_raised": false,
      "item_in_hands_type_id": "..."
    },
    "target": {
      "...": "same player context fields"
    }
  }
}
```

`target` is absent for a neutral no-relevant-target opportunity. Vector values use the native DayZ JSON serializer representation and are retained unchanged in raw and normalized payloads.

The fields are facts available at capture time:

- direct server-bound identity and player-session identity;
- authoritative position, velocity, body orientation, stance, life state, and item in hands;
- velocity-derived speed, movement heading, and coarse moving/stopped state;
- server-observed raised state.

Client camera direction, ADS, optics, and transition timing remain separate Tier B evidence and are not promoted into this Tier A structure.

## Raw visibility ray evidence

`visibility_ray_evidence` is versioned independently:

```json
{
  "visibility_ray_evidence": {
    "version": "v1",
    "points": [
      {
        "point_name": "HEAD",
        "ray_origin": "...",
        "ray_destination": "...",
        "result": "CLEAR",
        "contact_present": true,
        "contact_position": "...",
        "contact_direction": "...",
        "contact_distance_metres": 12.3,
        "contact_component": 4,
        "contact_hierarchy_level": 0,
        "contact_object_type": "...",
        "blocker_type": "..."
      }
    ]
  }
}
```

Possible point names are `HEAD`, `TORSO`, `PELVIS`, `LEFT_UPPER`, and `RIGHT_UPPER`. A missing optional bone is retained as an `AMBIGUOUS` point with blocker type `INVALID_BONE` rather than silently disappearing.

The existing derived fields such as `classification`, aggregate blocker type, point counts, visibility-policy version, sampling probabilities, and queue timings remain present. Raw point evidence allows those classifications to be audited or rebuilt later.

## Storage and replay

No PostgreSQL migration is required. The immutable raw batch stores the complete payload, `normalized_events.payload` preserves the nested objects, and `visibility_probe_runs.raw_evidence` stores the complete visibility payload. Derived tables may add indexed columns later without requiring a DayZ collector change.

Changing the meaning or shape of either object requires a new version value. Existing `v1` fields must not be reinterpreted in place.

## Runtime validation gate

Before treating this contract as production-verified:

1. compile all DayZ script modules on the target build and mod set;
2. inspect emitted JSON for standing, crouched, prone, moving, stationary, raised, and lowered players;
3. verify neutral opportunities omit only the target object;
4. validate ray contacts for terrain, buildings, doors, base-building objects, vehicles, vegetation, target proxies, and invalid bones;
5. verify sustained-occlusion events retain both initial and final contexts and ray evidence;
6. measure payload bytes, export queue depth, spool use, and script cost at representative population;
7. retain the existing conservative visibility gate until the labelled fixture passes.
