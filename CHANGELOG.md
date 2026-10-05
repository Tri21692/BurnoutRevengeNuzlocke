# Changelog

## V1.1

- **Fixed:** ISOs made with the ISO patcher crashed on boot ("Jump to unmapped recLUT page"). The patcher
  no longer changes the game file's layout; the mod's code now goes into unused space inside the game's
  own data. Make your patched ISOs again with the new patcher.
- The tracker recognises ISOs made with the new patcher.
- **Race and Crash life pools.** Every car has separate lives for Race events (everything except Crash
  junctions) and for Crash junctions, each with the difficulty's full count. A car out of Race lives is
  blocked in the garage (`[RACE X]`), out of Crash lives it's blocked in crash junctions (`[CRASH X]`).
  The run is dead when every car is wrecked in either pool. Runs saved by V1.0 carry over, with each
  car's lives in both pools.
- **Crash cars have no Race pool.** The tracker notes where each car appears (garage, crash junction car
  select, or both) and only gives it the pools it can use. Crash cars only show Crash hearts, and only
  cars that can race count towards the Race pool's "run's dead" check.
- **Replayed events are named.** The tracker reads the event the game has loaded instead of guessing
  from the saved results, so replays no longer show as "Unknown / replayed event".
- **Retry on the results screens is blocked**, so a car that has just been wrecked can't be raced again.
- **Widescreen 16:9** and **60 FPS menus and crash mode** ("16:9 HUD Scale & 60 FPS Fix" by SuperType1/remco)
  added as optional groups in the `.pnach` and as options in the ISO patcher.

## V1.0

First release.

- **Lives tracker** (`nuzlocke.exe`): 3 / 2 / 1 lives per car, Gold + Perfect or lose a life, run stats,
  Run's Dead screen with Grace mode, control page and OBS stream overlay showing the difficulty and
  the opponents' AI level.
- **Wrecked cars** are renamed `[WRECKED]` and blocked in the garage and in crash junctions (including
  two-player junctions).
- **Every event type** is detected, through a finished-event signal patched into the game.
- **Pause menu**: Retry, Restart and Quit are disabled, so events can't be abandoned to save a life.
- **Harder AI**, three levels: faster cornering, opponents no longer ease off when ahead of their pace,
  higher speed limit, and a catch-up that boosts opponents who fall behind you.
- **ISO patcher** (`nuzlocke_isopatch.exe`): builds the mod into a copy of your own US ISO, one ISO per
  AI level, so no `.pnach` is needed.
