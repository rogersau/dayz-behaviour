class DBAProbeConstants
{
    static const int SCHEMA_VERSION = 1;

    // Project-owned RPC IDs. Milestone 0 must confirm these do not conflict with
    // the target server's mod set before production use.
    static const int RPC_SERVER_NONCE = 759430;
    static const int RPC_CLIENT_SAMPLE_BATCH = 759431;

    static const int MAX_CLIENT_SAMPLES_PER_BATCH = 32;
    static const int MAX_QUEUED_CLIENT_BATCHES = 64;
    static const int MAX_QUEUED_COMBAT_EVENTS = 128;
};

class DBAProbeServerConfig
{
    bool enabled = true;
    string server_id = "dayz-development";
    string endpoint = "http://127.0.0.1:8080/";
    string ingest_token = "";
    float server_snapshot_interval_seconds = 0.5;
    float export_interval_seconds = 1.0;
    int max_events_per_export = 128;
    int max_pending_events = 512;
    bool enable_visibility_probe = false;
    int max_visibility_pairs_per_tick = 1;
    float visibility_radius_metres = 1000.0;
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

class DBAProbeRuntime
{
    protected static string s_ClientNonce;
    protected static string s_ServerNonce;
    protected static int s_LocalShotCount;
    protected static int s_LocalShotMuzzleIndex = -1;

    protected static ref array<ref DBAReceivedClientBatch> s_ReceivedClientBatches = new array<ref DBAReceivedClientBatch>;
    protected static ref array<ref DBAProbeCombatEvent> s_CombatEvents = new array<ref DBAProbeCombatEvent>;
    protected static ref map<string, int> s_LastBatchSequenceByPlayer = new map<string, int>;

    static void SetClientNonce(string nonce)
    {
        s_ClientNonce = nonce;
        Print("[DayZBehaviourProbe] received server nonce");
    }

    static string GetClientNonce()
    {
        return s_ClientNonce;
    }

    static void SetServerNonce(string nonce)
    {
        s_ServerNonce = nonce;
        s_LastBatchSequenceByPlayer.Clear();
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

        if (schemaVersion != DBAProbeConstants.SCHEMA_VERSION || nonce != s_ServerNonce)
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

        for (int index = 0; index < sampleCount; index++)
        {
            DBAClientSample sample = new DBAClientSample;
            if (!ctx.Read(sample.client_sequence)
                || !ctx.Read(sample.client_monotonic_time_ms)
                || !ctx.Read(sample.camera_position)
                || !ctx.Read(sample.camera_direction)
                || !ctx.Read(sample.client_observed_player_position)
                || !ctx.Read(sample.is_weapon_raised)
                || !ctx.Read(sample.is_in_ironsights)
                || !ctx.Read(sample.is_in_optics)
                || !ctx.Read(sample.is_in_third_person)
                || !ctx.Read(sample.item_in_hands_type_id)
                || !ctx.Read(sample.local_shot_count)
                || !ctx.Read(sample.local_shot_muzzle_index))
            {
                Print("[DayZBehaviourProbe] rejected malformed client sample at index=" + index.ToString());
                return false;
            }

            batch.samples.Insert(sample);
        }

        s_LastBatchSequenceByPlayer.Set(playerID, batchSequence);
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
    static string Escape(string value)
    {
        value.Replace("\\", "\\\\");
        value.Replace("\"", "\\\"");
        value.Replace("\n", "\\n");
        value.Replace("\r", "\\r");
        value.Replace("\t", "\\t");
        return value;
    }

    static string Quote(string value)
    {
        return "\"" + Escape(value) + "\"";
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
    override void OnRPC(PlayerIdentity sender, Object target, int rpcType, ParamsReadContext ctx)
    {
        if (rpcType == DBAProbeConstants.RPC_SERVER_NONCE && !GetGame().IsServer())
        {
            int schemaVersion;
            string nonce;
            if (ctx.Read(schemaVersion) && ctx.Read(nonce) && schemaVersion == DBAProbeConstants.SCHEMA_VERSION)
            {
                DBAProbeRuntime.SetClientNonce(nonce);
            }
            return;
        }

        if (rpcType == DBAProbeConstants.RPC_CLIENT_SAMPLE_BATCH && GetGame().IsServer())
        {
            DBAProbeRuntime.ReceiveClientBatch(sender, ctx);
            return;
        }

        super.OnRPC(sender, target, rpcType, ctx);
    }
};
