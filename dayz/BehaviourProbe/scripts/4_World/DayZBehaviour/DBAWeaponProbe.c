modded class Weapon_Base
{
    override void OnFire(int muzzle_index)
    {
        super.OnFire(muzzle_index);

        PlayerBase owner = PlayerBase.Cast(GetHierarchyRootPlayer());
        string ownerID = "";
        if (owner && owner.GetIdentity())
        {
            ownerID = owner.GetIdentity().GetId();
        }

        string message = "[DayZBehaviourProbe] Weapon_Base.OnFire side server=" + GetGame().IsServer().ToString();
        message += " dedicated=" + GetGame().IsDedicatedServer().ToString();
        message += " owner=" + ownerID;
        message += " weapon=" + GetType();
        message += " muzzle=" + muzzle_index.ToString();
        Print(message);

        if (!GetGame().IsDedicatedServer() && owner && owner == GetGame().GetPlayer())
        {
            DBAProbeRuntime.MarkLocalShot(muzzle_index);
        }
    }
};
