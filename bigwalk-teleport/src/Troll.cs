using Il2Cpp;
using MelonLoader;
using UnityEngine;

namespace BigWalkTeleport
{
    // Troll pack: narcolepsy, phantom typist, head swivel and kidnap teleports.
    // Everything here only changes your own character (or a friend you're carrying the normal way),
    // so it works as a guest.
    public partial class Mod
    {
        MelonPreferences_Entry<float> napMinGap, napMaxGap, napMinLength, napMaxLength, swivelAmount, swivelSpeed;

        readonly System.Random trollRandom = new();

        // ---------------------------------------------------------- narcolepsy
        // PlayerSleeper has a forceSleeping switch; flipping it lets the game play and sync its own sleep.

        bool narcolepsy;
        bool napping;
        float nextNapChange;

        void CreateTrollPrefs(MelonPreferences_Category cat)
        {
            napMinGap = cat.CreateEntry("NapMinGap", 40f, "Narcolepsy: shortest time awake between naps (seconds)");
            napMaxGap = cat.CreateEntry("NapMaxGap", 120f, "Narcolepsy: longest time awake between naps (seconds)");
            napMinLength = cat.CreateEntry("NapMinLength", 5f, "Narcolepsy: shortest nap (seconds)");
            napMaxLength = cat.CreateEntry("NapMaxLength", 12f, "Narcolepsy: longest nap (seconds)");
            swivelAmount = cat.CreateEntry("SwivelAmount", 90f, "Head swivel: how far your head turns each way");
            swivelSpeed = cat.CreateEntry("SwivelSpeed", 1.2f, "Head swivel: swings per second");
        }

        float RandomRange(float a, float b) => a + (float)trollRandom.NextDouble() * Mathf.Max(0, b - a);

        void SetNarcolepsy(bool on)
        {
            narcolepsy = on;
            if (on) nextNapChange = Time.time + RandomRange(5f, 15f);
            else SetNap(false);
            Report(on ? "Narcolepsy ON: you'll doze off at random" : "Narcolepsy off");
        }

        void SetNap(bool asleep)
        {
            napping = asleep;
            var sleeper = FindLocalPlayer()?.sleeper;
            if (sleeper != null) sleeper.forceSleeping = asleep;
        }

        void UpdateNarcolepsy()
        {
            if (!narcolepsy || Time.time < nextNapChange) return;
            SetNap(!napping);
            nextNapChange = Time.time + (napping
                ? RandomRange(napMinLength.Value, napMaxLength.Value)
                : RandomRange(napMinGap.Value, napMaxGap.Value));
            MelonLogger.Msg(napping ? "Narcolepsy: zzz" : "Narcolepsy: awake");
        }

        // ---------------------------------------------------------- phantom typist
        // Holds the "typing..." bubble on and sends typing keystroke sounds, but never a message.

        bool phantomTyping;
        float nextStroke;
        int burstLeft;

        void SetPhantomTyping(bool on)
        {
            var net = FindLocalPlayer()?.playerNetworking;
            if (net == null) { status = "No local player found"; return; }
            phantomTyping = on;
            net.CmdSetIsTextChatting(on);
            burstLeft = 0;
            Report(on ? "Phantom typist ON" : "Phantom typist off");
        }

        void UpdatePhantomTyping()
        {
            if (!phantomTyping || Time.time < nextStroke) return;
            var net = FindLocalPlayer()?.playerNetworking;
            if (net == null) { phantomTyping = false; return; }

            if (burstLeft <= 0)
            {
                // Pause like you're thinking, then type another burst.
                burstLeft = trollRandom.Next(6, 30);
                nextStroke = Time.time + RandomRange(0.8f, 3.5f);
                return;
            }
            burstLeft--;
            // Mostly letters, the odd backspace.
            net.CmdTextChatStroke(trollRandom.NextDouble() < 0.12);
            nextStroke = Time.time + RandomRange(0.06f, 0.22f);
        }

        // ---------------------------------------------------------- head swivel
        // The game sends where your head points (headState) when you look around. The patch in
        // Patches.cs adds a sideways swing to what gets sent, so others see your head swivel while
        // your own camera stays put. We also resend regularly so it keeps going when you hold still.

        bool headSwivel;
        float nextHeadSend;
        bool loggedHeadState;

        void SetHeadSwivel(bool on)
        {
            headSwivel = on;
            loggedHeadState = false;
            Report(on ? "Head swivel ON (others see it, your view stays normal)" : "Head swivel off");
            if (!on) ResendHead();
        }

        void UpdateHeadSwivel()
        {
            if (!headSwivel || Time.time < nextHeadSend) return;
            nextHeadSend = Time.time + 1f / 15f;
            ResendHead();
        }

        void ResendHead()
        {
            var pc = FindLocalPlayer();
            if (pc?.head == null || pc.playerNetworking == null) return;
            pc.playerNetworking.CmdSetHeadState(pc.head.headState);
        }

        internal Vector2 ModifyOutgoingHeadState(Vector2 real)
        {
            if (!headSwivel) return real;
            if (!loggedHeadState)
            {
                loggedHeadState = true;
                MelonLogger.Msg($"Head swivel: your real head state is {real} (if the swing looks wrong, tell me this number)");
            }
            float swing = Mathf.Sin(Time.time * swivelSpeed.Value * Mathf.PI * 2f) * swivelAmount.Value;
            return new Vector2(real.x + swing, real.y);
        }

        // ---------------------------------------------------------- kidnap
        // Pick a friend up with the game's normal carry, then teleport: carried players come along.

        void KidnapToRandomSpot()
        {
            var pc = FindLocalPlayer();
            if (pc == null) { status = "No local player found"; return; }

            // Prefer one of your saved spots; otherwise somewhere a few hundred metres away.
            var saved = new System.Collections.Generic.List<Vector3>();
            for (int i = 0; i < SlotCount; i++)
                if (TryParseVector(slots[i].Value, out var v)) saved.Add(v);
            if (saved.Count > 0)
            {
                Teleport(saved[trollRandom.Next(saved.Count)]);
                return;
            }

            var from = PlayerPosition(pc);
            for (int tries = 0; tries < 20; tries++)
            {
                float angle = RandomRange(0, Mathf.PI * 2f);
                float dist = RandomRange(200f, 600f);
                float x = from.x + Mathf.Cos(angle) * dist, z = from.z + Mathf.Sin(angle) * dist;
                if (TryGroundHeight(x, z, out float y)) { Teleport(new Vector3(x, y, z)); return; }
            }
            status = "Couldn't find ground nearby; save a spot first";
        }

        float dropAndVanishAt;

        void DropAndVanish()
        {
            var net = FindLocalPlayer()?.playerNetworking;
            if (net == null) { status = "No local player found"; return; }
            net.CmdDropHeldPlayer();
            dropAndVanishAt = Time.time + 0.6f;
            Report("Dropped them. Bye!");
        }

        void UpdateDropAndVanish()
        {
            if (dropAndVanishAt <= 0 || Time.time < dropAndVanishAt) return;
            dropAndVanishAt = 0;
            var pc = FindLocalPlayer();
            if (pc == null) return;
            var from = PlayerPosition(pc);
            for (int tries = 0; tries < 20; tries++)
            {
                float angle = RandomRange(0, Mathf.PI * 2f);
                float dist = RandomRange(40f, 90f);
                float x = from.x + Mathf.Cos(angle) * dist, z = from.z + Mathf.Sin(angle) * dist;
                if (TryGroundHeight(x, z, out float y)) { Teleport(new Vector3(x, y, z)); return; }
            }
        }

        void UpdateTroll()
        {
            UpdateNarcolepsy();
            UpdatePhantomTyping();
            UpdateHeadSwivel();
            UpdateDropAndVanish();
        }

        float DrawTrollTab(float x, float y, float w)
        {
            GUI.Label(new Rect(x, y, w, 22), "Kidnap: pick a friend up the normal way first.");
            y += 24;
            float half = (w - 8) / 2;
            if (GUI.Button(new Rect(x, y, half, 26), "Kidnap to random spot")) KidnapToRandomSpot();
            if (GUI.Button(new Rect(x + half + 8, y, half, 26), "Drop & vanish")) DropAndVanish();
            y += 34;
            if (Toggle(x, y, w, headSwivel, "Head swivel")) SetHeadSwivel(!headSwivel);
            y += 30;
            if (Toggle(x, y, w, narcolepsy, "Narcolepsy")) SetNarcolepsy(!narcolepsy);
            y += 30;
            if (GUI.Button(new Rect(x, y, w, 26), napping ? "Wake up now" : "Nap right now"))
            {
                SetNap(!napping);
                if (narcolepsy) nextNapChange = Time.time + RandomRange(napMinLength.Value, napMaxLength.Value);
            }
            y += 30;
            if (Toggle(x, y, w, phantomTyping, "Phantom typist")) SetPhantomTyping(!phantomTyping);
            return y + 34;
        }
    }
}
