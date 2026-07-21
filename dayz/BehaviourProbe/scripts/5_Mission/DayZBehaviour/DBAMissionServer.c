modded class MissionServer
{
    protected const string DBA_CONFIG_PATH = "$profile:DayZBehaviourProbe/config.json";

    protected ref DBAProbeServerConfig m_DBAProbeConfig;
    protected ref DBAProbeExporter m_DBAProbeExporter;
    protected string m_DBAProbeServerSessionID;
    protected float m_DBAProbeSnapshotAccumulator;
    protected int m_DBAProbeServerSequence;
    protected ref array<ref DBAProbeRequest> m_DBARandomOpportunityQueue = new array<ref DBAProbeRequest>;
    protected ref array<ref DBAProbeRequest> m_DBAEventEnrichmentQueue = new array<ref DBAProbeRequest>;
    protected ref map<string, int> m_DBANextRandomOpportunityByPlayer = new map<string, int>;
    protected int m_DBARandomOpportunityDrops;
    protected int m_DBAEventEnrichmentDrops;
    protected int m_DBAClockChallengeSequence;
    protected int m_DBALastClockChallengeMS;
    protected ref map<string, vector> m_DBAPreviousPositionByPlayer = new map<string, vector>;
    protected ref map<string, int> m_DBAPreviousPositionTimeByPlayer = new map<string, int>;
    protected ref map<string, bool> m_DBAPreviousMovingByPlayer = new map<string, bool>;
    protected ref map<string, string> m_DBAPlayerSessionByID = new map<string, string>;
    protected int m_DBAPlayerSessionSequence;
    protected int m_DBALastHealthEventMS;

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
            DBAProbeWirePayload healthPayload = new DBAProbeWirePayload;
            healthPayload.scheduler_load_state = "starting";
            healthPayload.sampling_policy_version = "dual-stream-v1";
            healthPayload.visibility_policy_version = "head-origin-safe-v1";
            DBAProbeWireEvent healthEvent = BuildDBAProbeEvent("SERVER_COLLECTOR_HEALTH", "server", "", 0, 0, healthPayload);
            healthEvent.source_authority = "C";
            m_DBAProbeExporter.QueueEvent(healthEvent);
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
            DrainDBAClockAlignments();
            DrainDBAClientHealth();
            ScheduleDBARandomOpportunities();
            SendDBAClockChallenges();
        }

        ScheduleDBAEventEnrichment();
        DrainDBAProbeQueues();
        EmitDBACollectorHealth();
        m_DBAProbeExporter.Update(GetGame().GetTime());
    }

    protected void EmitDBACollectorHealth()
    {
        int nowMS = GetGame().GetTime();
        if (nowMS - m_DBALastHealthEventMS < 30000)
        {
            return;
        }
        m_DBALastHealthEventMS = nowMS;
        DBAProbeWirePayload payload = new DBAProbeWirePayload;
        payload.scheduler_load_state = "normal";
        payload.sampling_policy_version = "dual-stream-v1";
        payload.visibility_policy_version = "head-origin-safe-v1";
        payload.pending_export_events = m_DBAProbeExporter.GetPendingEvents();
        payload.dropped_export_events = m_DBAProbeExporter.GetDroppedEvents();
        payload.export_success_count = m_DBAProbeExporter.GetExportSuccessCount();
        payload.export_failure_count = m_DBAProbeExporter.GetExportFailureCount();
        payload.spool_overwrite_count = m_DBAProbeExporter.GetSpoolOverwriteCount();
        payload.random_opportunity_queue_depth = m_DBARandomOpportunityQueue.Count();
        payload.event_enrichment_queue_depth = m_DBAEventEnrichmentQueue.Count();
        payload.random_opportunity_drop_count = m_DBARandomOpportunityDrops;
        payload.event_enrichment_drop_count = m_DBAEventEnrichmentDrops;
        payload.accepted_client_rpc_count = DBAProbeRuntime.GetAcceptedClientRPCCount();
        payload.rejected_client_rpc_count = DBAProbeRuntime.GetRejectedClientRPCCount();
        DBAProbeWireEvent eventData = BuildDBAProbeEvent("SERVER_COLLECTOR_HEALTH", "server", "", 0, 0, payload);
        eventData.source_authority = "C";
        m_DBAProbeExporter.QueueEvent(eventData);
    }

    override void OnClientPrepareEvent(PlayerIdentity identity, out bool useDB, out vector pos, out float yaw, out int preloadTimeout)
    {
        super.OnClientPrepareEvent(identity, useDB, pos, yaw, preloadTimeout);
        EmitDBAPlayerLifecycle("PLAYER_CONNECTED", identity, null, false, 0);
    }

    override void OnClientReadyEvent(PlayerIdentity identity, PlayerBase player)
    {
        super.OnClientReadyEvent(identity, player);
        EmitDBAPlayerLifecycle("PLAYER_READY", identity, player, false, 0);
        SendDBAClientNonce(identity);
    }

    override PlayerBase OnClientNewEvent(PlayerIdentity identity, vector pos, ParamsReadContext ctx)
    {
        PlayerBase player = super.OnClientNewEvent(identity, pos, ctx);
        if (player)
        {
            EmitDBAPlayerLifecycle("PLAYER_READY", identity, player, false, 0);
            SendDBAClientNonce(identity);
        }
        return player;
    }

    override void OnClientRespawnEvent(PlayerIdentity identity, PlayerBase player)
    {
        super.OnClientRespawnEvent(identity, player);
        RotateDBAPlayerSession(identity);
        EmitDBAPlayerLifecycle("PLAYER_RESPAWNED", identity, player, false, 0);
    }

    override void OnClientReconnectEvent(PlayerIdentity identity, PlayerBase player)
    {
        super.OnClientReconnectEvent(identity, player);
        RotateDBAPlayerSession(identity);
        EmitDBAPlayerLifecycle("PLAYER_RECONNECTED", identity, player, false, 0);
        SendDBAClientNonce(identity);
    }

    protected void SendDBAClientNonce(PlayerIdentity identity)
    {
        if (!identity || !m_DBAProbeConfig || !m_DBAProbeConfig.enabled)
        {
            return;
        }

        ScriptRPC rpc = new ScriptRPC;
        rpc.Write(DBAProbeConstants.SCHEMA_VERSION);
        rpc.Write(m_DBAProbeServerSessionID);
        rpc.Send(null, DBAProbeConstants.RPC_SERVER_NONCE, true, identity);
    }

    override void OnClientDisconnectedEvent(PlayerIdentity identity, PlayerBase player, int logoutTime, bool authFailed)
    {
        EmitDBAPlayerLifecycle("PLAYER_DISCONNECTED", identity, player, authFailed, logoutTime);
        if (identity)
        {
            m_DBAPlayerSessionByID.Remove(identity.GetId());
        }
        super.OnClientDisconnectedEvent(identity, player, logoutTime, authFailed);
    }

    protected void EmitDBAPlayerLifecycle(string eventType, PlayerIdentity identity, PlayerBase player, bool authFailed, int logoutTime)
    {
        if (!m_DBAProbeConfig || !m_DBAProbeConfig.enabled || !m_DBAProbeExporter || !identity)
        {
            return;
        }

        DBAProbeWirePayload payload = new DBAProbeWirePayload;
        payload.lifecycle_state = eventType;
        payload.authentication_failed = authFailed;
        payload.logout_time_seconds = logoutTime;
        if (player)
        {
            payload.position = player.GetPosition();
            payload.orientation = player.GetOrientation();
            payload.alive = player.IsAlive();
            payload.unconscious = player.IsUnconscious();
        }
        string playerSessionID = GetDBAPlayerSessionID(identity.GetId());
        m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent(eventType, "server", playerSessionID, 0, 0, payload));
    }

    protected string GetDBAPlayerSessionID(string playerID)
    {
        string playerSessionID;
        if (m_DBAPlayerSessionByID.Find(playerID, playerSessionID))
        {
            return playerSessionID;
        }
        playerSessionID = m_DBAProbeServerSessionID + ":" + (++m_DBAPlayerSessionSequence).ToString() + ":" + playerID;
        m_DBAPlayerSessionByID.Set(playerID, playerSessionID);
        return playerSessionID;
    }

    protected void RotateDBAPlayerSession(PlayerIdentity identity)
    {
        if (!identity)
        {
            return;
        }
        m_DBAPlayerSessionByID.Remove(identity.GetId());
        GetDBAPlayerSessionID(identity.GetId());
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
            string itemType = "";
            if (itemInHands)
            {
                itemType = itemInHands.GetType();
            }
            string playerID = player.GetIdentity().GetId();
            string playerSessionID = GetDBAPlayerSessionID(playerID);

            DBAProbeWirePayload payload = new DBAProbeWirePayload;
            payload.position = player.GetPosition();
            payload.orientation = player.GetOrientation();
            payload.alive = player.IsAlive();
            payload.unconscious = player.IsUnconscious();
            payload.item_in_hands_type_id = itemType;
            PopulateDBAMovementFields(playerID, player.GetPosition(), payload);

            m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent("PLAYER_SNAPSHOT", "server", playerSessionID, 0, 0, payload));
        }
    }

    protected void DrainDBAProbeClientBatches()
    {
        array<ref DBAReceivedClientBatch> batches = new array<ref DBAReceivedClientBatch>;
        DBAProbeRuntime.DrainClientBatches(batches);

        foreach (DBAReceivedClientBatch batch : batches)
        {
            string playerSessionID = GetDBAPlayerSessionID(batch.player_id);
            foreach (DBAClientSample sample : batch.samples)
            {
                DBAProbeWirePayload payload = new DBAProbeWirePayload;
                payload.camera_position = sample.camera_position;
                payload.camera_direction = sample.camera_direction;
                payload.client_observed_player_position = sample.client_observed_player_position;
                payload.is_weapon_raised = sample.is_weapon_raised;
                payload.is_in_ironsights = sample.is_in_ironsights;
                payload.is_in_optics = sample.is_in_optics;
                payload.is_in_third_person = sample.is_in_third_person;
                payload.item_in_hands_type_id = sample.item_in_hands_type_id;
                payload.sample_mode = sample.sample_mode;
                payload.local_shot_count = sample.local_shot_count;
                payload.local_shot_muzzle_index = sample.local_shot_muzzle_index;

                m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent("CAMERA_SAMPLE", "client_untrusted", playerSessionID, sample.client_sequence, sample.client_monotonic_time_ms, payload));
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
                playerSessionID = GetDBAPlayerSessionID(eventData.target_player_id);
            }

            DBAProbeWirePayload payload = new DBAProbeWirePayload;
            payload.source_player_id = eventData.source_player_id;
            payload.source_type = eventData.source_type;
            payload.damage_type = eventData.damage_type;
            payload.component = eventData.component;
            payload.damage_zone = eventData.damage_zone;
            payload.ammo = eventData.ammo;
            payload.model_position = eventData.model_position;
            payload.speed_coefficient = eventData.speed_coefficient;

            m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent(eventData.event_type, "server", playerSessionID, 0, 0, payload));
        }
    }

    protected void ScheduleDBARandomOpportunities()
    {
        if (!m_DBAProbeConfig.enable_visibility_probe)
        {
            return;
        }

        array<Man> players = new array<Man>;
        GetGame().GetPlayers(players);
        int nowMS = GetGame().GetTime();

        foreach (Man man : players)
        {
            PlayerBase observer = PlayerBase.Cast(man);
            if (!observer || !observer.GetIdentity() || !observer.IsAlive() || observer.IsUnconscious())
            {
                continue;
            }

            string observerID = observer.GetIdentity().GetId();
            int nextOpportunityMS;
            if (!m_DBANextRandomOpportunityByPlayer.Find(observerID, nextOpportunityMS))
            {
                m_DBANextRandomOpportunityByPlayer.Set(observerID, BuildDBANextOpportunityTime(nowMS));
                continue;
            }
            if (nowMS < nextOpportunityMS)
            {
                continue;
            }
            m_DBANextRandomOpportunityByPlayer.Set(observerID, BuildDBANextOpportunityTime(nowMS));

            array<PlayerBase> candidates = new array<PlayerBase>;
            BuildDBAEligibleTargets(observer, players, candidates);
            if (candidates.Count() == 0)
            {
                continue;
            }

            DBAProbeRequest request = new DBAProbeRequest;
            request.observer = observer;
            request.target = candidates.Get(Math.RandomInt(0, candidates.Count()));
            request.sampling_stream = "random_opportunity";
            request.sampling_reason = "prospective_random_trigger";
            request.observer_eligible_count = players.Count();
            request.observer_inclusion_probability = 1.0;
            request.target_eligible_count = candidates.Count();
            request.target_inclusion_probability = 1.0 / candidates.Count();
            request.risk_set_complete = true;
            request.queue_admission_probability = 1.0;
            request.queued_ms = nowMS;
            if (m_DBARandomOpportunityQueue.Count() >= m_DBAProbeConfig.max_random_opportunity_queue)
            {
                m_DBARandomOpportunityDrops++;
                continue;
            }
            m_DBARandomOpportunityQueue.Insert(request);
        }
    }

    protected void PopulateDBAMovementFields(string playerID, vector currentPosition, DBAProbeWirePayload payload)
    {
        int nowMS = GetGame().GetTime();
        vector previousPosition;
        int previousMS;
        bool wasMoving;
        if (m_DBAPreviousPositionByPlayer.Find(playerID, previousPosition) && m_DBAPreviousPositionTimeByPlayer.Find(playerID, previousMS))
        {
            int elapsedMS = nowMS - previousMS;
            if (elapsedMS > 0)
            {
                vector displacement = currentPosition - previousPosition;
                float distance = displacement.Length();
                payload.movement_speed_mps = distance * 1000.0 / elapsedMS;
                if (distance >= 0.1)
                {
                    payload.movement_heading = displacement.Normalized();
                }
                bool moving = payload.movement_speed_mps >= 0.25;
                payload.movement_state = "STOPPED";
                if (moving)
                {
                    payload.movement_state = "MOVING";
                }
                if (m_DBAPreviousMovingByPlayer.Find(playerID, wasMoving) && wasMoving != moving)
                {
                    payload.movement_transition = "MOVEMENT_STOPPED";
                    if (moving)
                    {
                        payload.movement_transition = "MOVEMENT_STARTED";
                    }
                }
                m_DBAPreviousMovingByPlayer.Set(playerID, moving);
            }
        }
        m_DBAPreviousPositionByPlayer.Set(playerID, currentPosition);
        m_DBAPreviousPositionTimeByPlayer.Set(playerID, nowMS);
    }

    protected void SendDBAClockChallenges()
    {
        int nowMS = GetGame().GetTime();
        int intervalMS = m_DBAProbeConfig.clock_challenge_interval_seconds * 1000;
        if (intervalMS < 1000)
        {
            intervalMS = 1000;
        }
        if (nowMS - m_DBALastClockChallengeMS < intervalMS)
        {
            return;
        }
        m_DBALastClockChallengeMS = nowMS;

        array<Man> players = new array<Man>;
        GetGame().GetPlayers(players);
        foreach (Man man : players)
        {
            PlayerBase player = PlayerBase.Cast(man);
            if (!player || !player.GetIdentity())
            {
                continue;
            }
            ScriptRPC challenge = new ScriptRPC;
            challenge.Write(DBAProbeConstants.SCHEMA_VERSION);
            challenge.Write(m_DBAProbeServerSessionID);
            challenge.Write(++m_DBAClockChallengeSequence);
            challenge.Write(nowMS);
            challenge.Send(null, DBAProbeConstants.RPC_CLOCK_CHALLENGE, true, player.GetIdentity());
        }
    }

    protected void DrainDBAClockAlignments()
    {
        array<ref DBAClockAlignment> samples = new array<ref DBAClockAlignment>;
        DBAProbeRuntime.DrainClockSamples(samples);
        foreach (DBAClockAlignment sample : samples)
        {
            DBAProbeWirePayload payload = new DBAProbeWirePayload;
            payload.clock_server_send_ms = sample.server_send_ms;
            payload.clock_client_receive_ms = sample.client_receive_ms;
            payload.clock_client_send_ms = sample.client_send_ms;
            payload.clock_server_receive_ms = sample.server_receive_ms;
            payload.clock_round_trip_ms = sample.round_trip_ms;
            payload.clock_offset_estimate_ms = sample.offset_estimate_ms;
            payload.clock_uncertainty_ms = sample.uncertainty_ms;
            string playerSessionID = GetDBAPlayerSessionID(sample.player_id);
            DBAProbeWireEvent eventData = BuildDBAProbeEvent("CLOCK_ALIGNMENT", "client_untrusted", playerSessionID, sample.challenge_sequence, sample.client_send_ms, payload);
            eventData.source_authority = "C";
            eventData.server_receive_ms = sample.server_receive_ms;
            m_DBAProbeExporter.QueueEvent(eventData);
        }
    }

    protected void DrainDBAClientHealth()
    {
        array<ref DBAClientCollectorHealth> samples = new array<ref DBAClientCollectorHealth>;
        DBAProbeRuntime.DrainClientHealth(samples);
        foreach (DBAClientCollectorHealth sample : samples)
        {
            DBAProbeWirePayload payload = new DBAProbeWirePayload;
            payload.client_samples_captured = sample.samples_captured;
            payload.client_batches_attempted = sample.batches_attempted;
            payload.client_edges_attempted = sample.edges_attempted;
            payload.client_dropped_samples = sample.dropped_samples;
            payload.client_queued_samples = sample.queued_samples;
            string playerSessionID = GetDBAPlayerSessionID(sample.player_id);
            DBAProbeWireEvent eventData = BuildDBAProbeEvent("CLIENT_COLLECTOR_HEALTH", "client_untrusted", playerSessionID, 0, 0, payload);
            eventData.source_authority = "C";
            eventData.server_receive_ms = sample.server_receive_time_ms;
            m_DBAProbeExporter.QueueEvent(eventData);
        }
    }

    protected void ScheduleDBAEventEnrichment()
    {
        array<ref DBADecisionEdge> edges = new array<ref DBADecisionEdge>;
        DBAProbeRuntime.DrainDecisionEdges(edges);
        if (edges.Count() == 0)
        {
            return;
        }

        array<Man> players = new array<Man>;
        GetGame().GetPlayers(players);
        foreach (DBADecisionEdge edge : edges)
        {
            string playerSessionID = GetDBAPlayerSessionID(edge.player_id);
            DBAProbeWirePayload edgePayload = new DBAProbeWirePayload;
            edgePayload.sampling_reason = edge.edge_type;
            edgePayload.camera_direction = edge.camera_direction;
            DBAProbeWireEvent edgeEvent = BuildDBAProbeEvent("DECISION_EDGE", "client_untrusted", playerSessionID, edge.edge_sequence, edge.client_monotonic_time_ms, edgePayload);
            edgeEvent.server_receive_ms = edge.server_receive_time_ms;
            edgeEvent.server_time_ms = edge.server_receive_time_ms;
            m_DBAProbeExporter.QueueEvent(edgeEvent);
            if (!m_DBAProbeConfig.enable_visibility_probe)
            {
                continue;
            }

            PlayerBase observer = FindDBAPlayerByID(players, edge.player_id);
            if (!observer)
            {
                continue;
            }
            array<PlayerBase> candidates = new array<PlayerBase>;
            BuildDBAEligibleTargets(observer, players, candidates);
            if (candidates.Count() == 0)
            {
                continue;
            }

            DBAProbeRequest request = new DBAProbeRequest;
            request.observer = observer;
            request.target = candidates.Get(Math.RandomInt(0, candidates.Count()));
            request.sampling_stream = "event_enrichment";
            request.sampling_reason = edge.edge_type;
            request.observer_eligible_count = players.Count();
            request.observer_inclusion_probability = 1.0;
            request.target_eligible_count = candidates.Count();
            request.target_inclusion_probability = 1.0 / candidates.Count();
            request.risk_set_complete = false;
            request.queue_admission_probability = 1.0;
            request.queued_ms = edge.server_receive_time_ms;
            if (m_DBAEventEnrichmentQueue.Count() >= m_DBAProbeConfig.max_event_enrichment_queue)
            {
                m_DBAEventEnrichmentDrops++;
                continue;
            }
            m_DBAEventEnrichmentQueue.Insert(request);
        }
    }

    protected void DrainDBAProbeQueues()
    {
        if (!m_DBAProbeConfig.enable_visibility_probe || m_DBAProbeConfig.max_visibility_pairs_per_tick <= 0)
        {
            return;
        }

        int completed;
        while (completed < m_DBAProbeConfig.max_visibility_pairs_per_tick)
        {
            DBAProbeRequest request;
            if (m_DBARandomOpportunityQueue.Count() > 0)
            {
                request = m_DBARandomOpportunityQueue.Get(0);
                m_DBARandomOpportunityQueue.RemoveOrdered(0);
            }
            else if (m_DBAEventEnrichmentQueue.Count() > 0)
            {
                request = m_DBAEventEnrichmentQueue.Get(0);
                m_DBAEventEnrichmentQueue.RemoveOrdered(0);
            }
            else
            {
                return;
            }

            int nowMS = GetGame().GetTime();
            if (!request || !request.observer || !request.target || nowMS - request.queued_ms > m_DBAProbeConfig.max_probe_queue_age_ms)
            {
                continue;
            }
            if (request.not_before_ms > nowMS)
            {
                if (request.sampling_stream == "random_opportunity")
                {
                    m_DBARandomOpportunityQueue.Insert(request);
                }
                else
                {
                    m_DBAEventEnrichmentQueue.Insert(request);
                }
                return;
            }
            EmitDBAVisibilityObservation(request, nowMS);
            completed++;
        }
    }

    protected void EmitDBAVisibilityObservation(DBAProbeRequest request, int startedMS)
    {
        DBAVisibilityProbeResult result = DBAVisibilityProbe.Probe(request.observer, request.target);
        int completedMS = GetGame().GetTime();
        DBAProbeWirePayload payload = new DBAProbeWirePayload;
        payload.observer_player_id = request.observer.GetIdentity().GetId();
        payload.target_player_id = request.target.GetIdentity().GetId();
        payload.observer_player_session_id = GetDBAPlayerSessionID(payload.observer_player_id);
        payload.target_player_session_id = GetDBAPlayerSessionID(payload.target_player_id);
        payload.classification = result.GetClassificationName();
        payload.observer_origin_mode = result.observer_origin_mode;
        payload.observer_origin = result.observer_origin;
        payload.target_head = result.target_head;
        payload.target_torso = result.target_torso;
        payload.blocker_type = result.blocker_type;
        payload.probe_count = result.probe_count;
        payload.points_attempted = result.probe_count;
        payload.points_clear = result.points_clear;
        payload.points_hard_blocked = result.points_hard_blocked;
        payload.points_ambiguous = result.points_ambiguous;
        payload.head_point_result = result.head_result;
        payload.torso_point_result = result.torso_result;
        payload.pelvis_point_result = result.pelvis_result;
        payload.left_upper_point_result = result.left_upper_result;
        payload.right_upper_point_result = result.right_upper_result;
        payload.bone_validity = result.bone_validity;
        payload.sampling_stream = request.sampling_stream;
        payload.sampling_policy_version = "dual-stream-v1";
        payload.sampling_reason = request.sampling_reason;
        payload.observer_eligible_count = request.observer_eligible_count;
        payload.observer_inclusion_probability = request.observer_inclusion_probability;
        payload.target_eligible_count = request.target_eligible_count;
        payload.target_inclusion_probability = request.target_inclusion_probability;
        payload.risk_set_definition = "alive_nonself_within_radius";
        payload.risk_set_complete = request.risk_set_complete;
        payload.queue_admission_probability = request.queue_admission_probability;
        payload.scheduler_load_state = "normal";
        payload.queue_delay_ms = startedMS - request.queued_ms;
        payload.visibility_policy_version = "head-origin-safe-v1";
        payload.probe_queued_ms = request.queued_ms;
        payload.probe_started_ms = startedMS;
        payload.probe_completed_ms = completedMS;
        string classification = result.GetClassificationName();
        if (classification == "HEAD_ORIGIN_OCCLUDED" && m_DBAProbeConfig.visibility_origin_mode == "FIRST_PERSON_EYE" && m_DBAProbeConfig.visibility_validation_id != "")
        {
            if (request.first_occluded_ms == 0)
            {
                request.first_occluded_ms = completedMS;
                request.not_before_ms = completedMS + m_DBAProbeConfig.minimum_occlusion_duration_ms;
                if (request.sampling_stream == "random_opportunity")
                {
                    m_DBARandomOpportunityQueue.Insert(request);
                }
                else
                {
                    m_DBAEventEnrichmentQueue.Insert(request);
                }
                return;
            }
            payload.occlusion_duration_ms = completedMS - request.first_occluded_ms;
            if (payload.occlusion_duration_ms >= m_DBAProbeConfig.minimum_occlusion_duration_ms)
            {
                classification = "ROBUSTLY_OCCLUDED";
            }
        }
        payload.classification = classification;
        payload.observer_origin_mode = "PLAYER_HEAD_APPROXIMATION";
        if (m_DBAProbeConfig.visibility_origin_mode == "FIRST_PERSON_EYE" && m_DBAProbeConfig.visibility_validation_id != "")
        {
            payload.observer_origin_mode = "FIRST_PERSON_EYE";
        }
        payload.visibility_validation_id = m_DBAProbeConfig.visibility_validation_id;
        GetGame().GetWorldName(payload.map_id);
        vector observerPosition = request.observer.GetPosition();
        int areaCellX = Math.Floor(observerPosition[0] / 100.0);
        int areaCellZ = Math.Floor(observerPosition[2] / 100.0);
        payload.area_cell = areaCellX.ToString() + ":" + areaCellZ.ToString();
        payload.target_distance_metres = vector.Distance(observerPosition, request.target.GetPosition());
        payload.observer_speed_mps = GetVelocity(request.observer).Length();
        HumanMovementState movementState = new HumanMovementState;
        request.observer.GetMovementState(movementState);
        payload.observer_stance_id = movementState.m_iStanceIdx;
        payload.baseline_weapon_raised = request.observer.IsRaised();
        payload.baseline_weapon_state = "LOWERED";
        if (payload.baseline_weapon_raised)
        {
            payload.baseline_weapon_state = "RAISED";
        }
        payload.camera_mode = "UNKNOWN_SERVER_SIDE";
        array<Man> population = new array<Man>;
        GetGame().GetPlayers(population);
        payload.server_population_count = population.Count();

        string observerSessionID = payload.observer_player_session_id;
        m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent("VISIBILITY_OBSERVATION", "server", observerSessionID, 0, 0, payload));
    }

    protected void BuildDBAEligibleTargets(PlayerBase observer, notnull array<Man> players, notnull array<PlayerBase> output)
    {
        foreach (Man man : players)
        {
            PlayerBase target = PlayerBase.Cast(man);
            if (!target || !target.GetIdentity() || target == observer || !target.IsAlive() || target.IsUnconscious())
            {
                continue;
            }
            if (vector.Distance(observer.GetPosition(), target.GetPosition()) <= m_DBAProbeConfig.visibility_radius_metres)
            {
                output.Insert(target);
            }
        }
    }

    protected PlayerBase FindDBAPlayerByID(notnull array<Man> players, string playerID)
    {
        foreach (Man man : players)
        {
            PlayerBase player = PlayerBase.Cast(man);
            if (player && player.GetIdentity() && player.GetIdentity().GetId() == playerID)
            {
                return player;
            }
        }
        return null;
    }

    protected int BuildDBANextOpportunityTime(int nowMS)
    {
        int minimumMS = m_DBAProbeConfig.random_opportunity_min_seconds * 1000;
        int maximumMS = m_DBAProbeConfig.random_opportunity_max_seconds * 1000;
        if (maximumMS <= minimumMS)
        {
            maximumMS = minimumMS + 1;
        }
        return nowMS + Math.RandomInt(minimumMS, maximumMS);
    }

    protected DBAProbeWireEvent BuildDBAProbeEvent(string eventType, string source, string playerSessionID, int clientSequence, int clientTimeMS, DBAProbeWirePayload payload)
    {
        int serverSequence = ++m_DBAProbeServerSequence;
        DBAProbeWireEvent eventData = new DBAProbeWireEvent;
        eventData.event_type = eventType;
        eventData.source = source;
        eventData.source_authority = "A";
        eventData.source_component = "dayz_server";
        if (source == "client_untrusted")
        {
            eventData.source_authority = "B";
            eventData.source_component = "dayz_client";
        }
        eventData.source_schema_version = DBAProbeConstants.SCHEMA_VERSION;
        eventData.collector_version = "milestone-0.1";
        eventData.server_sequence = serverSequence;
        eventData.server_time_ms = GetGame().GetTime();
        eventData.server_receive_ms = eventData.server_time_ms;
        eventData.player_session_id = playerSessionID;
        eventData.client_sequence = clientSequence;
        eventData.client_monotonic_time_ms = clientTimeMS;
        eventData.payload = payload;
        eventData.source_event_id = m_DBAProbeServerSessionID + ":" + serverSequence.ToString();
        return eventData;
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

        string sessionID = year.ToString() + month.ToString() + day.ToString();
        sessionID += "-" + hour.ToString() + minute.ToString() + second.ToString();
        sessionID += "-" + GetGame().GetTime().ToString();
        sessionID += "-" + Math.RandomInt(100000, 999999).ToString();
        return sessionID;
    }
};
