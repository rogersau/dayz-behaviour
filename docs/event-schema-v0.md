# Telemetry Event Schema v0

Schema version 1 is implemented. Event-specific sampling and visibility invariants are validated before durable ingestion; payload additions remain backward-compatible within the version until the controlled fixture run freezes the contract.

## Envelope

Every exported event includes:

```text
schema_version
server_id
server_session_id
server_sequence
server_time_ms
event_type
player_session_id
payload
```

Client-derived events additionally preserve:

```text
client_sequence
client_monotonic_time_ms
server_receive_time_ms
client_time_offset_estimate_ms
client_time_uncertainty_ms
telemetry_trust = UNTRUSTED_CLIENT
```

## Event types

### Lifecycle

```text
PLAYER_CONNECTED
PLAYER_READY
PLAYER_RESPAWNED
PLAYER_RECONNECTED
PLAYER_DISCONNECTED
```

### Server snapshots

```text
PLAYER_SNAPSHOT
```

Payload:

```text
position
orientation
alive
unconscious
item_in_hands_type_id
movement_state
movement_speed_mps
movement_heading
movement_transition
```

### Client camera samples

```text
CAMERA_SAMPLE
```

Sample payload:

```text
camera_position
camera_direction
client_observed_player_position
is_weapon_raised
is_in_ironsights
is_in_optics
is_in_third_person
item_in_hands_type_id
flags
```

### Derived decision events

```text
WEAPON_RAISED
WEAPON_LOWERED
ADS_ENTERED
ADS_EXITED
CAMERA_TURN
MOVEMENT_STARTED
MOVEMENT_STOPPED
ROUTE_HEADING_CHANGED
```

Every derived event records the source sample/snapshot references and algorithm version.

Decision edges are exported as `DECISION_EDGE` with the edge name in `sampling_reason`; they remain Tier B and carry authoritative server-receive time.

### Combat

```text
SHOT_MARKER_CLIENT
SHOT_MARKER_SERVER
PLAYER_HIT
PLAYER_KILLED
```

`SHOT_MARKER_SERVER` is emitted only if the feasibility spike proves a reliable dedicated-server seam.

### Visibility

```text
VISIBILITY_OBSERVATION
DIAGNOSTIC_PAIR_WINDOW_STARTED
DIAGNOSTIC_PAIR_WINDOW_ENDED
```

Visibility events also carry authority, sampling stream/policy/reason, risk-set definition, eligible counts, inclusion and admission probabilities, queue state/timing, point results, blocker categories, origin mode, validation ID and occlusion duration. `ROBUSTLY_OCCLUDED` is rejected unless the origin is validated first person and duration is positive.

Visibility payload:

```text
observer_player_session_id
target_player_session_id
classification = HARD_OCCLUDED | GEOMETRICALLY_EXPOSED | UNKNOWN
observer_origin
target_probe_points
blocker_category
probe_count
probe_algorithm_version
trigger_event_reference
```

### Collector health

```text
CLIENT_TELEMETRY_HEALTH
SERVER_COLLECTOR_HEALTH
EXPORT_HEALTH
```

Health fields include sample counts, dropped samples, invalid RPCs, queue depth, probes completed/dropped, export failures and spool usage.

## Identity rules

- The DayZ server assigns `player_session_id`.
- RPC attribution uses server-supplied `PlayerIdentity` only.
- Client-supplied identity values are ignored.
- Long-term player identifiers are stored separately from high-volume event rows and exposed only through authorised review APIs.

## Versioning rules

- Breaking wire changes increment `schema_version`.
- Derived features record `feature_algorithm_version`.
- Visibility classifications record `probe_algorithm_version`.
- Raw batches are retained so later versions can replay historical data.
