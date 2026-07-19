modded class MissionGameplay
{
    protected float m_DBAProbeSampleAccumulator;
    protected float m_DBAProbeBatchAccumulator;
    protected int m_DBAProbeClientSequence;
    protected int m_DBAProbeBatchSequence;
    protected int m_DBAProbeDroppedSamples;
    protected ref array<ref DBAClientSample> m_DBAProbeSamples = new array<ref DBAClientSample>;

    override void OnUpdate(float timeslice)
    {
        super.OnUpdate(timeslice);

        PlayerBase player = PlayerBase.Cast(GetGame().GetPlayer());
        if (!player || !player.IsAlive() || player.IsUnconscious())
        {
            m_DBAProbeSamples.Clear();
            m_DBAProbeSampleAccumulator = 0;
            m_DBAProbeBatchAccumulator = 0;
            return;
        }

        float interval = 0.5;
        if (player.IsFireWeaponRaised() || player.IsInIronsights() || player.IsInOptics())
        {
            interval = 0.1;
        }

        m_DBAProbeSampleAccumulator += timeslice;
        m_DBAProbeBatchAccumulator += timeslice;

        if (m_DBAProbeSampleAccumulator >= interval)
        {
            m_DBAProbeSampleAccumulator = 0;
            CaptureDBAProbeSample(player);
        }

        if (m_DBAProbeBatchAccumulator >= 1.0 || m_DBAProbeSamples.Count() >= DBAProbeConstants.MAX_CLIENT_SAMPLES_PER_BATCH)
        {
            m_DBAProbeBatchAccumulator = 0;
            SendDBAProbeBatch();
        }
    }

    protected void CaptureDBAProbeSample(PlayerBase player)
    {
        DBAClientSample sample = new DBAClientSample;
        sample.client_sequence = ++m_DBAProbeClientSequence;
        sample.client_monotonic_time_ms = GetGame().GetTime();
        sample.camera_position = GetGame().GetCurrentCameraPosition();
        sample.camera_direction = GetGame().GetCurrentCameraDirection();
        sample.client_observed_player_position = player.GetPosition();
        sample.is_weapon_raised = player.IsFireWeaponRaised();
        sample.is_in_ironsights = player.IsInIronsights();
        sample.is_in_optics = player.IsInOptics();
        sample.is_in_third_person = player.IsInThirdPerson();

        EntityAI itemInHands = player.GetItemInHands();
        if (itemInHands)
        {
            sample.item_in_hands_type_id = itemInHands.GetType();
        }

        DBAProbeRuntime.ConsumeLocalShot(sample.local_shot_count, sample.local_shot_muzzle_index);

        if (m_DBAProbeSamples.Count() >= DBAProbeConstants.MAX_CLIENT_SAMPLES_PER_BATCH)
        {
            m_DBAProbeSamples.RemoveOrdered(0);
            m_DBAProbeDroppedSamples++;
        }
        m_DBAProbeSamples.Insert(sample);
    }

    protected void SendDBAProbeBatch()
    {
        string nonce = DBAProbeRuntime.GetClientNonce();
        if (nonce == "" || m_DBAProbeSamples.Count() == 0)
        {
            return;
        }

        ScriptRPC rpc = new ScriptRPC;
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
            rpc.Write(sample.local_shot_count);
            rpc.Write(sample.local_shot_muzzle_index);
        }

        rpc.Send(null, DBAProbeConstants.RPC_CLIENT_SAMPLE_BATCH, true, null);
        m_DBAProbeSamples.Clear();
    }
};
