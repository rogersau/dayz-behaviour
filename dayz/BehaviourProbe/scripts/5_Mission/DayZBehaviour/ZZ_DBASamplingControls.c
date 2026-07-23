modded class MissionServer
{
    override void ScheduleDBARandomOpportunities()
    {
        if (!m_DBAProbeConfig.enable_visibility_probe)
        {
            return;
        }

        array<Man> players = new array<Man>;
        GetGame().GetPlayers(players);
        int eligibleObserverCount = CountDBAEligibleObservers(players);
        int nowMS = GetGame().GetTime();

        foreach (Man man : players)
        {
            PlayerBase observer = PlayerBase.Cast(man);
            if (!IsDBAOpportunityObserverEligible(observer))
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
                EmitDBANeutralOpportunity(observer, eligibleObserverCount, players.Count(), nowMS);
                continue;
            }

            DBAProbeRequest request = new DBAProbeRequest;
            request.observer = observer;
            request.target = candidates.Get(Math.RandomInt(0, candidates.Count()));
            request.sampling_stream = "random_opportunity";
            request.sampling_reason = "prospective_random_trigger";
            request.observer_eligible_count = eligibleObserverCount;
            request.observer_inclusion_probability = 1.0;
            request.target_eligible_count = candidates.Count();
            request.target_inclusion_probability = 1.0 / candidates.Count();
            request.risk_set_complete = true;
            request.queue_admission_probability = 1.0;
            request.queued_ms = nowMS;
            if (m_DBARandomOpportunityQueue.Count() >= m_DBAProbeConfig.max_random_opportunity_queue)
            {
                m_DBARandomOpportunityDrops++;
                EmitDBADroppedOpportunity(observer, eligibleObserverCount, candidates.Count(), players.Count(), nowMS, "random_opportunity_queue_full");
                continue;
            }
            m_DBARandomOpportunityQueue.Insert(request);
        }
    }

    protected int CountDBAEligibleObservers(notnull array<Man> players)
    {
        int count;
        foreach (Man man : players)
        {
            if (IsDBAOpportunityObserverEligible(PlayerBase.Cast(man)))
            {
                count++;
            }
        }
        return count;
    }

    protected bool IsDBAOpportunityObserverEligible(PlayerBase player)
    {
        return player && player.GetIdentity() && player.IsAlive() && !player.IsUnconscious();
    }

    protected void EmitDBANeutralOpportunity(PlayerBase observer, int eligibleObserverCount, int populationCount, int nowMS)
    {
        DBAProbeWirePayload payload = BuildDBAOpportunityPayload(observer, eligibleObserverCount, populationCount, nowMS);
        payload.classification = "NO_RELEVANT_TARGET";
        payload.control_type = "NEUTRAL_NO_RELEVANT_TARGET";
        payload.sampling_reason = "prospective_no_target_trigger";
        payload.target_eligible_count = 0;
        payload.target_inclusion_probability = 1.0;
        payload.risk_set_definition = "no_alive_nonself_within_radius";
        payload.risk_set_complete = true;
        m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent("SAMPLING_OPPORTUNITY", "server", payload.observer_player_session_id, 0, 0, payload));
    }

    protected void EmitDBADroppedOpportunity(PlayerBase observer, int eligibleObserverCount, int targetCount, int populationCount, int nowMS, string reason)
    {
        DBAProbeWirePayload payload = BuildDBAOpportunityPayload(observer, eligibleObserverCount, populationCount, nowMS);
        payload.classification = "DROPPED";
        payload.control_type = "DROPPED_RANDOM_OPPORTUNITY";
        payload.sampling_reason = "prospective_random_trigger";
        payload.target_eligible_count = targetCount;
        payload.target_inclusion_probability = 1.0 / targetCount;
        payload.queue_admission_probability = 0.0;
        payload.risk_set_definition = "alive_nonself_within_radius";
        payload.risk_set_complete = true;
        payload.drop_reason = reason;
        payload.scheduler_load_state = "random_queue_full";
        m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent("SAMPLING_OPPORTUNITY_DROPPED", "server", payload.observer_player_session_id, 0, 0, payload));
    }

    protected DBAProbeWirePayload BuildDBAOpportunityPayload(PlayerBase observer, int eligibleObserverCount, int populationCount, int nowMS)
    {
        DBAProbeWirePayload payload = new DBAProbeWirePayload;
        payload.observer_player_id = observer.GetIdentity().GetId();
        payload.observer_player_session_id = GetDBAPlayerSessionID(payload.observer_player_id);
        payload.observer_origin_mode = "NOT_APPLICABLE";
        payload.sampling_stream = "random_opportunity";
        payload.sampling_policy_version = "dual-stream-v2-neutral-controls";
        payload.observer_eligible_count = eligibleObserverCount;
        payload.observer_inclusion_probability = 1.0;
        payload.queue_admission_probability = 1.0;
        payload.scheduler_load_state = "normal";
        payload.queue_delay_ms = 0;
        payload.visibility_policy_version = "not_applicable";
        payload.probe_queued_ms = nowMS;
        payload.probe_started_ms = nowMS;
        payload.probe_completed_ms = nowMS;
        GetGame().GetWorldName(payload.map_id);
        vector observerPosition = observer.GetPosition();
        int areaCellX = Math.Floor(observerPosition[0] / 100.0);
        int areaCellZ = Math.Floor(observerPosition[2] / 100.0);
        payload.area_cell = areaCellX.ToString() + ":" + areaCellZ.ToString();
        payload.observer_speed_mps = GetVelocity(observer).Length();
        HumanMovementState movementState = new HumanMovementState;
        observer.GetMovementState(movementState);
        payload.observer_stance_id = movementState.m_iStanceIdx;
        payload.baseline_weapon_raised = observer.IsRaised();
        payload.baseline_weapon_state = "LOWERED";
        if (payload.baseline_weapon_raised)
        {
            payload.baseline_weapon_state = "RAISED";
        }
        payload.camera_mode = "UNKNOWN_SERVER_SIDE";
        if (m_DBAProbeConfig.server_first_person_only)
        {
            payload.camera_mode = "FIRST_PERSON_SERVER_POLICY";
        }
        payload.server_population_count = populationCount;
        return payload;
    }
};
