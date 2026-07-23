modded class MissionServer
{
    override DBAProbeWirePayload BuildDBAOpportunityPayload(PlayerBase observer, int eligibleObserverCount, int populationCount, int nowMS)
    {
        DBAProbeWirePayload payload = super.BuildDBAOpportunityPayload(observer, eligibleObserverCount, populationCount, nowMS);
        payload.event_context = BuildDBAEventContextV1(observer, null, nowMS);
        return payload;
    }

    override void EmitDBAVisibilityObservation(DBAProbeRequest request, int startedMS)
    {
        if (!request || !IsDBAProbeEntityCurrent(request.observer) || !IsDBAProbeEntityCurrent(request.target))
        {
            return;
        }

        DBAEventContextV1 eventContext = BuildDBAEventContextV1(request.observer, request.target, startedMS);
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
        payload.event_context = eventContext;
        payload.visibility_ray_evidence = result.ray_evidence;
        payload.occlusion_start_event_context = request.first_event_context;
        payload.occlusion_start_ray_evidence = request.first_ray_evidence;
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
                request.first_probe_completed_ms = completedMS;
                request.first_event_context = eventContext;
                request.first_ray_evidence = result.ray_evidence;
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

    protected DBAEventContextV1 BuildDBAEventContextV1(PlayerBase observer, PlayerBase target, int capturedMS)
    {
        DBAEventContextV1 context = new DBAEventContextV1;
        context.captured_ms = capturedMS;
        context.observer = BuildDBAEventContextPlayerV1(observer);
        if (target)
        {
            context.target = BuildDBAEventContextPlayerV1(target);
        }
        return context;
    }

    protected DBAEventContextPlayerV1 BuildDBAEventContextPlayerV1(PlayerBase player)
    {
        DBAEventContextPlayerV1 context = new DBAEventContextPlayerV1;
        if (!player)
        {
            return context;
        }

        PlayerIdentity identity = player.GetIdentity();
        if (identity)
        {
            context.player_id = identity.GetId();
            context.player_session_id = GetDBAPlayerSessionID(context.player_id);
        }

        context.position = player.GetPosition();
        context.velocity = GetVelocity(player);
        context.orientation = player.GetOrientation();
        context.movement_speed_mps = context.velocity.Length();
        context.movement_state = "STOPPED";
        if (context.movement_speed_mps >= 0.25)
        {
            context.movement_state = "MOVING";
            context.movement_heading = context.velocity.Normalized();
        }

        HumanMovementState movementState = new HumanMovementState;
        player.GetMovementState(movementState);
        context.stance_id = movementState.m_iStanceIdx;
        context.alive = player.IsAlive();
        context.unconscious = player.IsUnconscious();
        context.weapon_raised = player.IsRaised();

        EntityAI itemInHands = player.GetItemInHands();
        if (itemInHands)
        {
            context.item_in_hands_type_id = itemInHands.GetType();
        }
        return context;
    }
};
