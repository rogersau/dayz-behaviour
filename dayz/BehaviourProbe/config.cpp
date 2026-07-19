class CfgPatches
{
    class DayZBehaviourProbe
    {
        units[] = {};
        weapons[] = {};
        requiredVersion = 0.1;
        requiredAddons[] = {"DZ_Data", "DZ_Scripts"};
    };
};

class CfgMods
{
    class DayZBehaviourProbe
    {
        dir = "DayZBehaviourProbe";
        name = "DayZ Behaviour Probe";
        author = "rogersau";
        version = "0.1.0";
        type = "mod";
        dependencies[] = {"Game", "World", "Mission"};

        class defs
        {
            class gameScriptModule
            {
                value = "";
                files[] = {"DayZBehaviourProbe/scripts/3_Game"};
            };

            class worldScriptModule
            {
                value = "";
                files[] = {"DayZBehaviourProbe/scripts/4_World"};
            };

            class missionScriptModule
            {
                value = "";
                files[] = {"DayZBehaviourProbe/scripts/5_Mission"};
            };
        };
    };
};
