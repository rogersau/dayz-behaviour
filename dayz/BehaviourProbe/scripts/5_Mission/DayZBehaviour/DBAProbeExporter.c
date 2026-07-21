class DBAProbeExportCallback : RestCallback
{
    protected DBAProbeExporter m_Owner;

    void DBAProbeExportCallback(DBAProbeExporter owner)
    {
        m_Owner = owner;
    }

    override void OnError(int errorCode)
    {
        if (m_Owner)
        {
            m_Owner.OnExportFailed("error:" + errorCode.ToString());
        }
    }

    override void OnTimeout()
    {
        if (m_Owner)
        {
            m_Owner.OnExportFailed("timeout");
        }
    }

    override void OnSuccess(string data, int dataSize)
    {
        if (m_Owner)
        {
            m_Owner.OnExportSucceeded();
        }
    }
};

class DBAProbeExporter
{
    protected ref DBAProbeServerConfig m_Config;
    protected string m_ServerSessionID;
    protected ref array<ref DBAProbeWireEvent> m_PendingEvents = new array<ref DBAProbeWireEvent>;
    protected int m_BatchSequence;
    protected int m_DroppedEvents;
    protected int m_LastFlushTimeMS;
    protected bool m_RequestInFlight;
    protected string m_InFlightPayload;
    protected RestContext m_RestContext;
    protected ref DBAProbeExportCallback m_Callback;
    protected int m_SpoolSlot;
    protected int m_SpoolBatchCount;
    protected bool m_SpoolInitialized;
    protected int m_ExportSuccessCount;
    protected int m_ExportFailureCount;
    protected int m_SpoolOverwriteCount;

    void DBAProbeExporter(DBAProbeServerConfig config, string serverSessionID)
    {
        m_Config = config;
        m_ServerSessionID = serverSessionID;
        m_Callback = new DBAProbeExportCallback(this);

        MakeDirectory("$profile:DayZBehaviourProbe");
        MakeDirectory("$profile:DayZBehaviourProbe/spool");

        RestApi api = GetRestApi();
        if (!api)
        {
            api = CreateRestApi();
        }
        if (api && m_Config.endpoint != "")
        {
            m_RestContext = api.GetRestContext(m_Config.endpoint);
            if (m_RestContext)
            {
                m_RestContext.SetHeader("application/json");
            }
        }
    }

    void QueueEvent(DBAProbeWireEvent eventData)
    {
        if (!eventData)
        {
            return;
        }
        if (m_PendingEvents.Count() >= m_Config.max_pending_events)
        {
            m_PendingEvents.RemoveOrdered(0);
            m_DroppedEvents++;
        }
        m_PendingEvents.Insert(eventData);
    }

    int GetDroppedEvents()
    {
        return m_DroppedEvents;
    }

    int GetPendingEvents()
    {
        return m_PendingEvents.Count();
    }

    int GetExportSuccessCount() { return m_ExportSuccessCount; }
    int GetExportFailureCount() { return m_ExportFailureCount; }
    int GetSpoolOverwriteCount() { return m_SpoolOverwriteCount; }

    void Update(int nowMS)
    {
        float flushIntervalMS = m_Config.export_interval_seconds * 1000.0;
        if (flushIntervalMS < 100.0)
        {
            flushIntervalMS = 100.0;
        }
        if (nowMS - m_LastFlushTimeMS < flushIntervalMS)
        {
            return;
        }
        m_LastFlushTimeMS = nowMS;
        Flush(nowMS);
    }

    void Flush(int nowMS)
    {
        if (m_RequestInFlight || m_PendingEvents.Count() == 0)
        {
            return;
        }

        int eventCount = Math.Min(m_Config.max_events_per_export, m_PendingEvents.Count());
        DBAProbeWireBatch batch = new DBAProbeWireBatch;
        batch.schema_version = DBAProbeConstants.SCHEMA_VERSION;
        batch.server_id = m_Config.server_id;
        batch.server_session_id = m_ServerSessionID;
        batch.batch_sequence = ++m_BatchSequence;
        batch.server_time_ms = nowMS;
        batch.collector_version = "milestone-0.1";
        batch.dayz_build = "1.29.0.163451";
        batch.configuration_hash = m_Config.configuration_hash;

        for (int index = 0; index < eventCount; index++)
        {
            batch.events.Insert(m_PendingEvents.Get(index));
        }
        for (index = 0; index < eventCount; index++)
        {
            m_PendingEvents.RemoveOrdered(0);
        }

        JsonSerializer serializer = new JsonSerializer;
        if (!serializer.WriteToString(batch, false, m_InFlightPayload))
        {
            m_DroppedEvents += eventCount;
            m_InFlightPayload = "";
            Print("[DayZBehaviourProbe] failed to serialize export batch");
            return;
        }

        if (!m_RestContext)
        {
            Spool(m_InFlightPayload, "rest_context_unavailable");
            m_InFlightPayload = "";
            return;
        }

        string requestPath = "v1/telemetry/batches";
        if (m_Config.ingest_token != "")
        {
            requestPath += "?token=" + m_Config.ingest_token;
        }

        m_RequestInFlight = true;
        int result = m_RestContext.POST(m_Callback, requestPath, m_InFlightPayload);
        if (result >= ERestResultState.EREST_ERROR)
        {
            m_RequestInFlight = false;
            Spool(m_InFlightPayload, "post_dispatch:" + result.ToString());
            m_InFlightPayload = "";
        }
    }

    void OnExportSucceeded()
    {
        m_ExportSuccessCount++;
        m_RequestInFlight = false;
        m_InFlightPayload = "";
    }

    void OnExportFailed(string reason)
    {
        m_ExportFailureCount++;
        m_RequestInFlight = false;
        Spool(m_InFlightPayload, reason);
        m_InFlightPayload = "";
    }

    protected void Spool(string payload, string reason)
    {
        if (payload == "")
        {
            return;
        }

        if (m_Config.max_spool_files < 1 || m_Config.max_spool_batches_per_file < 1)
        {
            Print("[DayZBehaviourProbe] spool disabled by invalid bounds; reason=" + reason);
            return;
        }
        if (m_SpoolBatchCount >= m_Config.max_spool_batches_per_file)
        {
            m_SpoolSlot = (m_SpoolSlot + 1) % m_Config.max_spool_files;
            m_SpoolBatchCount = 0;
            m_SpoolInitialized = false;
            if (m_SpoolSlot == 0)
            {
                m_SpoolOverwriteCount++;
            }
        }

        string path = "$profile:DayZBehaviourProbe/spool/failed-batches-" + m_SpoolSlot.ToString() + ".ndjson";
        int mode = FileMode.APPEND;
        if (!m_SpoolInitialized)
        {
            mode = FileMode.WRITE;
        }
        FileHandle handle = OpenFile(path, mode);
        if (handle == 0)
        {
            Print("[DayZBehaviourProbe] failed to open spool file; reason=" + reason);
            return;
        }

        FPrint(handle, payload + "\n");
        CloseFile(handle);
        m_SpoolInitialized = true;
        m_SpoolBatchCount++;
        Print("[DayZBehaviourProbe] spooled failed export; reason=" + reason);
    }
};
