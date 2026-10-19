# Big Walk Teleport

A MelonLoader mod for Big Walk: teleporting and saved spots, plus three solo packs (troll, movement and helper), voice of god, and a host-only outfit disco. Everything in the troll, movement and helper packs works as a guest.

## Install

There are two builds of the same mod. Use one or the other, not both.

### With Gale (BepInEx)

1. In Gale, pick **Big Walk** and make or open a profile. If the profile doesn't have BepInEx yet, install the **BepInExPack** from Gale's mod browser.
2. Launch the game once from Gale and wait until you reach the menu. The first launch takes a few minutes while BepInEx reads the game.
3. Close the game. In Gale's menu bar, choose **Import → ...local mod** and pick `release/BigWalkTeleport-Gale.zip`.
4. Launch from Gale. The BepInEx console should show `Big Walk Teleport 1.1.0 loaded`.

Settings live in `BepInEx/config/tri21692.BigWalkTeleport.cfg`, which Gale's config editor can open.

### With MelonLoader

1. Install **MelonLoader 0.7.3 or newer** into Big Walk with the [MelonLoader installer](https://github.com/LavaGang/MelonLoader.Installer/releases). Point it at the Big Walk `.exe`.
2. Start the game once and wait. The first launch takes a few minutes while MelonLoader reads the game and builds `MelonLoader/Il2CppAssemblies`. Close the game when you reach the menu.
3. Copy `release/BigWalkTeleport.dll` into the game's `Mods` folder (next to the `.exe`).
4. Start the game. The MelonLoader console should show `Big Walk Teleport v1.1.0` loading.

Settings live in `UserData/MelonPreferences.cfg` under `[BigWalkTeleport]`.

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

You can change every key (the window key is `WindowKey`, the coordinates key is `CoordsKey`), plus flight, moon, sonic and rocket strengths, nap timing, head-swivel size, disco speed, look-teleport range, ground offset and saved spots, in the settings file for your loader (see **Install**).

## Things to know

- **Outfit disco only works when you host.** The host's game owns everyone's outfit, and a guest's "change my look" request is ignored in the released game. The disco never writes to anyone's saved outfit, skips reward-only colours, and puts everyone's real outfit back when you stop it.
- **If teleporting doesn't work,** send the log (`MelonLoader/Latest.log`, or `BepInEx/LogOutput.log` with Gale). Every teleport logs what it found and where you end up 1, 10, 60 and 300 frames later.
- **Voice of god** switches on the game's own "2D voice" setting, which any player can use. Every player's game then plays your voice without distance fade or muffling. If the game only sends voice to players within some range, people far away still won't hear you. Press **B** again to turn it off.
- **Movement mods are tuned blind.** The numbers are guesses until someone tries them, so adjust `FlySpeed`, `MoonGravity`, `SonicMultiplier` and the rest in the config if they feel wrong. Other players see the results because your own game controls where you are.
- **Head swivel** logs your real head direction the first time you use it. If the swing looks too small, too big or sideways, send me that log line and I'll calibrate it.
- **D-pad and `,` / `.` keys:** the released game still has its developer control map switched on. In it, d-pad down and `,` fire an unused "TeleportDown" action, and d-pad up and `.` fire "TeleportUp". The mod listens for those, so it works with any controller and nothing in the game clashes. Turn them off with `DpadDownTeleport` / `DpadUpGoBack`. D-pad left and right aren't used by the game during play.
- **Hiding your model only changes your own screen.** Other players still see you, because each player's game draws everyone itself. Props you are holding are hidden too.
- **Online:** the game syncs your position from your own client, so other players should see you move to the new spot. It hasn't been tested with friends yet.
- While you type in the window, the game still reads the keyboard, so typing letters like W/A/S/D may move you. Numbers are safe.
- Teleporting while sitting, sleeping or holding onto something may look odd until you move again.

## Building from source

One codebase builds both versions. `Platform.cs` and `Entry.cs` hold the parts that differ.

MelonLoader (needs `MelonLoader\Il2CppAssemblies` from the first launch):

```
cd src
dotnet build -c Release -p:GameDir="C:\Program Files (x86)\Steam\steamapps\common\Big Walk"
```

BepInEx / Gale (needs `BepInEx\interop` and `BepInEx\core` from the first launch, so point `GameDir` at the Gale profile folder):

```
cd src
dotnet build -c Release -p:Loader=BepInEx -p:GameDir="<Gale profile folder>"
../thunderstore/package.sh bin/BepInEx/Release/net6.0/BigWalkTeleport.dll
```

You can also pass `-p:InteropDir=<folder>` (and `-p:BepInExCoreDir=<folder>` for BepInEx) to point straight at those folders.
