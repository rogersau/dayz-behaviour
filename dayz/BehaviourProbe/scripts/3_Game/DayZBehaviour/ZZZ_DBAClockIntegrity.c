modded class DBAProbeRuntime
{
    protected static ref map<string, int> s_ClockChallengeSequenceByPlayer = new map<string, int>;
    protected static ref map<string, int> s_ClockChallengeSendMSByPlayer = new map<string, int>;
    protected static ref map<string, int> s_ClockChallengeExpiryMSByPlayer = new map<string, int>;

    static void RecordClockChallenge(string playerID, int challengeSequence, int serverSendMS)
    {
        if (playerID == "" || challengeSequence < 1 || serverSendMS < 0)
        {
            return;
        }
        s_ClockChallengeSequenceByPlayer.Set(playerID, challengeSequence);
        s_ClockChallengeSendMSByPlayer.Set(playerID, serverSendMS);
        s_ClockChallengeExpiryMSByPlayer.Set(playerID, serverSendMS + 10000);
    }

    static void ResetClockChallenge(string playerID)
    {
        if (playerID == "")
        {
            return;
        }
        s_ClockChallengeSequenceByPlayer.Remove(playerID);
        s_ClockChallengeSendMSByPlayer.Remove(playerID);
        s_ClockChallengeExpiryMSByPlayer.Remove(playerID);
    }

    static void ResetClockAlignment(string playerID)
    {
        ResetClockChallenge(playerID);
        if (playerID == "")
        {
            return;
        }
        s_LatestClockOffsetByPlayer.Remove(playerID);
        s_LatestClockUncertaintyByPlayer.Remove(playerID);
    }

    static bool ReceiveValidatedClockResponse(PlayerIdentity sender, ParamsReadContext ctx)
    {
        if (!sender)
        {
            return false;
        }

        int schemaVersion;
        string nonce;
        int challengeSequence;
        int echoedServerSendMS;
        int clientReceiveMS;
        int clientSendMS;
        bool valid = ctx.Read(schemaVersion);
        valid = ctx.Read(nonce) && valid;
        valid = ctx.Read(challengeSequence) && valid;
        valid = ctx.Read(echoedServerSendMS) && valid;
        valid = ctx.Read(clientReceiveMS) && valid;
        valid = ctx.Read(clientSendMS) && valid;
        if (!valid || schemaVersion != DBAProbeConstants.SCHEMA_VERSION || nonce != s_ServerNonce || clientReceiveMS < 0 || clientSendMS < clientReceiveMS)
        {
            return false;
        }

        string playerID = sender.GetId();
        int expectedSequence;
        int expectedServerSendMS;
        int expiryMS;
        if (!s_ClockChallengeSequenceByPlayer.Find(playerID, expectedSequence))
        {
            return false;
        }
        if (!s_ClockChallengeSendMSByPlayer.Find(playerID, expectedServerSendMS))
        {
            return false;
        }
        if (!s_ClockChallengeExpiryMSByPlayer.Find(playerID, expiryMS))
        {
            return false;
        }

        int serverReceiveMS = GetGame().GetTime();
        ResetClockChallenge(playerID);
        if (challengeSequence != expectedSequence || echoedServerSendMS != expectedServerSendMS || serverReceiveMS < expectedServerSendMS || serverReceiveMS > expiryMS)
        {
            return false;
        }

        int roundTripMS = (serverReceiveMS - expectedServerSendMS) - (clientSendMS - clientReceiveMS);
        if (roundTripMS < 0)
        {
            roundTripMS = 0;
        }
        DBAClockAlignment sample = new DBAClockAlignment;
        sample.player_id = playerID;
        sample.challenge_sequence = challengeSequence;
        sample.server_send_ms = expectedServerSendMS;
        sample.client_receive_ms = clientReceiveMS;
        sample.client_send_ms = clientSendMS;
        sample.server_receive_ms = serverReceiveMS;
        sample.round_trip_ms = roundTripMS;
        sample.offset_estimate_ms = ((expectedServerSendMS - clientReceiveMS) + (serverReceiveMS - clientSendMS)) * 0.5;
        sample.uncertainty_ms = roundTripMS * 0.5;
        s_LatestClockOffsetByPlayer.Set(playerID, sample.offset_estimate_ms);
        s_LatestClockUncertaintyByPlayer.Set(playerID, sample.uncertainty_ms);
        if (s_ClockSamples.Count() >= DBAProbeConstants.MAX_QUEUED_CLOCK_SAMPLES)
        {
            s_ClockSamples.RemoveOrdered(0);
        }
        s_ClockSamples.Insert(sample);
        return true;
    }
};

modded class DayZGame
{
    override void OnRPC(PlayerIdentity sender, Object target, int rpc_type, ParamsReadContext ctx)
    {
        if (rpc_type == DBAProbeConstants.RPC_CLOCK_RESPONSE && GetGame().IsServer())
        {
            DBAProbeRuntime.RecordClientRPCResult(DBAProbeRuntime.ReceiveValidatedClockResponse(sender, ctx));
            return;
        }
        super.OnRPC(sender, target, rpc_type, ctx);
    }
};
