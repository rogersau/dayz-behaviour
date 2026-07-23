modded class DBAProbeServerConfig
{
    bool enable_audio_cues = true;
    float audio_context_interval_seconds = 2.0;
    float audio_min_movement_speed_mps = 0.25;
};

modded class DBAProbeWirePayload
{
    string weapon_type;
    int muzzle_type;
    int fire_mode;
    bool is_suppressed;
    string suppressor_type;
    string surface_type;
    string footwear_type;
    string stance_name;
};

modded class DBAProbeCombatEvent
{
    string weapon_type;
    int muzzle_type;
    int fire_mode;
    bool is_suppressed;
    string suppressor_type;
    vector world_position;
};
