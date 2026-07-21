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
    string observer_origin_mode = "PLAYER_HEAD_APPROXIMATION";
    string head_result = "NOT_PROBED";
    string torso_result = "NOT_PROBED";
    string pelvis_result = "NOT_PROBED";
    string left_upper_result = "NOT_PROBED";
    string right_upper_result = "NOT_PROBED";
    int points_clear;
    int points_hard_blocked;
    int points_ambiguous;
    bool bone_validity = true;

    void CountPointResult(string pointResult)
    {
        if (pointResult == "CLEAR")
        {
            points_clear++;
        }
        else if (pointResult == "HARD_BLOCKED")
        {
            points_hard_blocked++;
        }
        else
        {
            points_ambiguous++;
        }
    }

    string GetClassificationName()
    {
        if (classification == DBAVisibilityClassification.GEOMETRICALLY_EXPOSED)
        {
            return "EXPOSED";
        }
        if (classification == DBAVisibilityClassification.HARD_OCCLUDED)
        {
            return "HEAD_ORIGIN_OCCLUDED";
        }
        return "AMBIGUOUS";
    }
};

class DBAProbeRequest
{
    PlayerBase observer;
    PlayerBase target;
    string sampling_stream;
    string sampling_reason;
    int observer_eligible_count;
    float observer_inclusion_probability;
    int target_eligible_count;
    float target_inclusion_probability;
    bool risk_set_complete;
    float queue_admission_probability;
    int queued_ms;
    int not_before_ms;
    int first_occluded_ms;
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

        string headBlocker;
        string torsoBlocker;

        output.head_result = ProbePoint(observer, target, output.observer_origin, output.target_head, headBlocker);
        output.probe_count++;
        output.torso_result = ProbePoint(observer, target, output.observer_origin, output.target_torso, torsoBlocker);
        output.probe_count++;
        output.CountPointResult(output.head_result);
        output.CountPointResult(output.torso_result);

        if (output.head_result == "CLEAR" || output.torso_result == "CLEAR")
        {
            output.classification = DBAVisibilityClassification.GEOMETRICALLY_EXPOSED;
            return output;
        }

        if (output.head_result == "HARD_BLOCKED" && output.torso_result == "HARD_BLOCKED")
        {
            output.blocker_type = headBlocker;
            if (headBlocker != torsoBlocker)
            {
                output.blocker_type = headBlocker + "," + torsoBlocker;
            }
            ProbeAdditionalPoints(observer, target, output);
            if (output.points_clear > 0)
            {
                output.classification = DBAVisibilityClassification.GEOMETRICALLY_EXPOSED;
            }
            else if (output.points_ambiguous == 0)
            {
                output.classification = DBAVisibilityClassification.HARD_OCCLUDED;
            }
        }

        return output;
    }

    protected static void ProbeAdditionalPoints(PlayerBase observer, PlayerBase target, DBAVisibilityProbeResult output)
    {
        array<string> boneNames = {"Pelvis", "LeftArm", "RightArm"};
        for (int index = 0; index < boneNames.Count(); index++)
        {
            int boneIndex = target.GetBoneIndexByName(boneNames.Get(index));
            string pointResult = "AMBIGUOUS";
            string blocker;
            if (boneIndex != -1)
            {
                vector point = target.GetBonePositionWS(boneIndex);
                pointResult = ProbePoint(observer, target, output.observer_origin, point, blocker);
                output.probe_count++;
            }
            else
            {
                output.bone_validity = false;
            }
            if (index == 0)
            {
                output.pelvis_result = pointResult;
            }
            else if (index == 1)
            {
                output.left_upper_result = pointResult;
            }
            else
            {
                output.right_upper_result = pointResult;
            }
            output.CountPointResult(pointResult);
        }
    }

    protected static string ProbePoint(
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
            return "CLEAR";
        }

        RaycastRVResult first = results.Get(0);
        Object hitObject = first.parent;
        if (!hitObject)
        {
            hitObject = first.obj;
        }

        if (hitObject == target)
        {
            return "CLEAR";
        }

        EntityAI hitEntity = EntityAI.Cast(hitObject);
        if (hitEntity && hitEntity.GetHierarchyRootPlayer() == target)
        {
            return "CLEAR";
        }

        if (hitObject)
        {
            blockerType = hitObject.GetType();
        }
        else
        {
            blockerType = "TERRAIN_OR_WORLD";
        }
        if (IsAmbiguousBlocker(blockerType))
        {
            return "AMBIGUOUS";
        }
        return "HARD_BLOCKED";
    }

    protected static bool IsAmbiguousBlocker(string blockerType)
    {
        return blockerType.Contains("Bush") || blockerType.Contains("Tree") || blockerType.Contains("Plant") || blockerType.Contains("Grass") || blockerType.Contains("Vehicle");
    }
};
