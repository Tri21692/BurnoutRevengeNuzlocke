// Loader entry points. Only one of these is compiled into each build.
#if BEPINEX
using System;
using BepInEx;
using BepInEx.Unity.IL2CPP;
using HarmonyLib;
using UnityEngine;

namespace BigWalkTeleport
{
    [BepInPlugin(Guid, "Big Walk Teleport", Version)]
    public class BepInExPlugin : BasePlugin
    {
        public const string Guid = "tri21692.BigWalkTeleport";
        public const string Version = "1.1.0";

        public override void Load()
        {
            ModLog.Source = Log;
            var mod = new Mod();
            mod.Init(new SettingsCategory(Config));
            new Harmony(Guid).PatchAll(typeof(BepInExPlugin).Assembly);
            // BepInEx has no per-frame callbacks of its own; this component forwards Unity's.
            AddComponent<ModRunner>();
            Log.LogInfo($"Big Walk Teleport {Version} loaded");
        }
    }

    public class ModRunner : MonoBehaviour
    {
        public ModRunner(IntPtr ptr) : base(ptr) { }

        void Update() => Mod.Instance?.Update();
        void FixedUpdate() => Mod.Instance?.FixedUpdate();
        void OnGUI() => Mod.Instance?.Gui();
    }
}
#else
using MelonLoader;

[assembly: MelonInfo(typeof(BigWalkTeleport.MelonEntry), "Big Walk Teleport", "1.1.0", "tri21692")]
[assembly: MelonGame("House House", "Big Walk")]

namespace BigWalkTeleport
{
    // MelonLoader applies the [HarmonyPatch] classes in this assembly on its own.
    public class MelonEntry : MelonMod
    {
        Mod mod;

        public override void OnInitializeMelon()
        {
            mod = new Mod();
            mod.Init(new SettingsCategory(MelonPreferences.CreateCategory("BigWalkTeleport", "Big Walk Teleport")));
        }

        public override void OnUpdate() => mod?.Update();
        public override void OnFixedUpdate() => mod?.FixedUpdate();
        public override void OnGUI() => mod?.Gui();
    }
}
#endif
