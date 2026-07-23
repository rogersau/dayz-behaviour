modded class MissionServer
{
    protected float m_DBAAudioContextAccumulator;

    override void OnUpdate(float timeslice)
    {
        super.OnUpdate(timeslice);

        if (!m_DBAProbeConfig || !m_DBAProbeConfig.enabled || !m_DBAProbeConfig.enable_audio_cues || !m_DBAProbeExporter)
        {
            return;
        }

        m_DBAAudioContextAccumulator += timeslice;
        float interval = Math.Max(0.25, m_DBAProbeConfig.audio_context_interval_seconds);
        if (m_DBAAudioContextAccumulator < interval)
        {
            return;
        }
        m_DBAAudioContextAccumulator = 0;
        CaptureDBAMovementAudioOpportunities();
    }

    override void DrainDBAProbeCombatEvents()
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
            else if (eventData.source_player_id != "")
            {
                playerSessionID = GetDBAPlayerSessionID(eventData.source_player_id);
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
            payload.weapon_type = eventData.weapon_type;
            payload.muzzle_index = eventData.muzzle_index;
            payload.is_suppressed = eventData.is_suppressed;
            payload.suppressor_type = eventData.suppressor_type;
            if (eventData.event_type == "SHOT_FIRED_SERVER")
            {
                payload.position = eventData.world_position;
            }

            m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent(eventData.event_type, "server", playerSessionID, 0, 0, payload));
        }
    }

    protected void CaptureDBAMovementAudioOpportunities()
    {
        array<Man> players = new array<Man>;
        GetGame().GetPlayers(players);

        foreach (Man man : players)
        {
            PlayerBase player = PlayerBase.Cast(man);
            if (!player || !player.GetIdentity() || !player.IsAlive() || player.IsUnconscious())
            {
                continue;
            }

            float speedMPS = GetVelocity(player).Length();
            if (speedMPS < m_DBAProbeConfig.audio_min_movement_speed_mps)
            {
                continue;
            }

            string playerID = player.GetIdentity().GetId();
            DBAProbeWirePayload payload = new DBAProbeWirePayload;
            payload.source_player_id = playerID;
            payload.position = player.GetPosition();
            payload.movement_speed_mps = speedMPS;
            payload.movement_state = GetDBAAudioGait(speedMPS);

            HumanMovementState movementState = new HumanMovementState;
            player.GetMovementState(movementState);
            payload.observer_stance_id = movementState.m_iStanceIdx;
            payload.stance_name = GetDBAStanceName(movementState.m_iStanceIdx);

            vector position = player.GetPosition();
            GetGame().SurfaceGetType3D(position[0], position[1], position[2], payload.surface_type);

            EntityAI footwear = player.FindAttachmentBySlotName("Feet");
            if (footwear)
            {
                payload.footwear_type = footwear.GetType();
            }

            string playerSessionID = GetDBAPlayerSessionID(playerID);
            m_DBAProbeExporter.QueueEvent(BuildDBAProbeEvent("MOVEMENT_AUDIO_OPPORTUNITY", "server", playerSessionID, 0, 0, payload));
        }
    }

    protected string GetDBAAudioGait(float speedMPS)
    {
        if (speedMPS < 1.2)
        {
            return "SLOW";
        }
        if (speedMPS < 2.5)
        {
            return "WALK";
        }
        if (speedMPS < 4.5)
        {
            return "JOG";
        }
        return "SPRINT";
    }

    protected string GetDBAStanceName(int stanceIndex)
    {
        if (stanceIndex == DayZPlayerConstants.STANCEIDX_PRONE)
        {
            return "PRONE";
        }
        if (stanceIndex == DayZPlayerConstants.STANCEIDX_CROUCH)
        {
            return "CROUCH";
        }
        return "ERECT";
    }
};
