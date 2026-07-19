modded class MissionServer
{
    protected const string DBA_CONFIG_PATH = "$profile:DayZBehaviourProbe/config.json";

    protected ref DBAProbeServerConfig m_DBAProbeConfig;
    protected ref DBAProbeExporter m_DBAProbeExporter;
    protected string m_DBAProbeServerSessionID;
    protected float m_DBAProbeSnapshotAccumulator;
    protected int m_DBAProbeServerSequence;

    override void OnInit()
    {
        super.OnInit();

        m_DBAProbeConfig = LoadDBAProbeConfig();
        m_DBAProbeServerSessionID = BuildDBAProbeSessionID();
        DBAProbeRuntime.SetServerNonce(m_DBAProbeServerSessionID);

        if (m_DBAProbeConfig.enabled)
        {
            m_DBAProbeExporter = new DBAProbeExporter(m_DBAProbeConfig, m_DBAProbeServerSessionID);
            Print("[DayZBehaviourProbe] enabled session=" + m_DBAProbeServerSessionID);
        }
    }

    override void OnUpdate(float timeslice)
    {
        super.OnUpdate(timeslice);

        if (!m_DBAProbeConfig || !m_DBAProbeConfig.enabled || !m_DBAProbeExporter)
        {
            return;
        }

        m_DBAProbeSnapshotAccumulator += timeslice;
        if (m_DBAProbeSnapshotAccumulator >= m_DBAProbeConfig.server_snapshot_interval_seconds)
        {
            m_DBAProbeSnapshotAccumulator = 0;
            CaptureDBAProbeServerSnapshots();
            DrainDBAProbeClientBatches();
            DrainDBAProbeCombatEvents();
            CaptureDBAProbeVisibility();
        }

        m_DBAProbeExporter.Update(GetGame().GetTime());
    }

    override void OnEvent(EventType eventTypeId, Param params)
    {
        super.OnEvent(eventTypeId, params);

        if (eventTypeId != ClientReadyEventTypeID || !m_DBAProbeConfig || !m_DBAProbeConfig.enabled)
        {
            return;
        }

        ClientReadyEventParams readyParams;
        Class.CastTo(readyParams, params);
        PlayerIdentity identity = readyParams.param1;
        if (!identity)
        {
            return;
        }

        ScriptRPC rpc = new ScriptRPC;
        rpc.Write(DBAProbeConstants.SCHEMA_VERSION);
        rpc.Write(m_DBAProbeServerSessionID);
        rpc.Send(null, DBAProbeConstants.RPC_SERVER_NONCE, true, identity);
    }

    protected void CaptureDBAProbeServerSnapshots()
    {
        array<Man> players = new array<Man>;
        GetGame().GetPlayers(players);

        foreach (Man man : players)
        {
            PlayerBase player = PlayerBase.Cast(man);
            if (!player || !player.GetIdentity())
            {
                continue;
            }

            EntityAI itemInHands = player.GetItemInHands();
            string itemType = itemInHands ? itemInHands.GetType() : "";
            string playerID = player.GetIdentity().GetId();
            string playerSessionID = m_DBAProbeServerSessionID + ":" + playerID;

            string payload = "{"
                + "\"position\":" + DBAProbeJson.VectorArray(player.GetPosition()) + ","
                + "\"orientation\":" + DBAProbeJson.VectorArray(player.GetOrientation()) + ","
                + "\"alive\":" + DBAProbeJson.Bool(player.IsAlive()) + ","
                + "\"unconscious\":" + DBAProbeJson.Bool(player.IsUnconscious()) + ","
                + "\"item_in_hands_type_id\":" + DBAProbeJson.Quote(itemType)
                + "}";

            m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent(
                "PLAYER_SNAPSHOT",
                "server",
                playerSessionID,
                playerID,
                -1,
                -1,
                payload
            ));
        }
    }

    protected void DrainDBAProbeClientBatches()
    {
        array<ref DBAReceivedClientBatch> batches = new array<ref DBAReceivedClientBatch>;
        DBAProbeRuntime.DrainClientBatches(batches);

        foreach (DBAReceivedClientBatch batch : batches)
        {
            string playerSessionID = m_DBAProbeServerSessionID + ":" + batch.player_id;
            foreach (DBAClientSample sample : batch.samples)
            {
                string payload = "{"
                    + "\"camera_position\":" + DBAProbeJson.VectorArray(sample.camera_position) + ","
                    + "\"camera_direction\":" + DBAProbeJson.VectorArray(sample.camera_direction) + ","
                    + "\"client_observed_player_position\":" + DBAProbeJson.VectorArray(sample.client_observed_player_position) + ","
                    + "\"is_weapon_raised\":" + DBAProbeJson.Bool(sample.is_weapon_raised) + ","
                    + "\"is_in_ironsights\":" + DBAProbeJson.Bool(sample.is_in_ironsights) + ","
                    + "\"is_in_optics\":" + DBAProbeJson.Bool(sample.is_in_optics) + ","
                    + "\"is_in_third_person\":" + DBAProbeJson.Bool(sample.is_in_third_person) + ","
                    + "\"item_in_hands_type_id\":" + DBAProbeJson.Quote(sample.item_in_hands_type_id) + ","
                    + "\"local_shot_count\":" + sample.local_shot_count.ToString() + ","
                    + "\"local_shot_muzzle_index\":" + sample.local_shot_muzzle_index.ToString()
                    + "}";

                m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent(
                    "CAMERA_SAMPLE",
                    "client_untrusted",
                    playerSessionID,
                    batch.player_id,
                    sample.client_sequence,
                    sample.client_monotonic_time_ms,
                    payload
                ));
            }
        }
    }

    protected void DrainDBAProbeCombatEvents()
    {
        array<ref DBAProbeCombatEvent> events = new array<ref DBAProbeCombatEvent>;
        DBAProbeRuntime.DrainCombatEvents(events);

        foreach (DBAProbeCombatEvent eventData : events)
        {
            string playerSessionID = "";
            if (eventData.target_player_id != "")
            {
                playerSessionID = m_DBAProbeServerSessionID + ":" + eventData.target_player_id;
            }

            string payload = "{"
                + "\"source_player_id\":" + DBAProbeJson.Quote(eventData.source_player_id) + ","
                + "\"source_type\":" + DBAProbeJson.Quote(eventData.source_type) + ","
                + "\"damage_type\":" + eventData.damage_type.ToString() + ","
                + "\"component\":" + eventData.component.ToString() + ","
                + "\"damage_zone\":" + DBAProbeJson.Quote(eventData.damage_zone) + ","
                + "\"ammo\":" + DBAProbeJson.Quote(eventData.ammo) + ","
                + "\"model_position\":" + DBAProbeJson.VectorArray(eventData.model_position) + ","
                + "\"speed_coefficient\":" + eventData.speed_coefficient.ToString()
                + "}";

            m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent(
                eventData.event_type,
                "server",
                playerSessionID,
                eventData.target_player_id,
                -1,
                -1,
                payload
            ));
        }
    }

    protected void CaptureDBAProbeVisibility()
    {
        if (!m_DBAProbeConfig.enable_visibility_probe || m_DBAProbeConfig.max_visibility_pairs_per_tick <= 0)
        {
            return;
        }

        array<Man> players = new array<Man>;
        GetGame().GetPlayers(players);
        int completedPairs;

        for (int observerIndex = 0; observerIndex < players.Count(); observerIndex++)
        {
            PlayerBase observer = PlayerBase.Cast(players.Get(observerIndex));
            if (!observer || !observer.GetIdentity())
            {
                continue;
            }

            for (int targetIndex = 0; targetIndex < players.Count(); targetIndex++)
            {
                PlayerBase target = PlayerBase.Cast(players.Get(targetIndex));
                if (!target || !target.GetIdentity() || target == observer)
                {
                    continue;
                }

                if (vector.Distance(observer.GetPosition(), target.GetPosition()) > m_DBAProbeConfig.visibility_radius_metres)
                {
                    continue;
                }

                DBAVisibilityProbeResult result = DBAVisibilityProbe.Probe(observer, target);
                string payload = "{"
                    + "\"observer_player_id\":" + DBAProbeJson.Quote(observer.GetIdentity().GetId()) + ","
                    + "\"target_player_id\":" + DBAProbeJson.Quote(target.GetIdentity().GetId()) + ","
                    + "\"classification\":" + result.classification.ToString() + ","
                    + "\"observer_origin\":" + DBAProbeJson.VectorArray(result.observer_origin) + ","
                    + "\"target_head\":" + DBAProbeJson.VectorArray(result.target_head) + ","
                    + "\"target_torso\":" + DBAProbeJson.VectorArray(result.target_torso) + ","
                    + "\"blocker_type\":" + DBAProbeJson.Quote(result.blocker_type) + ","
                    + "\"probe_count\":" + result.probe_count.ToString()
                    + "}";

                m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent(
                    "VISIBILITY_OBSERVATION",
                    "server",
                    m_DBAProbeServerSessionID + ":" + observer.GetIdentity().GetId(),
                    observer.GetIdentity().GetId(),
                    -1,
                    -1,
                    payload
                ));

                completedPairs++;
                if (completedPairs >= m_DBAProbeConfig.max_visibility_pairs_per_tick)
                {
                    return;
                }
            }
        }
    }

    protected string BuildDBAProbeEvent(
        string eventType,
        string source,
        string playerSessionID,
        string playerID,
        int clientSequence,
        int clientTimeMS,
        string payload
    )
    {
        int serverSequence = ++m_DBAProbeServerSequence;
        string eventJson = "{"
            + "\"event_type\":" + DBAProbeJson.Quote(eventType) + ","
            + "\"source\":" + DBAProbeJson.Quote(source) + ","
            + "\"server_sequence\":" + serverSequence.ToString() + ","
            + "\"server_time_ms\":" + GetGame().GetTime().ToString() + ","
            + "\"player_session_id\":" + DBAProbeJson.Quote(playerSessionID) + ","
            + "\"player_id\":" + DBAProbeJson.Quote(playerID);

        if (clientSequence >= 0)
        {
            eventJson += ",\"client_sequence\":" + clientSequence.ToString();
        }
        if (clientTimeMS >= 0)
        {
            eventJson += ",\"client_monotonic_time_ms\":" + clientTimeMS.ToString();
        }

        eventJson += ",\"payload\":" + payload + "}";
        return eventJson;
    }

    protected DBAProbeServerConfig LoadDBAProbeConfig()
    {
        MakeDirectory("$profile:DayZBehaviourProbe");

        DBAProbeServerConfig config = new DBAProbeServerConfig;
        string errorMessage;
        if (!JsonFileLoader<DBAProbeServerConfig>.LoadFile(DBA_CONFIG_PATH, config, errorMessage))
        {
            Print("[DayZBehaviourProbe] creating default config: " + errorMessage);
            JsonFileLoader<DBAProbeServerConfig>.SaveFile(DBA_CONFIG_PATH, config, errorMessage);
        }
        return config;
    }

    protected string BuildDBAProbeSessionID()
    {
        int year;
        int month;
        int day;
        int hour;
        int minute;
        int second;
        GetYearMonthDayUTC(year, month, day);
        GetHourMinuteSecondUTC(hour, minute, second);

        return string.Format(
            "%1%2%3-%4%5%6-%7-%8",
            year,
            month,
            day,
            hour,
            minute,
            second,
            GetGame().GetTime(),
            Math.RandomInt(100000, 999999)
        );
    }
};
