modded class MissionServer
{
    override void EmitDBACollectorHealth()
    {
        int nowMS = GetGame().GetTime();
        if (nowMS - m_DBALastHealthEventMS < 30000)
        {
            return;
        }
        m_DBALastHealthEventMS = nowMS;
        DBAProbeWirePayload payload = new DBAProbeWirePayload;
        payload.scheduler_load_state = "normal";
        payload.sampling_policy_version = "dual-stream-v2-neutral-controls";
        payload.visibility_policy_version = "validated-first-person-head-v1";
        payload.pending_export_events = m_DBAProbeExporter.GetPendingEvents();
        payload.dropped_export_events = m_DBAProbeExporter.GetDroppedEvents();
        payload.dropped_critical_events = m_DBAProbeExporter.GetDroppedCriticalEvents();
        payload.dropped_evidence_events = m_DBAProbeExporter.GetDroppedEvidenceEvents();
        payload.dropped_context_events = m_DBAProbeExporter.GetDroppedContextEvents();
        payload.dropped_health_events = m_DBAProbeExporter.GetDroppedHealthEvents();
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
};
