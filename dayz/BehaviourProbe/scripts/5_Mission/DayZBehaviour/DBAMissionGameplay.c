modded class MissionGameplay
{
    protected float m_DBAProbeDetectorAccumulator;
    protected float m_DBAProbeBaselineAccumulator;
    protected float m_DBAProbeBatchAccumulator;
    protected int m_DBAProbeClientSequence;
    protected int m_DBAProbeBatchSequence;
    protected int m_DBAProbeEdgeSequence;
    protected int m_DBAProbeDroppedSamples;
    protected int m_DBAProbeSamplesCaptured;
    protected int m_DBAProbeBatchesAttempted;
    protected int m_DBAProbeEdgesAttempted;
    protected int m_DBAProbeLastHealthMS;
    protected int m_DBAProbeLastDetectorMS;
    protected int m_DBAProbeBurstUntilMS;
    protected int m_DBAProbeLastQueuedSequence;
    protected ref array<ref DBAClientSample> m_DBAProbeSamples = new array<ref DBAClientSample>;
    protected ref array<ref DBAClientSample> m_DBAProbeRing = new array<ref DBAClientSample>;
    protected ref DBAClientTransitionDetector m_DBAProbeTransitionDetector = new DBAClientTransitionDetector;

    override void OnUpdate(float timeslice)
    {
        super.OnUpdate(timeslice);

        PlayerBase player = PlayerBase.Cast(GetGame().GetPlayer());
        if (!player || !player.IsAlive() || player.IsUnconscious())
        {
            ResetDBALocalCollectorState();
            return;
        }

        m_DBAProbeDetectorAccumulator += timeslice;
        m_DBAProbeBaselineAccumulator += timeslice;
        m_DBAProbeBatchAccumulator += timeslice;

        if (m_DBAProbeDetectorAccumulator >= 0.1)
        {
            m_DBAProbeDetectorAccumulator = 0;
            CaptureDBADetectorSample(player);
        }

        if (m_DBAProbeBatchAccumulator >= 1.0 || m_DBAProbeSamples.Count() >= DBAProbeConstants.MAX_CLIENT_SAMPLES_PER_BATCH)
        {
            m_DBAProbeBatchAccumulator = 0;
            SendDBAProbeBatch();
        }
        SendDBAProbeHealth();
    }

    protected void ResetDBALocalCollectorState()
    {
        m_DBAProbeSamples.Clear();
        m_DBAProbeRing.Clear();
        m_DBAProbeDetectorAccumulator = 0;
        m_DBAProbeBaselineAccumulator = 0;
        m_DBAProbeBatchAccumulator = 0;
        m_DBAProbeLastDetectorMS = 0;
        m_DBAProbeBurstUntilMS = 0;
        m_DBAProbeLastQueuedSequence = 0;
        m_DBAProbeTransitionDetector = new DBAClientTransitionDetector;
    }

    protected void CaptureDBADetectorSample(PlayerBase player)
    {
        DBAClientSample sample = new DBAClientSample;
        m_DBAProbeSamplesCaptured++;
        sample.client_sequence = ++m_DBAProbeClientSequence;
        sample.client_monotonic_time_ms = GetGame().GetTime();
        sample.camera_position = GetGame().GetCurrentCameraPosition();
        sample.camera_direction = GetGame().GetCurrentCameraDirection();
        sample.client_observed_player_position = player.GetPosition();
        sample.is_weapon_raised = player.IsFireWeaponRaised();
        sample.is_in_ironsights = player.IsInIronsights();
        sample.is_in_optics = player.IsInOptics();
        sample.is_in_third_person = player.IsInThirdPerson();
        sample.sample_mode = "detector";

        EntityAI itemInHands = player.GetItemInHands();
        if (itemInHands)
        {
            sample.item_in_hands_type_id = itemInHands.GetType();
        }
        DBAProbeRuntime.ConsumeLocalShot(sample.local_shot_count, sample.local_shot_muzzle_index);

        if (m_DBAProbeRing.Count() >= 50)
        {
            m_DBAProbeRing.RemoveOrdered(0);
        }
        m_DBAProbeRing.Insert(sample);

        int lowerMS = m_DBAProbeLastDetectorMS;
        if (lowerMS <= 0)
        {
            lowerMS = Math.Max(0, sample.client_monotonic_time_ms - 100);
        }
        array<string> transitions = new array<string>;
        m_DBAProbeTransitionDetector.Process(sample, transitions);
        bool meaningfulEdge;
        foreach (string transition : transitions)
        {
            if (transition == "WEAPON_RAISED" || transition == "ADS_ENTERED" || transition == "OPTICS_ENTERED" || transition == "DELIBERATE_CAMERA_TURN" || transition == "SHOT_FIRED_CLIENT")
            {
                meaningfulEdge = true;
                m_DBAProbeBurstUntilMS = sample.client_monotonic_time_ms + 10000;
                SendDBATemporalDecisionEdge(transition, lowerMS, sample.client_monotonic_time_ms, sample.camera_direction);
            }
        }
        if (meaningfulEdge)
        {
            QueueDBARingForExport();
        }

        if (sample.client_monotonic_time_ms <= m_DBAProbeBurstUntilMS)
        {
            sample.sample_mode = "burst";
            QueueDBASampleForExport(sample);
        }
        else if (m_DBAProbeBaselineAccumulator >= 0.5)
        {
            m_DBAProbeBaselineAccumulator = 0;
            sample.sample_mode = "baseline";
            QueueDBASampleForExport(sample);
        }
        m_DBAProbeLastDetectorMS = sample.client_monotonic_time_ms;
    }

    protected void QueueDBARingForExport()
    {
        foreach (DBAClientSample sample : m_DBAProbeRing)
        {
            sample.sample_mode = "pre_event_ring";
            QueueDBASampleForExport(sample);
        }
    }

    protected void QueueDBASampleForExport(DBAClientSample sample)
    {
        if (!sample || sample.client_sequence <= m_DBAProbeLastQueuedSequence)
        {
            return;
        }
        if (m_DBAProbeSamples.Count() >= DBAProbeConstants.MAX_CLIENT_SAMPLES_PER_BATCH)
        {
            m_DBAProbeSamples.RemoveOrdered(0);
            m_DBAProbeDroppedSamples++;
        }
        m_DBAProbeSamples.Insert(sample);
        m_DBAProbeLastQueuedSequence = sample.client_sequence;
    }

    protected void SendDBATemporalDecisionEdge(string edgeType, int clientLowerMS, int clientUpperMS, vector cameraDirection)
    {
        string nonce = DBAProbeRuntime.GetClientNonce();
        if (nonce == "")
        {
            return;
        }

        ScriptRPC rpc = new ScriptRPC;
        m_DBAProbeEdgesAttempted++;
        rpc.Write(DBAProbeConstants.SCHEMA_VERSION);
        rpc.Write(nonce);
        rpc.Write(++m_DBAProbeEdgeSequence);
        rpc.Write(clientLowerMS);
        rpc.Write(clientUpperMS);
        rpc.Write(edgeType);
        rpc.Write(cameraDirection);
        rpc.Send(null, DBAProbeConstants.RPC_DECISION_EDGE_V2, true, null);
    }

    protected void SendDBAProbeBatch()
    {
        string nonce = DBAProbeRuntime.GetClientNonce();
        if (nonce == "" || m_DBAProbeSamples.Count() == 0)
        {
            return;
        }

        ScriptRPC rpc = new ScriptRPC;
        m_DBAProbeBatchesAttempted++;
        rpc.Write(DBAProbeConstants.SCHEMA_VERSION);
        rpc.Write(nonce);
        rpc.Write(++m_DBAProbeBatchSequence);
        rpc.Write(m_DBAProbeSamples.Count());

        foreach (DBAClientSample sample : m_DBAProbeSamples)
        {
            rpc.Write(sample.client_sequence);
            rpc.Write(sample.client_monotonic_time_ms);
            rpc.Write(sample.camera_position);
            rpc.Write(sample.camera_direction);
            rpc.Write(sample.client_observed_player_position);
            rpc.Write(sample.is_weapon_raised);
            rpc.Write(sample.is_in_ironsights);
            rpc.Write(sample.is_in_optics);
            rpc.Write(sample.is_in_third_person);
            rpc.Write(sample.item_in_hands_type_id);
            rpc.Write(sample.sample_mode);
            rpc.Write(sample.local_shot_count);
            rpc.Write(sample.local_shot_muzzle_index);
        }

        rpc.Send(null, DBAProbeConstants.RPC_CLIENT_SAMPLE_BATCH, true, null);
        m_DBAProbeSamples.Clear();
    }

    protected void SendDBAProbeHealth()
    {
        int nowMS = GetGame().GetTime();
        string nonce = DBAProbeRuntime.GetClientNonce();
        if (nonce == "" || nowMS - m_DBAProbeLastHealthMS < 30000)
        {
            return;
        }
        m_DBAProbeLastHealthMS = nowMS;
        ScriptRPC rpc = new ScriptRPC;
        rpc.Write(DBAProbeConstants.SCHEMA_VERSION);
        rpc.Write(nonce);
        rpc.Write(m_DBAProbeSamplesCaptured);
        rpc.Write(m_DBAProbeBatchesAttempted);
        rpc.Write(m_DBAProbeEdgesAttempted);
        rpc.Write(m_DBAProbeDroppedSamples);
        rpc.Write(m_DBAProbeSamples.Count());
        rpc.Send(null, DBAProbeConstants.RPC_CLIENT_HEALTH, true, null);
    }
};
