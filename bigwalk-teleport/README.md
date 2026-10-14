# Big Walk Teleport

A MelonLoader mod for Big Walk that lets you teleport to coordinates, save spots, see where you are, and hide your own player model.

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

In the teleport window:

- Type **X, Y, Z** and press **Teleport**. Leave **Y** blank to drop onto the ground at that X/Z.
- You can also paste a whole `x, y, z` into the X box and leave the other two empty.
- **Use my position** fills the boxes with where you are now.
- **Saved spots**: **Save** stores where you are now, **Go** teleports there, and **Load** copies the spot into the boxes. Spots are kept between sessions.

You can change the keys, ground offset and saved spots in `UserData/MelonPreferences.cfg` under `[BigWalkTeleport]`.

## Things to know

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
