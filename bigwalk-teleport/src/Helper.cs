using Il2Cpp;
using MelonLoader;
using UnityEngine;

namespace BigWalkTeleport
{
    // Helper pack: friend markers, teleport to a friend, and auto-walk.
    public partial class Mod
    {
        MelonPreferences_Entry<KeyCode> markersKey, autoWalkKey;
        MelonPreferences_Entry<bool> showMarkers;

        bool autoWalk;
        // Rewired player the game reads your movement from; the auto-walk patch only touches this one.
        internal System.IntPtr localInputPlayer;

        void CreateHelperPrefs(MelonPreferences_Category cat)
        {
            markersKey = cat.CreateEntry("MarkersKey", KeyCode.F6, "Toggle friend markers (name and distance, through walls)");
            autoWalkKey = cat.CreateEntry("AutoWalkKey", KeyCode.F10, "Toggle auto-walk (walk forward hands-free; pull back to stop)");
            showMarkers = cat.CreateEntry("ShowMarkers", false, "Show friend markers");
        }

        void UpdateHelper(bool typing)
        {
            var pc = FindLocalPlayer();
            localInputPlayer = pc?.inputPlayer != null ? pc.inputPlayer.Pointer : System.IntPtr.Zero;
            if (typing) return;
            if (KeyDown(markersKey)) { showMarkers.Value = !showMarkers.Value; Report(showMarkers.Value ? "Friend markers ON" : "Friend markers off"); }
            if (KeyDown(autoWalkKey)) SetAutoWalk(!autoWalk);
        }

        void SetAutoWalk(bool on)
        {
            autoWalk = on;
            Report(on ? "Auto-walk ON (pull back or press again to stop)" : "Auto-walk off");
        }

        // Called from the Rewired Player.GetAxis patch for the local player's forward axis.
        internal float ModifyForwardAxis(float value)
        {
            if (!autoWalk) return value;
            if (value < -0.3f) { autoWalk = false; Report("Auto-walk off"); return value; }
            return Mathf.Max(value, 1f);
        }

        static string PlayerName(PlayerCharacter pc)
        {
            var name = pc.playerNetworking?.username;
            return string.IsNullOrEmpty(name) ? "player" : name;
        }

        void DrawFriendMarkers()
        {
            if (!showMarkers.Value) return;
            var me = FindLocalPlayer();
            var cam = PlayerCamera();
            var all = PlayerCharacter.allPlayerCharacters;
            if (me == null || cam == null || all == null) return;
            var myPos = PlayerPosition(me);

            for (int i = 0; i < all.Count; i++)
            {
                var pc = all[i];
                if (pc == null || pc.Pointer == me.Pointer) continue;
                var pos = PlayerPosition(pc) + Vector3.up * 1.2f;
                var sp = cam.WorldToScreenPoint(pos);
                // Behind the camera: flip and pin to the edge so you can still tell which way to turn.
                if (sp.z < 0) { sp.x = Screen.width - sp.x; sp.y = Screen.height - sp.y; }
                float sx = Mathf.Clamp(sp.x, 60, Screen.width - 60);
                float sy = Mathf.Clamp(Screen.height - sp.y, 20, Screen.height - 40);
                if (sp.z < 0) sy = Screen.height - 40;

                string text = $"{PlayerName(pc)}  {Vector3.Distance(myPos, pos):0}m";
                GUI.Box(new Rect(sx - 70, sy - 12, 140, 24), "");
                GUI.Label(new Rect(sx - 64, sy - 10, 132, 22), text);
            }
        }

        void TeleportToFriend(PlayerCharacter friend)
        {
            // Land a couple of metres behind them.
            var back = friend.transform.forward;
            back.y = 0;
            back = back.sqrMagnitude > 0.01f ? back.normalized : Vector3.forward;
            Teleport(PlayerPosition(friend) - back * 2.5f + Vector3.up * 0.5f);
        }

        float DrawHelperTab(float x, float y, float w)
        {
            if (Toggle(x, y, w, showMarkers.Value, $"Friend markers ({markersKey.Value})")) showMarkers.Value = !showMarkers.Value;
            y += 30;
            if (Toggle(x, y, w, autoWalk, $"Auto-walk ({autoWalkKey.Value})")) SetAutoWalk(!autoWalk);
            y += 34;

            GUI.Label(new Rect(x, y, w, 22), "Teleport to a friend:");
            y += 24;
            var me = FindLocalPlayer();
            var all = PlayerCharacter.allPlayerCharacters;
            int shown = 0;
            for (int i = 0; me != null && all != null && i < all.Count && shown < 8; i++)
            {
                var pc = all[i];
                if (pc == null || pc.Pointer == me.Pointer) continue;
                shown++;
                float dist = Vector3.Distance(PlayerPosition(me), PlayerPosition(pc));
                GUI.Label(new Rect(x, y, w - 60, 22), $"{PlayerName(pc)}  ({dist:0}m)");
                if (GUI.Button(new Rect(x + w - 54, y, 54, 22), "Go")) TeleportToFriend(pc);
                y += 26;
            }
            if (shown == 0) { GUI.Label(new Rect(x, y, w, 22), "(nobody else here)"); y += 26; }
            return y + 4;
        }
    }
}
