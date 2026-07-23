modded class DBAProbeRuntime
{
    static void ResetPlayerState(string playerID)
    {
        if (playerID == "")
        {
            return;
        }

        s_LastBatchSequenceByPlayer.Remove(playerID);
        s_LastEdgeSequenceByPlayer.Remove(playerID);
        s_EdgeWindowStartByPlayer.Remove(playerID);
        s_EdgeCountByPlayer.Remove(playerID);
        s_BatchWindowStartByPlayer.Remove(playerID);
        s_BatchCountByPlayer.Remove(playerID);
        ResetClockChallenge(playerID);

        for (int batchIndex = s_ReceivedClientBatches.Count() - 1; batchIndex >= 0; batchIndex--)
        {
            DBAReceivedClientBatch batch = s_ReceivedClientBatches.Get(batchIndex);
            if (batch && batch.player_id == playerID)
            {
                s_ReceivedClientBatches.RemoveOrdered(batchIndex);
            }
        }
        for (int edgeIndex = s_DecisionEdges.Count() - 1; edgeIndex >= 0; edgeIndex--)
        {
            DBADecisionEdge edge = s_DecisionEdges.Get(edgeIndex);
            if (edge && edge.player_id == playerID)
            {
                s_DecisionEdges.RemoveOrdered(edgeIndex);
            }
        }
        for (int clockIndex = s_ClockSamples.Count() - 1; clockIndex >= 0; clockIndex--)
        {
            DBAClockAlignment clockSample = s_ClockSamples.Get(clockIndex);
            if (clockSample && clockSample.player_id == playerID)
            {
                s_ClockSamples.RemoveOrdered(clockIndex);
            }
        }
        for (int healthIndex = s_ClientHealth.Count() - 1; healthIndex >= 0; healthIndex--)
        {
            DBAClientCollectorHealth health = s_ClientHealth.Get(healthIndex);
            if (health && health.player_id == playerID)
            {
                s_ClientHealth.RemoveOrdered(healthIndex);
            }
        }
    }
};
