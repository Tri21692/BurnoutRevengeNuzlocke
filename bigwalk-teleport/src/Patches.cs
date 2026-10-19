using System;
using HarmonyLib;
using UnityEngine;

namespace BigWalkTeleport
{
    // Harmony patches, applied when the mod loads (see Entry.cs). Each one swallows its own errors so a
    // problem here can't break the game's movement or input.

    // Runs our flight / moon / rocket physics right after the game's own movement step.
    [HarmonyPatch(typeof(PlayerMover), nameof(PlayerMover.FixedUpdate))]
    static class PlayerMoverFixedUpdatePatch
    {
        static bool failed;

        static void Postfix(PlayerMover __instance)
        {
            if (failed || Mod.Instance == null) return;
            try { Mod.Instance.AfterMoverFixedUpdate(__instance); }
            catch (Exception e) { failed = true; ModLog.Error($"Movement patch failed and is now off: {e}"); }
        }
    }

    // Adds the head swivel to the head direction the game sends to everyone else.
    [HarmonyPatch(typeof(PlayerNetworking), nameof(PlayerNetworking.CmdSetHeadState))]
    static class CmdSetHeadStatePatch
    {
        static void Prefix(ref Vector2 __0)
        {
            try { if (Mod.Instance != null) __0 = Mod.Instance.ModifyOutgoingHeadState(__0); }
            catch { }
        }
    }

    // Auto-walk: holds the local player's forward axis down.
    [HarmonyPatch(typeof(RW.Player), nameof(RW.Player.GetAxis), new[] { typeof(int) })]
    static class RewiredGetAxisPatch
    {
        const int MoveY = 8;

        static void Postfix(RW.Player __instance, int __0, ref float __result)
        {
            if (__0 != MoveY) return;
            var mod = Mod.Instance;
            if (mod == null || mod.localInputPlayer == IntPtr.Zero || __instance.Pointer != mod.localInputPlayer) return;
            try { __result = mod.ModifyForwardAxis(__result); }
            catch { }
        }
    }
}
