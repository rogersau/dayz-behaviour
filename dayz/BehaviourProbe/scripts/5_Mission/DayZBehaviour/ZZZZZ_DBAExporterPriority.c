modded class DBAProbeExporter
{
    protected int m_DroppedCriticalEvents;
    protected int m_DroppedEvidenceEvents;
    protected int m_DroppedContextEvents;
    protected int m_DroppedHealthEvents;

    override void QueueEvent(DBAProbeWireEvent eventData)
    {
        if (!eventData)
        {
            return;
        }
        if (m_PendingEvents.Count() >= m_Config.max_pending_events)
        {
            int incomingPriority = DBAEventPriority(eventData.event_type);
            int lowestPriority = 100;
            int lowestIndex = -1;
            for (int index = 0; index < m_PendingEvents.Count(); index++)
            {
                DBAProbeWireEvent existing = m_PendingEvents.Get(index);
                int priority = 0;
                if (existing)
                {
                    priority = DBAEventPriority(existing.event_type);
                }
                if (priority < lowestPriority)
                {
                    lowestPriority = priority;
                    lowestIndex = index;
                }
            }

            if (lowestIndex < 0 || incomingPriority < lowestPriority)
            {
                RecordDBADroppedEvent(eventData.event_type);
                return;
            }

            DBAProbeWireEvent dropped = m_PendingEvents.Get(lowestIndex);
            if (dropped)
            {
                RecordDBADroppedEvent(dropped.event_type);
            }
            else
            {
                m_DroppedEvents++;
            }
            m_PendingEvents.RemoveOrdered(lowestIndex);
        }
        m_PendingEvents.Insert(eventData);
    }

    int GetDroppedCriticalEvents() { return m_DroppedCriticalEvents; }
    int GetDroppedEvidenceEvents() { return m_DroppedEvidenceEvents; }
    int GetDroppedContextEvents() { return m_DroppedContextEvents; }
    int GetDroppedHealthEvents() { return m_DroppedHealthEvents; }

    protected void RecordDBADroppedEvent(string eventType)
    {
        m_DroppedEvents++;
        int priority = DBAEventPriority(eventType);
        if (priority >= 4)
        {
            m_DroppedCriticalEvents++;
        }
        else if (priority == 3)
        {
            m_DroppedEvidenceEvents++;
        }
        else if (priority == 2)
        {
            m_DroppedContextEvents++;
        }
        else
        {
            m_DroppedHealthEvents++;
        }
    }

    protected int DBAEventPriority(string eventType)
    {
        if (eventType == "PLAYER_CONNECTED" || eventType == "PLAYER_READY" || eventType == "PLAYER_RESPAWNED" || eventType == "PLAYER_RECONNECTED" || eventType == "PLAYER_DISCONNECTED" || eventType == "PLAYER_HIT" || eventType == "PLAYER_KILLED" || eventType == "SHOT_FIRED_SERVER")
        {
            return 4;
        }
        if (eventType == "VISIBILITY_OBSERVATION" || eventType == "SAMPLING_OPPORTUNITY" || eventType == "SAMPLING_OPPORTUNITY_DROPPED" || eventType == "DECISION_EDGE")
        {
            return 3;
        }
        if (eventType == "PLAYER_SNAPSHOT" || eventType == "CAMERA_SAMPLE")
        {
            return 2;
        }
        return 1;
    }
};
