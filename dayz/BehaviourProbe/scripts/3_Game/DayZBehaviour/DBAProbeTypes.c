class DBAProbeConstants
{
    static const int SCHEMA_VERSION = 1;

    // Project-owned RPC IDs. Milestone 0 must confirm these do not conflict with
    // the target server's mod set before production use.
    static const int RPC_SERVER_NONCE = 759430;
    static const int RPC_CLIENT_SAMPLE_BATCH = 759431;
    static const int RPC_DECISION_EDGE = 759432;
    static const int RPC_CLOCK_CHALLENGE = 759433;
    static const int RPC_CLOCK_RESPONSE = 759434;
    static const int RPC_CLIENT_HEALTH = 759435;

    static const int MAX_CLIENT_SAMPLES_PER_BATCH = 32;
    static const int MAX_QUEUED_CLIENT_BATCHES = 64;
    static const int MAX_QUEUED_COMBAT_EVENTS = 128;
    static const int MAX_QUEUED_DECISION_EDGES = 128;
    static const int MAX_QUEUED_CLOCK_SAMPLES = 128;
    static const int MAX_QUEUED_CLIENT_HEALTH = 64;
};

class DBAProbeServerConfig
{
    bool enabled = true;
    string server_id = "dayz-development";
    string endpoint = "http://127.0.0.1:8080/";
    string ingest_token = "";
    string configuration_hash = "development-default";
    float server_snapshot_interval_seconds = 0.5;
    float export_interval_seconds = 1.0;
    int max_events_per_export = 128;
    int max_pending_events = 512;
    bool enable_visibility_probe = false;
    int max_visibility_pairs_per_tick = 1;
    float visibility_radius_metres = 1000.0;
    float random_opportunity_min_seconds = 3.0;
    float random_opportunity_max_seconds = 8.0;
    int max_random_opportunity_queue = 64;
    int max_event_enrichment_queue = 64;
    int max_probe_queue_age_ms = 1000;
    int max_spool_batches_per_file = 256;
    int max_spool_files = 8;
    float clock_challenge_interval_seconds = 30.0;
    string visibility_origin_mode = "PLAYER_HEAD_APPROXIMATION";
    string visibility_validation_id = "";
    int minimum_occlusion_duration_ms = 250;
};

class DBAClientSample
{
    int client_sequence;
    int client_monotonic_time_ms;
    vector camera_position;
    vector camera_direction;
    vector client_observed_player_position;
    bool is_weapon_raised;
    bool is_in_ironsights;
    bool is_in_optics;
    bool is_in_third_person;
    string item_in_hands_type_id;
    string sample_mode;
    int local_shot_count;
    int local_shot_muzzle_index;
};

class DBAReceivedClientBatch
{
    string player_id;
    string player_name;
    int batch_sequence;
    int server_receive_time_ms;
    ref array<ref DBAClientSample> samples = new array<ref DBAClientSample>;
};

class DBAProbeCombatEvent
{
    string event_type;
    int server_time_ms;
    string target_player_id;
    string source_player_id;
    string source_type;
    int damage_type;
    int component;
    string damage_zone;
    string ammo;
    vector model_position;
    float speed_coefficient;
};

class DBADecisionEdge
{
    string player_id;
    string edge_type;
    int edge_sequence;
    int client_monotonic_time_ms;
    int server_receive_time_ms;
    vector camera_direction;
};

class DBAClockAlignment
{
    string player_id;
    int challenge_sequence;
    int server_send_ms;
    int client_receive_ms;
    int client_send_ms;
    int server_receive_ms;
    int round_trip_ms;
    float offset_estimate_ms;
    float uncertainty_ms;
};

class DBAClientCollectorHealth
{
    string player_id;
    int server_receive_time_ms;
    int samples_captured;
    int batches_attempted;
    int edges_attempted;
    int dropped_samples;
    int queued_samples;
};

class DBAClientTransitionDetector
{
    protected ref DBAClientSample m_Previous;
    protected bool m_TurnActive;
    protected int m_LastTurnMS;

    void Process(DBAClientSample current, notnull array<string> output)
    {
        if (!current)
        {
            return;
        }
        if (!m_Previous)
        {
            m_Previous = current;
            return;
        }

        if (!m_Previous.is_weapon_raised && current.is_weapon_raised)
        {
            output.Insert("WEAPON_RAISED");
        }
        if (m_Previous.is_weapon_raised && !current.is_weapon_raised)
        {
            output.Insert("WEAPON_LOWERED");
        }
        if (!m_Previous.is_in_ironsights && current.is_in_ironsights)
        {
            output.Insert("ADS_ENTERED");
        }
        if (m_Previous.is_in_ironsights && !current.is_in_ironsights)
        {
            output.Insert("ADS_EXITED");
        }
        if (!m_Previous.is_in_optics && current.is_in_optics)
        {
            output.Insert("OPTICS_ENTERED");
        }
        if (m_Previous.is_in_optics && !current.is_in_optics)
        {
            output.Insert("OPTICS_EXITED");
        }
        if (m_Previous.is_in_third_person != current.is_in_third_person)
        {
            output.Insert("CAMERA_MODE_CHANGED");
        }
        if (current.local_shot_count > 0)
        {
            output.Insert("SHOT_FIRED_CLIENT");
        }
        float directionDot = vector.Dot(m_Previous.camera_direction, current.camera_direction);
        if (directionDot > 1.0)
        {
            directionDot = 1.0;
        }
        if (directionDot < -1.0)
        {
            directionDot = -1.0;
        }
        float angularDisplacement = Math.Acos(directionDot) * Math.RAD2DEG;
        if (angularDisplacement >= 25.0 && !m_TurnActive && current.client_monotonic_time_ms - m_LastTurnMS >= 750)
        {
            output.Insert("DELIBERATE_CAMERA_TURN");
            m_TurnActive = true;
            m_LastTurnMS = current.client_monotonic_time_ms;
        }
        else if (angularDisplacement <= 8.0)
        {
            m_TurnActive = false;
        }
        m_Previous = current;
    }
};

class DBAProbeWirePayload
{
    vector position;
    vector orientation;
    bool alive;
    bool unconscious;
    string item_in_hands_type_id;
    string lifecycle_state;
    bool authentication_failed;
    int logout_time_seconds;
    string movement_state;
    float movement_speed_mps;
    vector movement_heading;
    string movement_transition;
    string map_id;
    string area_cell;
    float target_distance_metres;
    float observer_speed_mps;
    int observer_stance_id;
    bool baseline_weapon_raised;
    string baseline_weapon_state;
    string camera_mode;
    int server_population_count;

    vector camera_position;
    vector camera_direction;
    vector client_observed_player_position;
    bool is_weapon_raised;
    bool is_in_ironsights;
    bool is_in_optics;
    bool is_in_third_person;
    string sample_mode;
    int local_shot_count;
    int local_shot_muzzle_index;

    string source_player_id;
    string source_type;
    int damage_type;
    int component;
    string damage_zone;
    string ammo;
    vector model_position;
    float speed_coefficient;

    string observer_player_id;
    string target_player_id;
    string observer_player_session_id;
    string target_player_session_id;
    string classification;
    string observer_origin_mode;
    vector observer_origin;
    vector target_head;
    vector target_torso;
    string blocker_type;
    int probe_count;
    int points_attempted;
    int points_clear;
    int points_hard_blocked;
    int points_ambiguous;
    string head_point_result;
    string torso_point_result;
    string pelvis_point_result;
    string left_upper_point_result;
    string right_upper_point_result;
    bool bone_validity;

    string sampling_stream;
    string sampling_policy_version;
    string sampling_reason;
    int observer_eligible_count;
    float observer_inclusion_probability;
    int target_eligible_count;
    float target_inclusion_probability;
    string risk_set_definition;
    bool risk_set_complete;
    float queue_admission_probability;
    string scheduler_load_state;
    int queue_delay_ms;
    string drop_reason;

    string visibility_policy_version;
    int probe_queued_ms;
    int probe_started_ms;
    int probe_completed_ms;
    int occlusion_duration_ms;
    string visibility_validation_id;
    int clock_server_send_ms;
    int clock_client_receive_ms;
    int clock_client_send_ms;
    int clock_server_receive_ms;
    int clock_round_trip_ms;
    float clock_offset_estimate_ms;
    float clock_uncertainty_ms;
    int pending_export_events;
    int dropped_export_events;
    int export_success_count;
    int export_failure_count;
    int spool_overwrite_count;
    int random_opportunity_queue_depth;
    int event_enrichment_queue_depth;
    int random_opportunity_drop_count;
    int event_enrichment_drop_count;
    int client_samples_captured;
    int client_batches_attempted;
    int client_edges_attempted;
    int client_dropped_samples;
    int client_queued_samples;
    int accepted_client_rpc_count;
    int rejected_client_rpc_count;
};

class DBAProbeWireEvent
{
    string event_type;
    string source;
    string source_authority;
    string source_component;
    string source_event_id;
    int source_schema_version;
    string collector_version;
    int server_sequence;
    int server_time_ms;
    int server_receive_ms;
    string player_session_id;
    int client_sequence;
    int client_monotonic_time_ms;
    ref DBAProbeWirePayload payload;
};

class DBAProbeWireBatch
{
    int schema_version;
    string server_id;
    string server_session_id;
    int batch_sequence;
    int server_time_ms;
    string collector_version;
    string dayz_build;
    string configuration_hash;
    ref array<ref DBAProbeWireEvent> events = new array<ref DBAProbeWireEvent>;
};

class DBAProbeRuntime
{
    protected static string s_ClientNonce;
    protected static string s_ServerNonce;
    protected static int s_LocalShotCount;
    protected static int s_LocalShotMuzzleIndex = -1;

    protected static ref array<ref DBAReceivedClientBatch> s_ReceivedClientBatches = new array<ref DBAReceivedClientBatch>;
    protected static ref array<ref DBAProbeCombatEvent> s_CombatEvents = new array<ref DBAProbeCombatEvent>;
    protected static ref array<ref DBADecisionEdge> s_DecisionEdges = new array<ref DBADecisionEdge>;
    protected static ref array<ref DBAClockAlignment> s_ClockSamples = new array<ref DBAClockAlignment>;
    protected static ref array<ref DBAClientCollectorHealth> s_ClientHealth = new array<ref DBAClientCollectorHealth>;
    protected static ref map<string, int> s_LastBatchSequenceByPlayer = new map<string, int>;
    protected static ref map<string, int> s_LastEdgeSequenceByPlayer = new map<string, int>;
    protected static ref map<string, int> s_EdgeWindowStartByPlayer = new map<string, int>;
    protected static ref map<string, int> s_EdgeCountByPlayer = new map<string, int>;
    protected static ref map<string, int> s_BatchWindowStartByPlayer = new map<string, int>;
    protected static ref map<string, int> s_BatchCountByPlayer = new map<string, int>;
    protected static int s_AcceptedClientRPCCount;
    protected static int s_RejectedClientRPCCount;
    protected static bool s_DiagClientTestPassed;

    static void SetClientNonce(string nonce)
    {
        s_ClientNonce = nonce;
        Print("[DayZBehaviourProbe] received server nonce");
#ifdef DIAG
        Print("[DAYZ-CLIENT-TEST] CHECK name=server_nonce_received pass=1");
#endif
    }

    static string GetClientNonce()
    {
        return s_ClientNonce;
    }

    static void SetServerNonce(string nonce)
    {
        s_ServerNonce = nonce;
        s_LastBatchSequenceByPlayer.Clear();
        s_LastEdgeSequenceByPlayer.Clear();
        s_EdgeWindowStartByPlayer.Clear();
        s_EdgeCountByPlayer.Clear();
        s_BatchWindowStartByPlayer.Clear();
        s_BatchCountByPlayer.Clear();
        s_AcceptedClientRPCCount = 0;
        s_RejectedClientRPCCount = 0;
        s_DiagClientTestPassed = false;
    }

    static string GetServerNonce()
    {
        return s_ServerNonce;
    }

    static void MarkLocalShot(int muzzleIndex)
    {
        s_LocalShotCount++;
        s_LocalShotMuzzleIndex = muzzleIndex;
    }

    static void ConsumeLocalShot(out int count, out int muzzleIndex)
    {
        count = s_LocalShotCount;
        muzzleIndex = s_LocalShotMuzzleIndex;
        s_LocalShotCount = 0;
        s_LocalShotMuzzleIndex = -1;
    }

    static bool ReceiveClientBatch(PlayerIdentity sender, ParamsReadContext ctx)
    {
        if (!sender)
        {
            Print("[DayZBehaviourProbe] rejected client batch without sender identity");
            return false;
        }

        int schemaVersion;
        string nonce;
        int batchSequence;
        int sampleCount;

        if (!ctx.Read(schemaVersion) || !ctx.Read(nonce) || !ctx.Read(batchSequence) || !ctx.Read(sampleCount))
        {
            Print("[DayZBehaviourProbe] rejected malformed client batch header");
            return false;
        }

        if (schemaVersion != DBAProbeConstants.SCHEMA_VERSION || nonce != s_ServerNonce || batchSequence < 1)
        {
            Print("[DayZBehaviourProbe] rejected client batch with invalid schema or nonce");
            return false;
        }

        if (sampleCount < 1 || sampleCount > DBAProbeConstants.MAX_CLIENT_SAMPLES_PER_BATCH)
        {
            Print("[DayZBehaviourProbe] rejected client batch with invalid sample count=" + sampleCount.ToString());
            return false;
        }

        string playerID = sender.GetId();
        int nowMS = GetGame().GetTime();
        int batchWindowStart;
        int batchCount;
        if (!s_BatchWindowStartByPlayer.Find(playerID, batchWindowStart) || nowMS - batchWindowStart >= 1000)
        {
            batchWindowStart = nowMS;
            batchCount = 0;
        }
        else
        {
            s_BatchCountByPlayer.Find(playerID, batchCount);
        }
        if (batchCount >= 4)
        {
            return false;
        }
        int previousSequence;
        if (s_LastBatchSequenceByPlayer.Find(playerID, previousSequence) && batchSequence <= previousSequence)
        {
            Print("[DayZBehaviourProbe] rejected duplicate/stale client batch for " + playerID);
            return false;
        }

        DBAReceivedClientBatch batch = new DBAReceivedClientBatch;
        batch.player_id = playerID;
        batch.player_name = sender.GetName();
        batch.batch_sequence = batchSequence;
        batch.server_receive_time_ms = GetGame().GetTime();

        int previousClientSequence;
        int previousClientTimeMS;
        for (int index = 0; index < sampleCount; index++)
        {
            DBAClientSample sample = new DBAClientSample;
            bool sampleValid = ctx.Read(sample.client_sequence);
            sampleValid = ctx.Read(sample.client_monotonic_time_ms) && sampleValid;
            sampleValid = ctx.Read(sample.camera_position) && sampleValid;
            sampleValid = ctx.Read(sample.camera_direction) && sampleValid;
            sampleValid = ctx.Read(sample.client_observed_player_position) && sampleValid;
            sampleValid = ctx.Read(sample.is_weapon_raised) && sampleValid;
            sampleValid = ctx.Read(sample.is_in_ironsights) && sampleValid;
            sampleValid = ctx.Read(sample.is_in_optics) && sampleValid;
            sampleValid = ctx.Read(sample.is_in_third_person) && sampleValid;
            sampleValid = ctx.Read(sample.item_in_hands_type_id) && sampleValid;
            sampleValid = ctx.Read(sample.sample_mode) && sampleValid;
            sampleValid = ctx.Read(sample.local_shot_count) && sampleValid;
            sampleValid = ctx.Read(sample.local_shot_muzzle_index) && sampleValid;
            if (!sampleValid)
            {
                Print("[DayZBehaviourProbe] rejected malformed client sample at index=" + index.ToString());
                return false;
            }

            if (sample.client_sequence < 1 || sample.client_monotonic_time_ms < 0 || (index > 0 && (sample.client_sequence <= previousClientSequence || sample.client_monotonic_time_ms < previousClientTimeMS)))
            {
                return false;
            }
            float directionLength = sample.camera_direction.Length();
            if (directionLength < 0.5 || directionLength > 1.5 || !IsDBAProbeVectorPlausible(sample.camera_position) || !IsDBAProbeVectorPlausible(sample.client_observed_player_position))
            {
                return false;
            }
            previousClientSequence = sample.client_sequence;
            previousClientTimeMS = sample.client_monotonic_time_ms;

            batch.samples.Insert(sample);
        }

        s_LastBatchSequenceByPlayer.Set(playerID, batchSequence);
        s_BatchWindowStartByPlayer.Set(playerID, batchWindowStart);
        s_BatchCountByPlayer.Set(playerID, batchCount + 1);
        if (s_ReceivedClientBatches.Count() >= DBAProbeConstants.MAX_QUEUED_CLIENT_BATCHES)
        {
            s_ReceivedClientBatches.RemoveOrdered(0);
        }
        s_ReceivedClientBatches.Insert(batch);
        return true;
    }

    static void DrainClientBatches(notnull array<ref DBAReceivedClientBatch> output)
    {
        while (s_ReceivedClientBatches.Count() > 0)
        {
            output.Insert(s_ReceivedClientBatches.Get(0));
            s_ReceivedClientBatches.RemoveOrdered(0);
        }
    }

    static void RecordClientRPCResult(bool accepted)
    {
        if (accepted)
        {
            s_AcceptedClientRPCCount++;
        }
        else
        {
            s_RejectedClientRPCCount++;
        }
    }

    static int GetAcceptedClientRPCCount() { return s_AcceptedClientRPCCount; }
    static int GetRejectedClientRPCCount() { return s_RejectedClientRPCCount; }

    static void MarkDiagClientTestPassed()
    {
#ifdef DIAG
        if (!s_DiagClientTestPassed)
        {
            s_DiagClientTestPassed = true;
            Print("[DAYZ-CLIENT-TEST] CHECK name=client_sample_accepted pass=1");
            Print("[DAYZ-CLIENT-TEST] RESULT outcome=PASS");
        }
#endif
    }

    static bool ReceiveDecisionEdge(PlayerIdentity sender, ParamsReadContext ctx)
    {
        if (!sender)
        {
            return false;
        }

        int schemaVersion;
        string nonce;
        int edgeSequence;
        int clientTimeMS;
        string edgeType;
        bool valid = ctx.Read(schemaVersion);
        valid = ctx.Read(nonce) && valid;
        valid = ctx.Read(edgeSequence) && valid;
        valid = ctx.Read(clientTimeMS) && valid;
        valid = ctx.Read(edgeType) && valid;
        vector edgeDirection;
        valid = ctx.Read(edgeDirection) && valid;
        if (!valid || schemaVersion != DBAProbeConstants.SCHEMA_VERSION || nonce != s_ServerNonce || edgeSequence < 1 || clientTimeMS < 0 || !IsDBAProbeDecisionEdge(edgeType) || edgeDirection.Length() < 0.5 || edgeDirection.Length() > 1.5)
        {
            return false;
        }

        string playerID = sender.GetId();
        int previousSequence;
        if (s_LastEdgeSequenceByPlayer.Find(playerID, previousSequence) && edgeSequence <= previousSequence)
        {
            return false;
        }

        int nowMS = GetGame().GetTime();
        int windowStart;
        int edgeCount;
        if (!s_EdgeWindowStartByPlayer.Find(playerID, windowStart) || nowMS - windowStart >= 1000)
        {
            windowStart = nowMS;
            edgeCount = 0;
        }
        else
        {
            s_EdgeCountByPlayer.Find(playerID, edgeCount);
        }
        if (edgeCount >= 10)
        {
            return false;
        }

        DBADecisionEdge edge = new DBADecisionEdge;
        edge.player_id = playerID;
        edge.edge_type = edgeType;
        edge.edge_sequence = edgeSequence;
        edge.client_monotonic_time_ms = clientTimeMS;
        edge.server_receive_time_ms = nowMS;
        edge.camera_direction = edgeDirection;
        s_LastEdgeSequenceByPlayer.Set(playerID, edgeSequence);
        s_EdgeWindowStartByPlayer.Set(playerID, windowStart);
        s_EdgeCountByPlayer.Set(playerID, edgeCount + 1);

        if (s_DecisionEdges.Count() >= DBAProbeConstants.MAX_QUEUED_DECISION_EDGES)
        {
            s_DecisionEdges.RemoveOrdered(0);
        }
        s_DecisionEdges.Insert(edge);
        return true;
    }

    static void DrainDecisionEdges(notnull array<ref DBADecisionEdge> output)
    {
        while (s_DecisionEdges.Count() > 0)
        {
            output.Insert(s_DecisionEdges.Get(0));
            s_DecisionEdges.RemoveOrdered(0);
        }
    }

    static bool ReceiveClockResponse(PlayerIdentity sender, ParamsReadContext ctx)
    {
        if (!sender)
        {
            return false;
        }
        int schemaVersion;
        string nonce;
        int challengeSequence;
        int serverSendMS;
        int clientReceiveMS;
        int clientSendMS;
        bool valid = ctx.Read(schemaVersion);
        valid = ctx.Read(nonce) && valid;
        valid = ctx.Read(challengeSequence) && valid;
        valid = ctx.Read(serverSendMS) && valid;
        valid = ctx.Read(clientReceiveMS) && valid;
        valid = ctx.Read(clientSendMS) && valid;
        if (!valid || schemaVersion != DBAProbeConstants.SCHEMA_VERSION || nonce != s_ServerNonce)
        {
            return false;
        }

        int serverReceiveMS = GetGame().GetTime();
        int roundTripMS = (serverReceiveMS - serverSendMS) - (clientSendMS - clientReceiveMS);
        if (roundTripMS < 0)
        {
            roundTripMS = 0;
        }
        DBAClockAlignment sample = new DBAClockAlignment;
        sample.player_id = sender.GetId();
        sample.challenge_sequence = challengeSequence;
        sample.server_send_ms = serverSendMS;
        sample.client_receive_ms = clientReceiveMS;
        sample.client_send_ms = clientSendMS;
        sample.server_receive_ms = serverReceiveMS;
        sample.round_trip_ms = roundTripMS;
        sample.offset_estimate_ms = ((serverSendMS - clientReceiveMS) + (serverReceiveMS - clientSendMS)) * 0.5;
        sample.uncertainty_ms = roundTripMS * 0.5;
        if (s_ClockSamples.Count() >= DBAProbeConstants.MAX_QUEUED_CLOCK_SAMPLES)
        {
            s_ClockSamples.RemoveOrdered(0);
        }
        s_ClockSamples.Insert(sample);
        return true;
    }

    static void DrainClockSamples(notnull array<ref DBAClockAlignment> output)
    {
        while (s_ClockSamples.Count() > 0)
        {
            output.Insert(s_ClockSamples.Get(0));
            s_ClockSamples.RemoveOrdered(0);
        }
    }

    static bool ReceiveClientHealth(PlayerIdentity sender, ParamsReadContext ctx)
    {
        if (!sender)
        {
            return false;
        }
        int schemaVersion;
        string nonce;
        DBAClientCollectorHealth health = new DBAClientCollectorHealth;
        bool valid = ctx.Read(schemaVersion);
        valid = ctx.Read(nonce) && valid;
        valid = ctx.Read(health.samples_captured) && valid;
        valid = ctx.Read(health.batches_attempted) && valid;
        valid = ctx.Read(health.edges_attempted) && valid;
        valid = ctx.Read(health.dropped_samples) && valid;
        valid = ctx.Read(health.queued_samples) && valid;
        if (!valid || schemaVersion != DBAProbeConstants.SCHEMA_VERSION || nonce != s_ServerNonce)
        {
            return false;
        }
        health.player_id = sender.GetId();
        health.server_receive_time_ms = GetGame().GetTime();
        if (s_ClientHealth.Count() >= DBAProbeConstants.MAX_QUEUED_CLIENT_HEALTH)
        {
            s_ClientHealth.RemoveOrdered(0);
        }
        s_ClientHealth.Insert(health);
        return true;
    }

    static void DrainClientHealth(notnull array<ref DBAClientCollectorHealth> output)
    {
        while (s_ClientHealth.Count() > 0)
        {
            output.Insert(s_ClientHealth.Get(0));
            s_ClientHealth.RemoveOrdered(0);
        }
    }

    protected static bool IsDBAProbeDecisionEdge(string edgeType)
    {
        return edgeType == "WEAPON_RAISED" || edgeType == "ADS_ENTERED" || edgeType == "OPTICS_ENTERED" || edgeType == "DELIBERATE_CAMERA_TURN" || edgeType == "SHOT_FIRED_CLIENT";
    }

    protected static bool IsDBAProbeVectorPlausible(vector value)
    {
        return Math.AbsFloat(value[0]) <= 1000000.0 && Math.AbsFloat(value[1]) <= 1000000.0 && Math.AbsFloat(value[2]) <= 1000000.0;
    }

    static void QueueCombatEvent(DBAProbeCombatEvent eventData)
    {
        if (!eventData)
        {
            return;
        }

        if (s_CombatEvents.Count() >= DBAProbeConstants.MAX_QUEUED_COMBAT_EVENTS)
        {
            s_CombatEvents.RemoveOrdered(0);
        }
        s_CombatEvents.Insert(eventData);
    }

    static void DrainCombatEvents(notnull array<ref DBAProbeCombatEvent> output)
    {
        while (s_CombatEvents.Count() > 0)
        {
            output.Insert(s_CombatEvents.Get(0));
            s_CombatEvents.RemoveOrdered(0);
        }
    }
};

class DBAProbeJson
{
    static string Quote(string value)
    {
        JsonSerializer serializer = new JsonSerializer;
        string output;
        if (!serializer.WriteToString(value, false, output))
        {
            return "";
        }
        return output;
    }

    static string Bool(bool value)
    {
        if (value)
        {
            return "true";
        }
        return "false";
    }

    static string VectorArray(vector value)
    {
        return "[" + value[0].ToString() + "," + value[1].ToString() + "," + value[2].ToString() + "]";
    }
};

modded class DayZGame
{
    override void OnRPC(PlayerIdentity sender, Object target, int rpc_type, ParamsReadContext ctx)
    {
        if (rpc_type == DBAProbeConstants.RPC_SERVER_NONCE && !GetGame().IsServer())
        {
            int schemaVersion;
            string nonce;
            if (ctx.Read(schemaVersion) && ctx.Read(nonce) && schemaVersion == DBAProbeConstants.SCHEMA_VERSION)
            {
                DBAProbeRuntime.SetClientNonce(nonce);
            }
            return;
        }

        if (rpc_type == DBAProbeConstants.RPC_CLIENT_SAMPLE_BATCH && GetGame().IsServer())
        {
            bool accepted = DBAProbeRuntime.ReceiveClientBatch(sender, ctx);
            DBAProbeRuntime.RecordClientRPCResult(accepted);
#ifdef DIAG
            if (accepted)
            {
                DBAProbeRuntime.MarkDiagClientTestPassed();
            }
#endif
            return;
        }

        if (rpc_type == DBAProbeConstants.RPC_CLOCK_CHALLENGE && !GetGame().IsServer())
        {
            int clockSchemaVersion;
            string clockNonce;
            int challengeSequence;
            int serverSendMS;
            if (ctx.Read(clockSchemaVersion) && ctx.Read(clockNonce) && ctx.Read(challengeSequence) && ctx.Read(serverSendMS) && clockSchemaVersion == DBAProbeConstants.SCHEMA_VERSION)
            {
                int clientReceiveMS = GetGame().GetTime();
                ScriptRPC response = new ScriptRPC;
                response.Write(DBAProbeConstants.SCHEMA_VERSION);
                response.Write(clockNonce);
                response.Write(challengeSequence);
                response.Write(serverSendMS);
                response.Write(clientReceiveMS);
                response.Write(GetGame().GetTime());
                response.Send(null, DBAProbeConstants.RPC_CLOCK_RESPONSE, true, null);
            }
            return;
        }

        if (rpc_type == DBAProbeConstants.RPC_DECISION_EDGE && GetGame().IsServer())
        {
            DBAProbeRuntime.RecordClientRPCResult(DBAProbeRuntime.ReceiveDecisionEdge(sender, ctx));
            return;
        }

        if (rpc_type == DBAProbeConstants.RPC_CLOCK_RESPONSE && GetGame().IsServer())
        {
            DBAProbeRuntime.RecordClientRPCResult(DBAProbeRuntime.ReceiveClockResponse(sender, ctx));
            return;
        }

        if (rpc_type == DBAProbeConstants.RPC_CLIENT_HEALTH && GetGame().IsServer())
        {
            DBAProbeRuntime.RecordClientRPCResult(DBAProbeRuntime.ReceiveClientHealth(sender, ctx));
            return;
        }

        super.OnRPC(sender, target, rpc_type, ctx);
    }
};
