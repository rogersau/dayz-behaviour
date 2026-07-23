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
    ref DBAVisibilityRayEvidenceV1 ray_evidence = new DBAVisibilityRayEvidenceV1;

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
    int first_probe_completed_ms;
    ref DBAEventContextV1 first_event_context;
    ref DBAVisibilityRayEvidenceV1 first_ray_evidence;
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

        output.head_result = ProbePoint("HEAD", observer, target, output.observer_origin, output.target_head, output, headBlocker);
        output.probe_count++;
        output.torso_result = ProbePoint("TORSO", observer, target, output.observer_origin, output.target_torso, output, torsoBlocker);
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
        array<string> pointNames = {"PELVIS", "LEFT_UPPER", "RIGHT_UPPER"};
        for (int index = 0; index < boneNames.Count(); index++)
        {
            int boneIndex = target.GetBoneIndexByName(boneNames.Get(index));
            string pointResult = "AMBIGUOUS";
            string blocker;
            if (boneIndex != -1)
            {
                vector point = target.GetBonePositionWS(boneIndex);
                pointResult = ProbePoint(pointNames.Get(index), observer, target, output.observer_origin, point, output, blocker);
                output.probe_count++;
            }
            else
            {
                output.bone_validity = false;
                RecordInvalidPoint(pointNames.Get(index), output.observer_origin, output);
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
        string pointName,
        PlayerBase observer,
        PlayerBase target,
        vector origin,
        vector point,
        DBAVisibilityProbeResult output,
        out string blockerType
    )
    {
        blockerType = "";

        DBAVisibilityPointEvidenceV1 evidence = new DBAVisibilityPointEvidenceV1;
        evidence.point_name = pointName;
        evidence.ray_origin = origin;
        evidence.ray_destination = point;

        RaycastRVParams parameters = new RaycastRVParams(origin, point, observer);
        parameters.type = ObjIntersectView;
        parameters.flags = CollisionFlags.NEARESTCONTACT;

        array<ref RaycastRVResult> results = new array<ref RaycastRVResult>;
        array<Object> excluded = new array<Object>;
        excluded.Insert(observer);

        bool hit = DayZPhysics.RaycastRVProxy(parameters, results, excluded);
        if (!hit || results.Count() == 0)
        {
            evidence.result = "CLEAR";
            output.ray_evidence.points.Insert(evidence);
            return evidence.result;
        }

        RaycastRVResult first = results.Get(0);
        evidence.contact_present = true;
        evidence.contact_position = first.pos;
        evidence.contact_direction = first.dir;
        evidence.contact_component = first.component;
        evidence.contact_hierarchy_level = first.hierLevel;
        evidence.contact_distance_metres = vector.Distance(origin, first.pos);

        Object hitObject = first.parent;
        if (!hitObject)
        {
            hitObject = first.obj;
        }

        if (hitObject)
        {
            evidence.contact_object_type = hitObject.GetType();
        }
        else
        {
            evidence.contact_object_type = "TERRAIN_OR_WORLD";
        }

        if (hitObject == target)
        {
            evidence.result = "CLEAR";
            output.ray_evidence.points.Insert(evidence);
            return evidence.result;
        }

        EntityAI hitEntity = EntityAI.Cast(hitObject);
        if (hitEntity && hitEntity.GetHierarchyRootPlayer() == target)
        {
            evidence.result = "CLEAR";
            output.ray_evidence.points.Insert(evidence);
            return evidence.result;
        }

        blockerType = evidence.contact_object_type;
        evidence.blocker_type = blockerType;
        if (IsAmbiguousBlocker(blockerType))
        {
            evidence.result = "AMBIGUOUS";
            output.ray_evidence.points.Insert(evidence);
            return evidence.result;
        }

        evidence.result = "HARD_BLOCKED";
        output.ray_evidence.points.Insert(evidence);
        return evidence.result;
    }

    protected static void RecordInvalidPoint(string pointName, vector origin, DBAVisibilityProbeResult output)
    {
        DBAVisibilityPointEvidenceV1 evidence = new DBAVisibilityPointEvidenceV1;
        evidence.point_name = pointName;
        evidence.ray_origin = origin;
        evidence.result = "AMBIGUOUS";
        evidence.blocker_type = "INVALID_BONE";
        output.ray_evidence.points.Insert(evidence);
    }

    protected static bool IsAmbiguousBlocker(string blockerType)
    {
        return blockerType.Contains("Bush") || blockerType.Contains("Tree") || blockerType.Contains("Plant") || blockerType.Contains("Grass") || blockerType.Contains("Vehicle");
    }
};
