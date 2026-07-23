class DBAEventContextPlayerV1
{
    string player_id;
    string player_session_id;
    vector position;
    vector velocity;
    vector orientation;
    vector movement_heading;
    float movement_speed_mps;
    int stance_id;
    string movement_state;
    bool alive;
    bool unconscious;
    bool weapon_raised;
    string item_in_hands_type_id;
};

class DBAEventContextV1
{
    string version = "v1";
    int captured_ms;
    ref DBAEventContextPlayerV1 observer;
    ref DBAEventContextPlayerV1 target;
};

class DBAVisibilityPointEvidenceV1
{
    string point_name;
    vector ray_origin;
    vector ray_destination;
    string result;
    bool contact_present;
    vector contact_position;
    vector contact_direction;
    float contact_distance_metres;
    int contact_component = -1;
    int contact_hierarchy_level = -1;
    string contact_object_type;
    string blocker_type;
};

class DBAVisibilityRayEvidenceV1
{
    string version = "v1";
    ref array<ref DBAVisibilityPointEvidenceV1> points = new array<ref DBAVisibilityPointEvidenceV1>;
};

modded class DBAProbeWirePayload
{
    ref DBAEventContextV1 event_context;
    ref DBAVisibilityRayEvidenceV1 visibility_ray_evidence;
    ref DBAEventContextV1 occlusion_start_event_context;
    ref DBAVisibilityRayEvidenceV1 occlusion_start_ray_evidence;
};
