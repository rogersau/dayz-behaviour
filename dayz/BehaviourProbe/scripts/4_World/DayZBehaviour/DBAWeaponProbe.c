modded class Weapon_Base
{
    override void OnFire(int muzzleIndex)
    {
        super.OnFire(muzzleIndex);

        PlayerBase owner = PlayerBase.Cast(GetHierarchyRootPlayer());
        string ownerID = "";
        if (owner && owner.GetIdentity())
        {
            ownerID = owner.GetIdentity().GetId();
        }

        Print(string.Format(
            "[DayZBehaviourProbe] Weapon_Base.OnFire side server=%1 dedicated=%2 owner=%3 weapon=%4 muzzle=%5",
            GetGame().IsServer(),
            GetGame().IsDedicatedServer(),
            ownerID,
            GetType(),
            muzzleIndex
        ));

        if (!GetGame().IsDedicatedServer() && owner && owner == GetGame().GetPlayer())
        {
            DBAProbeRuntime.MarkLocalShot(muzzleIndex);
        }
    }
};
