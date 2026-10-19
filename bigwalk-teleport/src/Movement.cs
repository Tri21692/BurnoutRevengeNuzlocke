using UnityEngine;

namespace BigWalkTeleport
{
    // Movement pack: flight, moon mode, sonic speed and rocket launch.
    // Speeds and jump strength come from the player's own PlayerTunings, so they're scaled there.
    // Flight, low gravity and the rocket run right after PlayerMover.FixedUpdate (see Patches.cs)
    // so the game's own movement step can't undo them.
    public partial class Mod
    {
        Setting<KeyCode> flyKey, moonKey, sonicKey, rocketKey;
        Setting<float> flySpeed, flySprintSpeed, moonJumpMultiplier, moonGravity, sonicMultiplier, rocketSpeed, slowFallSpeed;

        bool flying, moon, sonic;
        // Rocket: 0 = off, 1 = going up, 2 = drifting down slowly.
        int rocketPhase;
        float rocketPhaseStarted;
        bool weChangedGravity;

        // The player's untouched tunings, captured the first time we change them.
        System.IntPtr tuningsOwner;
        float baseForward, baseSprint, baseCrouch, baseCrouchSprint, baseSwim, baseSwimSprint, baseGhostSprint, baseJump, baseMaxUp;

        void CreateMovementPrefs(SettingsCategory cat)
        {
            flyKey = cat.CreateEntry("FlyKey", KeyCode.F1, "Toggle flight (move to steer, jump = up, crouch = down, sprint = fast)");
            moonKey = cat.CreateEntry("MoonKey", KeyCode.F2, "Toggle moon mode (big jumps, low gravity)");
            sonicKey = cat.CreateEntry("SonicKey", KeyCode.F3, "Toggle sonic speed");
            rocketKey = cat.CreateEntry("RocketKey", KeyCode.F4, "Rocket launch straight up, then drift down slowly");
            flySpeed = cat.CreateEntry("FlySpeed", 12f, "Flight speed in metres per second");
            flySprintSpeed = cat.CreateEntry("FlySprintSpeed", 40f, "Flight speed while holding sprint");
            moonJumpMultiplier = cat.CreateEntry("MoonJumpMultiplier", 3f, "How much stronger jumps are in moon mode");
            moonGravity = cat.CreateEntry("MoonGravity", 0.35f, "Gravity in moon mode, as a fraction of normal (0.35 = 35%)");
            sonicMultiplier = cat.CreateEntry("SonicMultiplier", 4f, "How much faster you walk and sprint in sonic mode");
            rocketSpeed = cat.CreateEntry("RocketSpeed", 70f, "Upward speed of the rocket launch in metres per second");
            slowFallSpeed = cat.CreateEntry("SlowFallSpeed", 6f, "Fastest you fall after a rocket launch or when flight is turned off mid-air");
        }

        void UpdateMovementKeys(bool typing)
        {
            if (typing) return;
            if (KeyDown(flyKey)) SetFlying(!flying);
            if (KeyDown(moonKey)) SetMoon(!moon);
            if (KeyDown(sonicKey)) SetSonic(!sonic);
            if (KeyDown(rocketKey)) LaunchRocket();
        }

        static bool KeyDown(Setting<KeyCode> key) => key.Value != KeyCode.None && Input.GetKeyDown(key.Value);

        void SetFlying(bool on)
        {
            flying = on;
            if (!on)
            {
                // Don't drop like a stone from wherever you stopped flying.
                rocketPhase = 2;
                rocketPhaseStarted = Time.time;
            }
            Report(on ? "Flight ON: move to steer, jump = up, crouch = down, sprint = fast" : "Flight off");
        }

        void SetMoon(bool on)
        {
            moon = on;
            ApplyTunings();
            Report(on ? "Moon mode ON" : "Moon mode off");
        }

        void SetSonic(bool on)
        {
            sonic = on;
            ApplyTunings();
            Report(on ? $"Sonic speed ON ({sonicMultiplier.Value:0.#}x)" : "Sonic speed off");
        }

        void LaunchRocket()
        {
            if (FindLocalPlayer() == null) { status = "No local player found"; return; }
            flying = false;
            rocketPhase = 1;
            rocketPhaseStarted = Time.time;
            ApplyTunings();
            Report("3... 2... 1... LIFTOFF");
        }

        void ApplyTunings()
        {
            var pc = FindLocalPlayer();
            var t = pc?.tunings;
            if (t == null) return;

            if (tuningsOwner != pc.Pointer)
            {
                tuningsOwner = pc.Pointer;
                baseForward = t.forwardSpeed; baseSprint = t.forwardSprintSpeed;
                baseCrouch = t.crouchForwardSpeed; baseCrouchSprint = t.crouchForwardSprintSpeed;
                baseSwim = t.swimForwardSpeed; baseSwimSprint = t.swimForwardSprintSpeed;
                baseGhostSprint = t.forwardSprintGhostSpeed;
                baseJump = t.jumpForce; baseMaxUp = t.maxUpwardsVelocity;
            }

            float s = sonic ? sonicMultiplier.Value : 1f;
            t.forwardSpeed = baseForward * s;
            t.forwardSprintSpeed = baseSprint * s;
            t.crouchForwardSpeed = baseCrouch * s;
            t.crouchForwardSprintSpeed = baseCrouchSprint * s;
            t.swimForwardSpeed = baseSwim * s;
            t.swimForwardSprintSpeed = baseSwimSprint * s;
            t.forwardSprintGhostSpeed = baseGhostSprint * s;

            float j = moon ? moonJumpMultiplier.Value : 1f;
            t.jumpForce = baseJump * j;
            // The game caps upward speed; lift the cap while the rocket is going up.
            t.maxUpwardsVelocity = rocketPhase == 1 ? Mathf.Max(baseMaxUp * j, rocketSpeed.Value * 2f) : baseMaxUp * j;
        }

        // Called from the PlayerMover.FixedUpdate postfix.
        internal void AfterMoverFixedUpdate(PlayerMover mover)
        {
            var pc = FindLocalPlayer();
            if (pc == null || pc.mover == null || pc.mover.Pointer != mover.Pointer) return;
            var rb = pc.rb;
            if (rb == null) return;

            if (flying)
            {
                if (!weChangedGravity) { rb.useGravity = false; weChangedGravity = true; }
                rb.linearVelocity = FlightVelocity(pc);
                return;
            }
            if (weChangedGravity) { rb.useGravity = true; weChangedGravity = false; }

            bool airborne = pc.ground != null && pc.ground.groundCollider == null;
            var v = rb.linearVelocity;

            if (rocketPhase == 1)
            {
                rb.linearVelocity = new Vector3(v.x, rocketSpeed.Value, v.z);
                if (Time.time - rocketPhaseStarted > 2.5f)
                {
                    rocketPhase = 2;
                    rocketPhaseStarted = Time.time;
                    ApplyTunings();
                }
                return;
            }

            if (rocketPhase == 2)
            {
                if (v.y < -slowFallSpeed.Value) rb.linearVelocity = new Vector3(v.x, -slowFallSpeed.Value, v.z);
                bool landed = !airborne && Time.time - rocketPhaseStarted > 0.5f;
                if (landed || Time.time - rocketPhaseStarted > 90f) rocketPhase = 0;
                return;
            }

            if (moon && airborne)
                rb.AddForce(-Physics.gravity * (1f - Mathf.Clamp01(moonGravity.Value)), ForceMode.Acceleration);
        }

        Vector3 FlightVelocity(PlayerCharacter pc)
        {
            var input = pc.inputPlayer;
            var cam = PlayerCamera();
            if (input == null || cam == null) return Vector3.zero;

            // Same Rewired actions the game uses, so keyboard and controller both work.
            float forward = input.GetAxis(RewiredActionMoveY);
            float strafe = input.GetAxis(RewiredActionMoveX);
            var dir = cam.transform.forward * forward + cam.transform.right * strafe;
            if (input.GetButton(RewiredActionJump)) dir += Vector3.up;
            if (input.GetButton(RewiredActionCrouch)) dir -= Vector3.up;
            dir = Vector3.ClampMagnitude(dir, 1f);

            float speed = input.GetButton(RewiredActionSprint) ? flySprintSpeed.Value : flySpeed.Value;
            return dir * speed;
        }

        // Action ids from the game's RewiredConsts.Action.
        const int RewiredActionMoveY = 8, RewiredActionMoveX = 9, RewiredActionSprint = 15, RewiredActionJump = 16, RewiredActionCrouch = 20;

        float DrawMovementTab(float x, float y, float w)
        {
            if (Toggle(x, y, w, flying, $"Flight ({flyKey.Value})")) SetFlying(!flying);
            y += 30;
            if (Toggle(x, y, w, moon, $"Moon mode ({moonKey.Value})")) SetMoon(!moon);
            y += 30;
            if (Toggle(x, y, w, sonic, $"Sonic speed ({sonicKey.Value})")) SetSonic(!sonic);
            y += 30;
            if (GUI.Button(new Rect(x, y, w, 26), $"Rocket launch ({rocketKey.Value})")) LaunchRocket();
            y += 30;
            GUI.Label(new Rect(x, y, w, 44), "Flight: move to steer, jump = up, crouch = down, sprint = fast. Turning flight off mid-air drifts you down slowly.");
            return y + 48;
        }
    }
}
