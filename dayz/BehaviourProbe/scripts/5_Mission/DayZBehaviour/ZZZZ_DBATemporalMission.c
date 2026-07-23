modded class MissionServer
{
    override void ScheduleDBAEventEnrichment()
    {
        array<ref DBADecisionEdge> edges = new array<ref DBADecisionEdge>;
        DBAProbeRuntime.DrainDecisionEdges(edges);
        if (edges.Count() == 0)
        {
            return;
        }

        array<Man> players = new array<Man>;
        GetGame().GetPlayers(players);
        int eligibleObserverCount;
        foreach (Man populationMember : players)
        {
            PlayerBase eligibleObserver = PlayerBase.Cast(populationMember);
            if (eligibleObserver && eligibleObserver.GetIdentity() && eligibleObserver.IsAlive() && !eligibleObserver.IsUnconscious())
            {
                eligibleObserverCount++;
            }
        }
        foreach (DBADecisionEdge edge : edges)
        {
            string playerSessionID = GetDBAPlayerSessionID(edge.player_id);
            DBAProbeWirePayload edgePayload = new DBAProbeWirePayload;
            edgePayload.sampling_reason = edge.edge_type;
            edgePayload.camera_direction = edge.camera_direction;
            edgePayload.event_time_lower_ms = edge.server_event_lower_ms;
            edgePayload.event_time_upper_ms = edge.server_event_upper_ms;
            edgePayload.event_time_estimate_ms = edge.server_event_lower_ms + ((edge.server_event_upper_ms - edge.server_event_lower_ms) / 2);
            edgePayload.event_time_uncertainty_ms = (edge.server_event_upper_ms - edge.server_event_lower_ms) / 2;
            edgePayload.event_timing_source = edge.timing_source;
            DBAProbeWireEvent edgeEvent = BuildDBAProbeEvent("DECISION_EDGE", "client_untrusted", playerSessionID, edge.edge_sequence, edge.client_monotonic_time_ms, edgePayload);
            edgeEvent.server_receive_ms = edge.server_receive_time_ms;
            edgeEvent.server_time_ms = edgePayload.event_time_estimate_ms;
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
            request.observer_eligible_count = eligibleObserverCount;
            request.observer_inclusion_probability = 1.0;
            request.target_eligible_count = candidates.Count();
            request.target_inclusion_probability = 1.0 / candidates.Count();
            request.risk_set_complete = false;
            request.queue_admission_probability = 1.0;
            request.queued_ms = edge.server_receive_time_ms;
            request.diagnostic_window_end_ms = edge.server_receive_time_ms + 5000;
            request.diagnostic_interval_ms = 250;
            request.decision_event_lower_ms = edge.server_event_lower_ms;
            request.decision_event_upper_ms = edge.server_event_upper_ms;
            request.decision_timing_source = edge.timing_source;
            if (m_DBAEventEnrichmentQueue.Count() >= m_DBAProbeConfig.max_event_enrichment_queue)
            {
                m_DBAEventEnrichmentDrops++;
                continue;
            }
            m_DBAEventEnrichmentQueue.Insert(request);
        }
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
            if (!request || !request.observer || !request.observer.GetIdentity() || !request.observer.IsAlive() || request.observer.IsUnconscious() || !request.target || !request.target.GetIdentity() || !request.target.IsAlive() || request.target.IsUnconscious() || request.observer == request.target || nowMS - request.queued_ms > m_DBAProbeConfig.max_probe_queue_age_ms)
            {
                continue;
            }
            if (vector.Distance(request.observer.GetPosition(), request.target.GetPosition()) > m_DBAProbeConfig.visibility_radius_metres)
            {
                continue;
            }
            if (request.not_before_ms > nowMS)
            {
                RequeueDBAProbeRequest(request);
                return;
            }

            EmitDBAVisibilityObservation(request, nowMS);
            completed++;

            if (request.sampling_stream == "event_enrichment" && request.diagnostic_window_end_ms > nowMS && request.not_before_ms <= nowMS)
            {
                request.queued_ms = nowMS;
                request.not_before_ms = nowMS + Math.Max(100, request.diagnostic_interval_ms);
                RequeueDBAProbeRequest(request);
            }
        }
    }

    protected void RequeueDBAProbeRequest(DBAProbeRequest request)
    {
        if (!request)
        {
            return;
        }
        if (request.sampling_stream == "random_opportunity")
        {
            if (m_DBARandomOpportunityQueue.Count() < m_DBAProbeConfig.max_random_opportunity_queue)
            {
                m_DBARandomOpportunityQueue.Insert(request);
            }
            else
            {
                m_DBARandomOpportunityDrops++;
            }
        }
        else
        {
            if (m_DBAEventEnrichmentQueue.Count() < m_DBAProbeConfig.max_event_enrichment_queue)
            {
                m_DBAEventEnrichmentQueue.Insert(request);
            }
            else
            {
                m_DBAEventEnrichmentDrops++;
            }
        }
    }
};
