modded class MissionServer
{
    override void OnClientReconnectEvent(PlayerIdentity identity, PlayerBase player)
    {
        if (identity)
        {
            DBAProbeRuntime.ResetPlayerState(identity.GetId());
        }
        super.OnClientReconnectEvent(identity, player);
    }

    override void OnClientDisconnectedEvent(PlayerIdentity identity, PlayerBase player, int logoutTime, bool authFailed)
    {
        string playerID = "";
        if (identity)
        {
            playerID = identity.GetId();
        }
        super.OnClientDisconnectedEvent(identity, player, logoutTime, authFailed);
        DBAProbeRuntime.ResetPlayerState(playerID);
    }

    override void DrainDBAProbeQueues()
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
            if (!request || !IsDBAProbeEntityCurrent(request.observer) || !IsDBAProbeEntityCurrent(request.target) || request.observer == request.target || nowMS - request.queued_ms > m_DBAProbeConfig.max_probe_queue_age_ms)
            {
                continue;
            }
            if (vector.Distance(request.observer.GetPosition(), request.target.GetPosition()) > m_DBAProbeConfig.visibility_radius_metres)
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

    override void EmitDBAVisibilityObservation(DBAProbeRequest request, int startedMS)
    {
        if (!request || !IsDBAProbeEntityCurrent(request.observer) || !IsDBAProbeEntityCurrent(request.target))
        {
            return;
        }

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
        payload.sampling_policy_version = "dual-stream-v2-neutral-controls";
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
        payload.visibility_policy_version = "validated-first-person-head-v1";
        payload.probe_queued_ms = request.queued_ms;
        payload.probe_started_ms = startedMS;
        payload.probe_completed_ms = completedMS;
        if (request.decision_event_upper_ms > 0)
        {
            payload.event_time_lower_ms = request.decision_event_lower_ms;
            payload.event_time_upper_ms = request.decision_event_upper_ms;
            payload.event_time_estimate_ms = request.decision_event_lower_ms + ((request.decision_event_upper_ms - request.decision_event_lower_ms) / 2);
            payload.event_time_uncertainty_ms = (request.decision_event_upper_ms - request.decision_event_lower_ms) / 2;
            payload.event_timing_source = request.decision_timing_source;
        }

        bool validatedFirstPerson = m_DBAProbeConfig.server_first_person_only && m_DBAProbeConfig.visibility_origin_mode == "VALIDATED_FIRST_PERSON_HEAD" && m_DBAProbeConfig.visibility_validation_id != "";
        string classification = result.GetClassificationName();
        if (classification == "HEAD_ORIGIN_OCCLUDED" && validatedFirstPerson)
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
        payload.camera_mode = "UNKNOWN_SERVER_SIDE";
        if (validatedFirstPerson)
        {
            payload.observer_origin_mode = "VALIDATED_FIRST_PERSON_HEAD";
            payload.camera_mode = "FIRST_PERSON_SERVER_POLICY";
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
        array<Man> population = new array<Man>;
        GetGame().GetPlayers(population);
        payload.server_population_count = population.Count();

        m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent("VISIBILITY_OBSERVATION", "server", payload.observer_player_session_id, 0, 0, payload));
    }

    protected bool IsDBAProbeEntityCurrent(PlayerBase player)
    {
        return player && player.GetIdentity() && player.IsAlive() && !player.IsUnconscious();
    }
};
