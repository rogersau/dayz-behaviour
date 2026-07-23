modded class MissionServer
{
    override void SendDBAClockChallenges()
    {
        int nowMS = GetGame().GetTime();
        int intervalMS = m_DBAProbeConfig.clock_challenge_interval_seconds * 1000;
        if (intervalMS < 1000)
        {
            intervalMS = 1000;
        }
        if (nowMS - m_DBALastClockChallengeMS < intervalMS)
        {
            return;
        }
        m_DBALastClockChallengeMS = nowMS;

        array<Man> players = new array<Man>;
        GetGame().GetPlayers(players);
        foreach (Man man : players)
        {
            PlayerBase player = PlayerBase.Cast(man);
            if (!player || !player.GetIdentity())
            {
                continue;
            }
            int challengeSequence = ++m_DBAClockChallengeSequence;
            string playerID = player.GetIdentity().GetId();
            DBAProbeRuntime.RecordClockChallenge(playerID, challengeSequence, nowMS);

            ScriptRPC challenge = new ScriptRPC;
            challenge.Write(DBAProbeConstants.SCHEMA_VERSION);
            challenge.Write(m_DBAProbeServerSessionID);
            challenge.Write(challengeSequence);
            challenge.Write(nowMS);
            challenge.Send(null, DBAProbeConstants.RPC_CLOCK_CHALLENGE, true, player.GetIdentity());
        }
    }
};
