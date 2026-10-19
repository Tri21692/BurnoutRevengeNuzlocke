# Big Walk Teleport

A MelonLoader mod for Big Walk: teleporting and saved spots, plus three solo packs (troll, movement and helper), voice of god, and a host-only outfit disco. Everything in the troll, movement and helper packs works as a guest.

## Install

1. Install **MelonLoader 0.7.3 or newer** into Big Walk with the [MelonLoader installer](https://github.com/LavaGang/MelonLoader.Installer/releases). Point it at the Big Walk `.exe`.
2. Start the game once and wait. The first launch takes a few minutes while MelonLoader reads the game and builds `MelonLoader/Il2CppAssemblies`. Close the game when you reach the menu.
3. Copy `release/BigWalkTeleport.dll` into the game's `Mods` folder (next to the `.exe`).
4. Start the game. The MelonLoader console should show `Big Walk Teleport v1.0.0` loading.

## Controls

| Key | What it does |
| --- | --- |
| **F7** | Open or close the mod window (frees the mouse while open) |
| **F5** | Show or hide your X / Y / Z in the top-left corner |
| **I** | Hide or show your player model |
| **O** | Outfit disco on yourself: your colours change every 0.6s for everyone to see (host only) |
| **B** | Voice of god: everyone hears your voice at full volume, wherever they are (works as a guest) |
| **F9** | Write all of the game's control bindings (keyboard, mouse and controller) to the log. This also happens once on its own when you enter the world |
| **D-pad down** (controller) or **,** | Teleport to wherever you're looking, up to 500m away |
| **D-pad up** (controller) or **.** | Go back to where you were before your last teleport. Press again to jump back and forth |
| **F1** | Flight |
| **F2** | Moon mode |
| **F3** | Sonic speed |
| **F4** | Rocket launch |
| **F6** | Friend markers |
| **F10** | Auto-walk |

The F7 window has five tabs.

**Teleport**
- Type **X, Y, Z** and press **Teleport**. Leave **Y** blank to drop onto the ground at that X/Z.
- You can also paste a whole `x, y, z` into the X box and leave the other two empty.
- **Use my position** fills the boxes with where you are now.
- **Go back** returns you to where you were before your last teleport.
- **Saved spots**: **Save** stores where you are now, **Go** teleports there, and **Load** copies the spot into the boxes. Spots are kept between sessions.

**Fun**
- **Hide my model** (only on your screen), **Voice of god**, and **Outfit disco** for you or everyone (host only).

**Troll** (works as a guest)
- **Kidnap to random spot**: pick a friend up with the game's normal carry, then press this. You both land at one of your saved spots, or somewhere 200 to 600m away if you have none.
- **Drop & vanish**: put down whoever you're carrying and teleport 40 to 90m away.
- **Head swivel**: other players see your head swing side to side. Your own camera stays normal.
- **Narcolepsy**: you fall asleep at random every 40 to 120 seconds for 5 to 12 seconds, using the game's own sleep. **Nap right now** / **Wake up now** do it on demand.
- **Phantom typist**: an endless "typing..." bubble with keyboard sounds in bursts, and nothing ever gets sent.

**Move** (works as a guest)
- **Flight**: steer with movement; jump goes up, crouch goes down, and sprint goes fast. If you turn flight off mid-air, you drift down slowly.
- **Moon mode**: 3x jumps and 35% gravity.
- **Sonic speed**: 4x walking, sprinting, crouching and swimming speed.
- **Rocket launch**: blast about 175m straight up, then drift down slowly.

**Helper**
- **Friend markers**: everyone's name and distance on screen, through walls. Friends behind you show along the bottom edge.
- **Auto-walk**: walks forward for you. Pull back on movement to stop.
- **Teleport to a friend**: a **Go** button for each player lands you just behind them.

You can change every key (the window key is `WindowKey`, the coordinates key is `CoordsKey`), plus flight, moon, sonic and rocket strengths, nap timing, head-swivel size, disco speed, look-teleport range, ground offset and saved spots, in `UserData/MelonPreferences.cfg` under `[BigWalkTeleport]`.

## Things to know

- **Outfit disco only works when you host.** The host's game owns everyone's outfit, and a guest's "change my look" request is ignored in the released game. The disco never writes to anyone's saved outfit, skips reward-only colours, and puts everyone's real outfit back when you stop it.
- **If teleporting doesn't work,** send `MelonLoader/Latest.log`. Every teleport logs what it found and where you end up 1, 10, 60 and 300 frames later.
- **Voice of god** switches on the game's own "2D voice" setting, which any player can use. Every player's game then plays your voice without distance fade or muffling. If the game only sends voice to players within some range, people far away still won't hear you. Press **B** again to turn it off.
- **Movement mods are tuned blind.** The numbers are guesses until someone tries them, so adjust `FlySpeed`, `MoonGravity`, `SonicMultiplier` and the rest in the config if they feel wrong. Other players see the results because your own game controls where you are.
- **Head swivel** logs your real head direction the first time you use it. If the swing looks too small, too big or sideways, send me that log line and I'll calibrate it.
- **D-pad and `,` / `.` keys:** the released game still has its developer control map switched on. In it, d-pad down and `,` fire an unused "TeleportDown" action, and d-pad up and `.` fire "TeleportUp". The mod listens for those, so it works with any controller and nothing in the game clashes. Turn them off with `DpadDownTeleport` / `DpadUpGoBack`. D-pad left and right aren't used by the game during play.
- **Hiding your model only changes your own screen.** Other players still see you, because each player's game draws everyone itself. Props you are holding are hidden too.
- **Online:** the game syncs your position from your own client, so other players should see you move to the new spot. It hasn't been tested with friends yet.
- While you type in the window, the game still reads the keyboard, so typing letters like W/A/S/D may move you. Numbers are safe.
- Teleporting while sitting, sleeping or holding onto something may look odd until you move again.

## Building from source

```
cd src
dotnet build -c Release -p:GameDir="C:\Program Files (x86)\Steam\steamapps\common\Big Walk"
```

`GameDir` must already contain `MelonLoader\Il2CppAssemblies` (from step 2 above). You can also pass `-p:InteropDir=<folder>` to point straight at a folder of interop assemblies.
