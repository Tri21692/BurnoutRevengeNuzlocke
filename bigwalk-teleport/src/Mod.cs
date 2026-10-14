using System;
using System.Collections.Generic;
using System.Globalization;
using Il2Cpp;
using MelonLoader;
using UnityEngine;

[assembly: MelonInfo(typeof(BigWalkTeleport.Mod), "Big Walk Teleport", "1.0.0", "tri21692")]
[assembly: MelonGame] // any game: Big Walk's exact company/product strings aren't known

namespace BigWalkTeleport
{
    public class Mod : MelonMod
    {
        const int SlotCount = 5;

        MelonPreferences_Entry<KeyCode> menuKey;
        MelonPreferences_Entry<KeyCode> hudKey;
        MelonPreferences_Entry<KeyCode> invisibleKey;
        MelonPreferences_Entry<bool> showHud;
        MelonPreferences_Entry<float> groundHeightOffset;
        MelonPreferences_Entry<KeyCode> discoKey;
        MelonPreferences_Entry<float> discoInterval;
        readonly MelonPreferences_Entry<string>[] slots = new MelonPreferences_Entry<string>[SlotCount];

        bool menuOpen;
        string inputX = "", inputY = "", inputZ = "";
        string status = "";
        CursorLockMode savedLockState;
        bool savedCursorVisible;

        bool invisible;
        float nextRendererRefresh;
        readonly Dictionary<int, Renderer> hiddenRenderers = new();

        public override void OnInitializeMelon()
        {
            var cat = MelonPreferences.CreateCategory("BigWalkTeleport", "Big Walk Teleport");
            menuKey = cat.CreateEntry("MenuKey", KeyCode.F8, "Key that opens the teleport window");
            hudKey = cat.CreateEntry("HudKey", KeyCode.F7, "Key that toggles the coordinate readout");
            invisibleKey = cat.CreateEntry("InvisibleKey", KeyCode.I, "Key that toggles your player model on and off");
            showHud = cat.CreateEntry("ShowHud", false, "Show your coordinates in the corner at all times");
            discoKey = cat.CreateEntry("DiscoKey", KeyCode.O, "Key that toggles outfit disco on yourself (host only)");
            discoInterval = cat.CreateEntry("DiscoInterval", 0.6f, "Seconds between outfit changes during disco");
            groundHeightOffset = cat.CreateEntry("GroundHeightOffset", 1.5f, "How far above the ground to land when Y is left blank");
            for (int i = 0; i < SlotCount; i++)
                slots[i] = cat.CreateEntry($"Slot{i + 1}", "", $"Saved spot {i + 1} as x,y,z");
        }

        public override void OnUpdate()
        {
            if (Input.GetKeyDown(menuKey.Value)) ToggleMenu();
            if (Input.GetKeyDown(hudKey.Value)) showHud.Value = !showHud.Value;
            // Don't let typing an "i" into a coordinate box toggle invisibility.
            bool typing = menuOpen && GUIUtility.keyboardControl != 0;
            if (!typing && Input.GetKeyDown(invisibleKey.Value)) SetInvisible(!invisible);
            if (!typing && Input.GetKeyDown(discoKey.Value)) ToggleDisco(everyone: false);

            if (menuOpen)
            {
                Cursor.lockState = CursorLockMode.None;
                Cursor.visible = true;
            }

            if (invisible && Time.unscaledTime >= nextRendererRefresh)
                RefreshHiddenRenderers();

            LogTeleportProgress();
            UpdateDisco();
        }

        void ToggleMenu()
        {
            menuOpen = !menuOpen;
            if (menuOpen)
            {
                savedLockState = Cursor.lockState;
                savedCursorVisible = Cursor.visible;
                var pc = FindLocalPlayer();
                if (pc != null) FillInputs(PlayerPosition(pc));
            }
            else
            {
                GUIUtility.keyboardControl = 0;
                Cursor.lockState = savedLockState;
                Cursor.visible = savedCursorVisible;
            }
        }

        // ---------------------------------------------------------------- player

        static PlayerCharacter FindLocalPlayer()
        {
            var all = PlayerCharacter.allPlayerCharacters;
            if (all == null) return null;
            PlayerCharacter owned = null, only = null;
            for (int i = 0; i < all.Count; i++)
            {
                var pc = all[i];
                if (pc == null) continue;
                if (pc.isLocalPlayer) return pc;
                if (owned == null && (pc.isOwned || (pc.houseNetworkTransform != null && pc.houseNetworkTransform.isOwned))) owned = pc;
                only = all.Count == 1 ? pc : null;
            }
            return owned ?? only;
        }

        // The physics body ("kernal") is what PlayerMover tracks; fall back to the root if it's missing.
        static Transform Body(PlayerCharacter pc) =>
            pc.kernal != null ? pc.kernal : pc.rb != null ? pc.rb.transform : pc.transform;

        static Vector3 PlayerPosition(PlayerCharacter pc) => Body(pc).position;

        // Teleports are re-applied for a few physics steps in case the game pulls the body back.
        const int HoldFixedSteps = 6;
        Vector3 teleportTarget;
        int holdStepsLeft;
        int diagnosticFrame = -1;

        void Teleport(Vector3 target)
        {
            var pc = FindLocalPlayer();
            if (pc == null)
            {
                int count = PlayerCharacter.allPlayerCharacters?.Count ?? -1;
                status = "No local player found (are you in the world?)";
                MelonLogger.Warning($"{status} allPlayerCharacters.Count={count}");
                return;
            }

            LogPlayer(pc, "before");
            ApplyTeleport(pc, target);
            LogPlayer(pc, "after");

            teleportTarget = target;
            holdStepsLeft = HoldFixedSteps;
            diagnosticFrame = 0;
            status = $"Teleported to {Format(target)}";
            MelonLogger.Msg(status);
        }

        static void ApplyTeleport(PlayerCharacter pc, Vector3 target)
        {
            // Positions are synced relative to whatever platform you're standing on; step off it first.
            var net = pc.houseNetworkTransform;
            if (net != null && net.stickyPlatform != null) net.stickyPlatform = null;

            var root = pc.transform;
            var body = Body(pc);
            var delta = target - body.position;

            // Shift root and body by the same amount so neither drags the other back.
            root.position += delta;
            if (body != root && !body.IsChildOf(root)) body.position += delta;
            var rb = pc.rb;
            if (rb != null && rb.transform != root && rb.transform != body && !rb.transform.IsChildOf(root) && !rb.transform.IsChildOf(body))
                rb.transform.position += delta;

            if (rb != null)
            {
                rb.position = rb.transform.position;
                rb.linearVelocity = Vector3.zero;
                rb.angularVelocity = Vector3.zero;
            }

            // PlayerMover works out your speed from how far the body moved since last step;
            // without this the jump reads as a huge velocity.
            var mover = pc.mover;
            if (mover != null)
            {
                mover.cachedKernalPos = body.position;
                mover.localKernalVelocity = Vector3.zero;
            }

            Physics.SyncTransforms();
        }

        public override void OnFixedUpdate()
        {
            if (holdStepsLeft <= 0) return;
            holdStepsLeft--;
            var pc = FindLocalPlayer();
            if (pc == null) { holdStepsLeft = 0; return; }
            float drift = Vector3.Distance(PlayerPosition(pc), teleportTarget);
            if (drift > 2f)
            {
                MelonLogger.Msg($"Teleport drifted {drift:0.0}m on hold step {HoldFixedSteps - holdStepsLeft}; re-applying");
                ApplyTeleport(pc, teleportTarget);
            }
        }

        void LogTeleportProgress()
        {
            if (diagnosticFrame < 0) return;
            diagnosticFrame++;
            if (diagnosticFrame != 1 && diagnosticFrame != 10 && diagnosticFrame != 60 && diagnosticFrame != 300) return;
            var pc = FindLocalPlayer();
            if (pc != null)
                MelonLogger.Msg($"[frame {diagnosticFrame}] body {Format(PlayerPosition(pc))}, {Vector3.Distance(PlayerPosition(pc), teleportTarget):0.0}m from target");
            if (diagnosticFrame >= 300) diagnosticFrame = -1;
        }

        static void LogPlayer(PlayerCharacter pc, string when)
        {
            var rb = pc.rb;
            var body = Body(pc);
            MelonLogger.Msg(
                $"[{when}] '{pc.name}' local={pc.isLocalPlayer} owned={pc.isOwned} " +
                $"root={Format(pc.transform.position)} kernal={(pc.kernal != null ? $"'{pc.kernal.name}' {Format(pc.kernal.position)} childOfRoot={pc.kernal.IsChildOf(pc.transform)}" : "null")} " +
                $"rb={(rb != null ? $"'{rb.name}' {Format(rb.position)} kinematic={rb.isKinematic} interp={rb.interpolation}" : "null")} " +
                $"cachedKernalPos={(pc.mover != null ? Format(pc.mover.cachedKernalPos) : "n/a")} body={Format(body.position)}");
        }

        bool TryGroundHeight(float x, float z, out float y)
        {
            if (Physics.Raycast(new Vector3(x, 3000f, z), Vector3.down, out RaycastHit hit, 6000f, ~0, QueryTriggerInteraction.Ignore))
            {
                y = hit.point.y + groundHeightOffset.Value;
                return true;
            }
            y = 0;
            return false;
        }

        // ---------------------------------------------------------- outfit disco
        // Outfits are owned by the host: a guest's "change my look" request is empty in the
        // release build, so this only works while you're hosting. ServerSetLook with
        // saveChange=false syncs the colour to everyone (with the game's puff effect) without
        // touching anyone's saved outfit, and the originals are put back when it stops.

        bool discoActive, discoEveryone;
        float nextDiscoTick;
        readonly Dictionary<IntPtr, (PlayerCharacter pc, int head, int torso, int legs)> discoOriginals = new();
        readonly System.Random discoRandom = new();

        void ToggleDisco(bool everyone)
        {
            if (discoActive)
            {
                bool wasEveryone = discoEveryone;
                StopDisco();
                if (wasEveryone == everyone) return;
            }

            var me = FindLocalPlayer();
            if (me == null) { status = "No local player found"; return; }
            if (!me.isServer) { status = "Outfit disco only works when you're hosting"; MelonLogger.Msg(status); return; }

            discoActive = true;
            discoEveryone = everyone;
            nextDiscoTick = 0;
            status = everyone ? "Outfit disco: everyone!" : "Outfit disco!";
            MelonLogger.Msg(status);
        }

        void StopDisco()
        {
            discoActive = false;
            foreach (var o in discoOriginals.Values)
            {
                var net = o.pc != null ? o.pc.playerNetworking : null;
                if (net == null) continue;
                net.ServerSetLook(o.head, PlayerLooks.LookPart.Head, false);
                net.ServerSetLook(o.torso, PlayerLooks.LookPart.Torso, false);
                net.ServerSetLook(o.legs, PlayerLooks.LookPart.Legs, false);
            }
            discoOriginals.Clear();
            status = "Disco over, outfits restored";
            MelonLogger.Msg(status);
        }

        void UpdateDisco()
        {
            if (!discoActive || Time.unscaledTime < nextDiscoTick) return;
            nextDiscoTick = Time.unscaledTime + Mathf.Max(0.1f, discoInterval.Value);

            var me = FindLocalPlayer();
            if (me == null || !me.isServer) { StopDisco(); return; }

            if (!discoEveryone) { DiscoStep(me); return; }
            var all = PlayerCharacter.allPlayerCharacters;
            for (int i = 0; all != null && i < all.Count; i++)
                if (all[i] != null) DiscoStep(all[i]);
        }

        void DiscoStep(PlayerCharacter pc)
        {
            var net = pc.playerNetworking;
            var colors = pc.looks?.lookSet?.colors;
            if (net == null || colors == null || colors.Length < 2) return;

            if (!discoOriginals.ContainsKey(pc.Pointer))
                discoOriginals[pc.Pointer] = (pc, net.lookIdHead, net.lookIdTorso, net.lookIdLegs);

            net.ServerSetLook(RandomColor(colors, net.lookIdHead), PlayerLooks.LookPart.Head, false);
            net.ServerSetLook(RandomColor(colors, net.lookIdTorso), PlayerLooks.LookPart.Torso, false);
            net.ServerSetLook(RandomColor(colors, net.lookIdLegs), PlayerLooks.LookPart.Legs, false);
        }

        // Skips reward-only colours so the disco doesn't hand out unlocks, and avoids repeating the current one.
        int RandomColor(Il2CppInterop.Runtime.InteropTypes.Arrays.Il2CppStructArray<PlayerLookSet.LookColor> colors, int current)
        {
            for (int tries = 0; tries < 20; tries++)
            {
                int id = discoRandom.Next(colors.Length);
                if (id != current && !colors[id].rewardOnly) return id;
            }
            return current;
        }

        // ---------------------------------------------------------- invisibility

        void SetInvisible(bool value)
        {
            invisible = value;
            if (invisible)
            {
                RefreshHiddenRenderers();
                status = "Player model hidden (only on your screen)";
            }
            else
            {
                foreach (var r in hiddenRenderers.Values)
                    if (r != null) r.forceRenderingOff = false;
                hiddenRenderers.Clear();
                status = "Player model visible";
            }
            MelonLogger.Msg(status);
        }

        // Re-scanned regularly because the game swaps outfit parts and parents props to your hands.
        void RefreshHiddenRenderers()
        {
            nextRendererRefresh = Time.unscaledTime + 0.5f;
            var pc = FindLocalPlayer();
            var current = new Dictionary<int, Renderer>();
            if (pc != null)
            {
                Collect(pc.transform, current);
                if (pc.kernal != null && !pc.kernal.IsChildOf(pc.transform)) Collect(pc.kernal, current);
            }

            foreach (var kv in hiddenRenderers)
                if (!current.ContainsKey(kv.Key) && kv.Value != null) kv.Value.forceRenderingOff = false;

            hiddenRenderers.Clear();
            foreach (var kv in current)
            {
                kv.Value.forceRenderingOff = true;
                hiddenRenderers[kv.Key] = kv.Value;
            }
        }

        static void Collect(Transform root, Dictionary<int, Renderer> into)
        {
            var renderers = root.GetComponentsInChildren<Renderer>(true);
            for (int i = 0; i < renderers.Length; i++)
            {
                var r = renderers[i];
                if (r != null) into[r.GetInstanceID()] = r;
            }
        }

        // -------------------------------------------------------------------- UI

        public override void OnGUI()
        {
            var pc = FindLocalPlayer();

            if (showHud.Value && pc != null)
            {
                var pos = PlayerPosition(pc);
                GUI.Box(new Rect(10, 10, 280, 26), "");
                GUI.Label(new Rect(18, 13, 270, 22), $"X {pos.x:0.00}   Y {pos.y:0.00}   Z {pos.z:0.00}");
            }

            if (!menuOpen) return;

            float w = 380, x0 = Screen.width - w - 20, y = 20;
            float h = 282 + SlotCount * 28;
            GUI.Box(new Rect(x0, y, w, h), "Big Walk Teleport");
            float x = x0 + 12;
            y += 28;

            GUI.Label(new Rect(x, y, w - 24, 22), pc != null ? $"You are at  {Format(PlayerPosition(pc))}" : "Not in a world yet");
            y += 28;

            GUI.Label(new Rect(x, y, 20, 22), "X");
            inputX = GUI.TextField(new Rect(x + 18, y, 95, 22), inputX);
            GUI.Label(new Rect(x + 120, y, 20, 22), "Y");
            inputY = GUI.TextField(new Rect(x + 138, y, 95, 22), inputY);
            GUI.Label(new Rect(x + 240, y, 20, 22), "Z");
            inputZ = GUI.TextField(new Rect(x + 258, y, 95, 22), inputZ);
            y += 26;
            GUI.Label(new Rect(x, y, w - 24, 22), "Leave Y blank to land on the ground. You can paste \"x, y, z\" into X.");
            y += 28;

            if (GUI.Button(new Rect(x, y, 170, 26), "Teleport")) TeleportFromInputs();
            if (GUI.Button(new Rect(x + 186, y, 170, 26), "Use my position") && pc != null) FillInputs(PlayerPosition(pc));
            y += 32;
            if (GUI.Button(new Rect(x, y, 170, 26), invisible ? "Show my model (I)" : "Hide my model (I)")) SetInvisible(!invisible);
            if (GUI.Button(new Rect(x + 186, y, 170, 26), "Close (F8)")) ToggleMenu();
            y += 32;
            if (GUI.Button(new Rect(x, y, 170, 26), discoActive && !discoEveryone ? "Stop disco (O)" : "Outfit disco: me (O)")) ToggleDisco(everyone: false);
            if (GUI.Button(new Rect(x + 186, y, 170, 26), discoActive && discoEveryone ? "Stop disco" : "Outfit disco: everyone")) ToggleDisco(everyone: true);
            y += 36;

            GUI.Label(new Rect(x, y, w - 24, 22), "Saved spots");
            y += 24;
            for (int i = 0; i < SlotCount; i++)
            {
                bool has = TryParseVector(slots[i].Value, out var saved);
                GUI.Label(new Rect(x, y, 190, 22), $"{i + 1}.  {(has ? Format(saved) : "(empty)")}");
                if (GUI.Button(new Rect(x + 196, y, 50, 22), "Save") && pc != null)
                {
                    slots[i].Value = Serialize(PlayerPosition(pc));
                    MelonPreferences.Save();
                    status = $"Saved spot {i + 1}";
                }
                if (has && GUI.Button(new Rect(x + 250, y, 50, 22), "Go")) Teleport(saved);
                if (has && GUI.Button(new Rect(x + 304, y, 50, 22), "Load")) FillInputs(saved);
                y += 28;
            }

            if (status.Length > 0) GUI.Label(new Rect(x, y + 4, w - 24, 22), status);
        }

        void TeleportFromInputs()
        {
            if (TryParseVector(inputX, out var pasted) && inputY.Trim().Length == 0 && inputZ.Trim().Length == 0)
            {
                Teleport(pasted);
                return;
            }

            if (!TryParseFloat(inputX, out float x) || !TryParseFloat(inputZ, out float z))
            {
                status = "X and Z need to be numbers";
                return;
            }

            float y;
            if (inputY.Trim().Length == 0)
            {
                if (!TryGroundHeight(x, z, out y)) { status = "No ground found there; enter a Y value"; return; }
            }
            else if (!TryParseFloat(inputY, out y))
            {
                status = "Y needs to be a number (or blank)";
                return;
            }

            Teleport(new Vector3(x, y, z));
        }

        void FillInputs(Vector3 v)
        {
            inputX = v.x.ToString("0.00", CultureInfo.InvariantCulture);
            inputY = v.y.ToString("0.00", CultureInfo.InvariantCulture);
            inputZ = v.z.ToString("0.00", CultureInfo.InvariantCulture);
        }

        // --------------------------------------------------------------- parsing

        static string Format(Vector3 v) => $"{v.x:0.0}, {v.y:0.0}, {v.z:0.0}";

        static string Serialize(Vector3 v) => string.Format(CultureInfo.InvariantCulture, "{0:0.00},{1:0.00},{2:0.00}", v.x, v.y, v.z);

        static bool TryParseFloat(string s, out float f) =>
            float.TryParse(s.Trim().Replace(',', '.'), NumberStyles.Float, CultureInfo.InvariantCulture, out f);

        static bool TryParseVector(string s, out Vector3 v)
        {
            v = default;
            if (string.IsNullOrWhiteSpace(s)) return false;
            var parts = s.Split(new[] { ',', ' ', ';' }, StringSplitOptions.RemoveEmptyEntries);
            if (parts.Length != 3) return false;
            if (!float.TryParse(parts[0], NumberStyles.Float, CultureInfo.InvariantCulture, out float x)) return false;
            if (!float.TryParse(parts[1], NumberStyles.Float, CultureInfo.InvariantCulture, out float y)) return false;
            if (!float.TryParse(parts[2], NumberStyles.Float, CultureInfo.InvariantCulture, out float z)) return false;
            v = new Vector3(x, y, z);
            return true;
        }
    }
}
