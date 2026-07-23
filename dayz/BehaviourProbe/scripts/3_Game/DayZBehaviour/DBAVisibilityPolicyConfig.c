modded class DBAProbeServerConfig
{
    // Strong visibility evidence may only be enabled when the server itself
    // enforces first-person perspective and the head-origin policy has passed
    // a controlled validation matrix identified by visibility_validation_id.
    bool server_first_person_only = false;
};
