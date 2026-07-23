modded class Weapon_Base
{
    override void EEFired(int muzzleType, int mode, string ammoType)
    {
        super.EEFired(muzzleType, mode, ammoType);

        if (!GetGame().IsServer())
        {
            return;
        }

        PlayerBase owner = PlayerBase.Cast(GetHierarchyRootPlayer());
        if (!owner || !owner.GetIdentity())
        {
            return;
        }

        DBAProbeCombatEvent eventData = new DBAProbeCombatEvent;
        eventData.event_type = "SHOT_FIRED_SERVER";
        eventData.server_time_ms = GetGame().GetTime();
        eventData.source_player_id = owner.GetIdentity().GetId();
        eventData.source_type = GetType();
        eventData.weapon_type = GetType();
        eventData.ammo = ammoType;
        eventData.muzzle_type = muzzleType;
        eventData.fire_mode = mode;
        eventData.world_position = owner.GetPosition();

        ItemBase suppressor = GetAttachedSuppressor();
        if (suppressor)
        {
            eventData.is_suppressed = true;
            eventData.suppressor_type = suppressor.GetType();
        }

        DBAProbeRuntime.QueueCombatEvent(eventData);
    }
};
