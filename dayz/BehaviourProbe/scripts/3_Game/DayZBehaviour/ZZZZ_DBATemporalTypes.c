modded class DBAProbeConstants
{
    static const int RPC_DECISION_EDGE_V2 = 759436;
};

modded class DBADecisionEdge
{
    int client_event_lower_ms;
    int client_event_upper_ms;
    int server_event_lower_ms;
    int server_event_upper_ms;
    string timing_source;
};

modded class DBAProbeWirePayload
{
    int event_time_lower_ms;
    int event_time_upper_ms;
    int event_time_estimate_ms;
    int event_time_uncertainty_ms;
    string event_timing_source;
};

modded class DBAProbeRequest
{
    int diagnostic_window_end_ms;
    int diagnostic_interval_ms;
};

modded class DBAProbeRuntime
{
    static bool ReceiveTemporalDecisionEdge(PlayerIdentity sender, ParamsReadContext ctx)
    {
        if (!sender)
        {
            return false;
        }

        int schemaVersion;
        string nonce;
        int edgeSequence;
        int clientLowerMS;
        int clientUpperMS;
        string edgeType;
        vector edgeDirection;
        bool valid = ctx.Read(schemaVersion);
        valid = ctx.Read(nonce) && valid;
        valid = ctx.Read(edgeSequence) && valid;
        valid = ctx.Read(clientLowerMS) && valid;
        valid = ctx.Read(clientUpperMS) && valid;
        valid = ctx.Read(edgeType) && valid;
        valid = ctx.Read(edgeDirection) && valid;
        if (!valid || schemaVersion != DBAProbeConstants.SCHEMA_VERSION || nonce != s_ServerNonce || edgeSequence < 1 || clientLowerMS < 0 || clientUpperMS < clientLowerMS || !IsDBAProbeDecisionEdge(edgeType) || edgeDirection.Length() < 0.5 || edgeDirection.Length() > 1.5)
        {
            return false;
        }

        string playerID = sender.GetId();
        int previousSequence;
        if (s_LastEdgeSequenceByPlayer.Find(playerID, previousSequence) && edgeSequence <= previousSequence)
        {
            return false;
        }

        int nowMS = GetGame().GetTime();
        int windowStart;
        int edgeCount;
        if (!s_EdgeWindowStartByPlayer.Find(playerID, windowStart) || nowMS - windowStart >= 1000)
        {
            windowStart = nowMS;
            edgeCount = 0;
        }
        else
        {
            s_EdgeCountByPlayer.Find(playerID, edgeCount);
        }
        if (edgeCount >= 10)
        {
            return false;
        }

        DBADecisionEdge edge = new DBADecisionEdge;
        edge.player_id = playerID;
        edge.edge_type = edgeType;
        edge.edge_sequence = edgeSequence;
        edge.client_monotonic_time_ms = clientLowerMS + ((clientUpperMS - clientLowerMS) / 2);
        edge.client_event_lower_ms = clientLowerMS;
        edge.client_event_upper_ms = clientUpperMS;
        edge.server_receive_time_ms = nowMS;
        edge.camera_direction = edgeDirection;

        float offsetMS;
        float uncertaintyMS;
        if (GetLatestClockEstimate(playerID, offsetMS, uncertaintyMS))
        {
            edge.server_event_lower_ms = clientLowerMS + offsetMS - uncertaintyMS;
            edge.server_event_upper_ms = clientUpperMS + offsetMS + uncertaintyMS;
            edge.timing_source = "ALIGNED_CLIENT_INTERVAL";
        }
        else
        {
            edge.server_event_lower_ms = Math.Max(0, nowMS - 1000);
            edge.server_event_upper_ms = nowMS;
            edge.timing_source = "SERVER_RECEIVE_FALLBACK";
        }

        s_LastEdgeSequenceByPlayer.Set(playerID, edgeSequence);
        s_EdgeWindowStartByPlayer.Set(playerID, windowStart);
        s_EdgeCountByPlayer.Set(playerID, edgeCount + 1);
        if (s_DecisionEdges.Count() >= DBAProbeConstants.MAX_QUEUED_DECISION_EDGES)
        {
            s_DecisionEdges.RemoveOrdered(0);
        }
        s_DecisionEdges.Insert(edge);
        return true;
    }
};

modded class DayZGame
{
    override void OnRPC(PlayerIdentity sender, Object target, int rpc_type, ParamsReadContext ctx)
    {
        if (rpc_type == DBAProbeConstants.RPC_DECISION_EDGE_V2 && GetGame().IsServer())
        {
            DBAProbeRuntime.RecordClientRPCResult(DBAProbeRuntime.ReceiveTemporalDecisionEdge(sender, ctx));
            return;
        }
        super.OnRPC(sender, target, rpc_type, ctx);
    }
};
