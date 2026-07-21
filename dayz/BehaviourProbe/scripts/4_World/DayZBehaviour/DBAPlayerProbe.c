modded class PlayerBase
{
    override void EEHitBy(
        TotalDamageResult damageResult,
        int damageType,
        EntityAI source,
        int component,
        string dmgZone,
        string ammo,
        vector modelPos,
        float speedCoef
    )
    {
        super.EEHitBy(damageResult, damageType, source, component, dmgZone, ammo, modelPos, speedCoef);

        if (!GetGame().IsServer())
        {
            return;
        }

        DBAProbeCombatEvent eventData = new DBAProbeCombatEvent;
        eventData.event_type = "PLAYER_HIT";
        eventData.server_time_ms = GetGame().GetTime();
        eventData.target_player_id = DBAProbePlayerIdentity.GetPlayerID(this);
        eventData.source_player_id = DBAProbePlayerIdentity.GetRootPlayerID(source);
        if (source)
        {
            eventData.source_type = source.GetType();
        }
        eventData.damage_type = damageType;
        eventData.component = component;
        eventData.damage_zone = dmgZone;
        eventData.ammo = ammo;
        eventData.model_position = modelPos;
        eventData.speed_coefficient = speedCoef;
        DBAProbeRuntime.QueueCombatEvent(eventData);
    }

    override void EEKilled(Object killer)
    {
        super.EEKilled(killer);

        if (!GetGame().IsServer())
        {
            return;
        }

        DBAProbeCombatEvent eventData = new DBAProbeCombatEvent;
        eventData.event_type = "PLAYER_KILLED";
        eventData.server_time_ms = GetGame().GetTime();
        eventData.target_player_id = DBAProbePlayerIdentity.GetPlayerID(this);
        eventData.source_player_id = DBAProbePlayerIdentity.GetRootPlayerID(killer);
        if (killer)
        {
            eventData.source_type = killer.GetType();
        }
        DBAProbeRuntime.QueueCombatEvent(eventData);
    }
};

class DBAProbePlayerIdentity
{
    static string GetPlayerID(PlayerBase player)
    {
        if (player && player.GetIdentity())
        {
            return player.GetIdentity().GetId();
        }
        return "";
    }

    static string GetRootPlayerID(Object source)
    {
        if (!source)
        {
            return "";
        }

        EntityAI sourceEntity = EntityAI.Cast(source);
        if (sourceEntity)
        {
            PlayerBase rootPlayer = PlayerBase.Cast(sourceEntity.GetHierarchyRootPlayer());
            return GetPlayerID(rootPlayer);
        }

        return GetPlayerID(PlayerBase.Cast(source));
    }
};
