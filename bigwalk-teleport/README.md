# Big Walk Teleport

A MelonLoader mod for Big Walk that lets you teleport to coordinates, save spots, see where you are, hide your own player model, use voice of god, and throw an outfit disco.

## Install

1. Install **MelonLoader 0.7.3 or newer** into Big Walk with the [MelonLoader installer](https://github.com/LavaGang/MelonLoader.Installer/releases). Point it at the Big Walk `.exe`.
2. Start the game once and wait. The first launch takes a few minutes while MelonLoader reads the game and builds `MelonLoader/Il2CppAssemblies`. Close the game when you reach the menu.
3. Copy `release/BigWalkTeleport.dll` into the game's `Mods` folder (next to the `.exe`).
4. Start the game. The MelonLoader console should show `Big Walk Teleport v1.0.0` loading.

## Controls

| Key | What it does |
| --- | --- |
| **F8** | Open or close the teleport window (frees the mouse while open) |
| **F7** | Show or hide your X / Y / Z in the top-left corner |
| **I** | Hide or show your player model |
| **O** | Outfit disco on yourself: your colours change every 0.6s for everyone to see (host only) |
| **B** | Voice of god: everyone hears your voice at full volume, wherever they are (works as a guest) |
| **F9** | Write all of the game's control bindings (keyboard, mouse and controller) to the log. This also happens once on its own when you enter the world |
| **D-pad down** (controller) | Teleport to wherever you're looking, up to 500m away |

In the teleport window:

- Type **X, Y, Z** and press **Teleport**. Leave **Y** blank to drop onto the ground at that X/Z.
- You can also paste a whole `x, y, z` into the X box and leave the other two empty.
- **Use my position** fills the boxes with where you are now.
- **Voice of god** toggles the same as **B**.
- **Outfit disco: me / everyone** starts or stops the disco on just you or on every player in your lobby (host only).
- **Saved spots**: **Save** stores where you are now, **Go** teleports there, and **Load** copies the spot into the boxes. Spots are kept between sessions.

You can change the keys, disco speed, look-teleport range, ground offset and saved spots in `UserData/MelonPreferences.cfg` under `[BigWalkTeleport]`.

## Things to know

- **Outfit disco only works when you host.** The host's game owns everyone's outfit, and a guest's "change my look" request is ignored in the released game. The disco never writes to anyone's saved outfit, skips reward-only colours, and puts everyone's real outfit back when you stop it.
- **If teleporting doesn't work,** send `MelonLoader/Latest.log`. Every teleport logs what it found and where you end up 1, 10, 60 and 300 frames later.
- **Voice of god** switches on the game's own "2D voice" setting, which any player can use. Every player's game then plays your voice without distance fade or muffling. If the game only sends voice to players within some range, people far away still won't hear you. Press **B** again to turn it off.
- **D-pad down teleport** finds the button through the game's own input system, so it should work with Xbox, PlayStation and most other pads. The first press logs which button it found. If the game also uses d-pad down for something, both will happen; set `DpadDownTeleport = false` to turn it off. To look-teleport from the keyboard, set `LookTeleportKey` (for example `T`).
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
