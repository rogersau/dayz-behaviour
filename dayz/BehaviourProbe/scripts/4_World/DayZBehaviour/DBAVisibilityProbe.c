enum DBAVisibilityClassification
{
    UNKNOWN = 0,
    HARD_OCCLUDED = 1,
    GEOMETRICALLY_EXPOSED = 2
};

class DBAVisibilityProbeResult
{
    int classification = DBAVisibilityClassification.UNKNOWN;
    vector observer_origin;
    vector target_head;
    vector target_torso;
    string blocker_type;
    int probe_count;
};

class DBAVisibilityProbe
{
    static DBAVisibilityProbeResult Probe(PlayerBase observer, PlayerBase target)
    {
        DBAVisibilityProbeResult output = new DBAVisibilityProbeResult;
        if (!observer || !target || observer == target || !observer.IsAlive() || !target.IsAlive())
        {
            return output;
        }

        MiscGameplayFunctions.GetHeadBonePos(observer, output.observer_origin);
        MiscGameplayFunctions.GetHeadBonePos(target, output.target_head);

        int torsoBone = target.GetBoneIndexByName("Spine3");
        if (torsoBone != -1)
        {
            output.target_torso = target.GetBonePositionWS(torsoBone);
        }
        else
        {
            output.target_torso = target.GetPosition() + "0 1 0";
        }

        bool headExposed;
        bool torsoExposed;
        string headBlocker;
        string torsoBlocker;

        headExposed = ProbePoint(observer, target, output.observer_origin, output.target_head, headBlocker);
        output.probe_count++;
        torsoExposed = ProbePoint(observer, target, output.observer_origin, output.target_torso, torsoBlocker);
        output.probe_count++;

        if (headExposed || torsoExposed)
        {
            output.classification = DBAVisibilityClassification.GEOMETRICALLY_EXPOSED;
            return output;
        }

        if (headBlocker != "" && torsoBlocker != "")
        {
            output.classification = DBAVisibilityClassification.HARD_OCCLUDED;
            output.blocker_type = headBlocker;
            if (headBlocker != torsoBlocker)
            {
                output.blocker_type = headBlocker + "," + torsoBlocker;
            }
        }

        return output;
    }

    protected static bool ProbePoint(
        PlayerBase observer,
        PlayerBase target,
        vector origin,
        vector point,
        out string blockerType
    )
    {
        blockerType = "";

        RaycastRVParams parameters = new RaycastRVParams(origin, point, observer);
        parameters.type = ObjIntersectView;
        parameters.flags = CollisionFlags.NEARESTCONTACT;

        array<ref RaycastRVResult> results = new array<ref RaycastRVResult>;
        array<Object> excluded = new array<Object>;
        excluded.Insert(observer);

        bool hit = DayZPhysics.RaycastRVProxy(parameters, results, excluded);
        if (!hit || results.Count() == 0)
        {
            return true;
        }

        RaycastRVResult first = results.Get(0);
        Object hitObject = first.parent;
        if (!hitObject)
        {
            hitObject = first.obj;
        }

        if (hitObject == target)
        {
            return true;
        }

        EntityAI hitEntity = EntityAI.Cast(hitObject);
        if (hitEntity && hitEntity.GetHierarchyRootPlayer() == target)
        {
            return true;
        }

        if (hitObject)
        {
            blockerType = hitObject.GetType();
        }
        else
        {
            blockerType = "TERRAIN_OR_WORLD";
        }
        return false;
    }
};
