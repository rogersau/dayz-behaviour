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
    protected ref array<string> m_PendingEvents = new array<string>;
    protected int m_BatchSequence;
    protected int m_DroppedEvents;
    protected int m_LastFlushTimeMS;
    protected bool m_RequestInFlight;
    protected string m_InFlightPayload;
    protected RestContext m_RestContext;
    protected ref DBAProbeExportCallback m_Callback;

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

    void QueueEvent(string eventJson)
    {
        if (m_PendingEvents.Count() >= m_Config.max_pending_events)
        {
            m_PendingEvents.RemoveOrdered(0);
            m_DroppedEvents++;
        }
        m_PendingEvents.Insert(eventJson);
    }

    int GetDroppedEvents()
    {
        return m_DroppedEvents;
    }

    int GetPendingEvents()
    {
        return m_PendingEvents.Count();
    }

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
        string eventsJson = "";
        for (int index = 0; index < eventCount; index++)
        {
            if (index > 0)
            {
                eventsJson += ",";
            }
            eventsJson += m_PendingEvents.Get(index);
        }
        for (index = 0; index < eventCount; index++)
        {
            m_PendingEvents.RemoveOrdered(0);
        }

        m_BatchSequence++;
        m_InFlightPayload = "{"
            + "\"schema_version\":" + DBAProbeConstants.SCHEMA_VERSION.ToString() + ","
            + "\"server_id\":" + DBAProbeJson.Quote(m_Config.server_id) + ","
            + "\"server_session_id\":" + DBAProbeJson.Quote(m_ServerSessionID) + ","
            + "\"batch_sequence\":" + m_BatchSequence.ToString() + ","
            + "\"server_time_ms\":" + nowMS.ToString() + ","
            + "\"events\":[" + eventsJson + "]"
            + "}";

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
        m_RequestInFlight = false;
        m_InFlightPayload = "";
    }

    void OnExportFailed(string reason)
    {
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

        string path = "$profile:DayZBehaviourProbe/spool/failed-batches.ndjson";
        FileHandle handle = OpenFile(path, FileMode.APPEND);
        if (handle == 0)
        {
            Print("[DayZBehaviourProbe] failed to open spool file; reason=" + reason);
            return;
        }

        FPrint(handle, payload + "\n");
        CloseFile(handle);
        Print("[DayZBehaviourProbe] spooled failed export; reason=" + reason);
    }
};
