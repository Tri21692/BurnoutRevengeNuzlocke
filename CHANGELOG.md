# Changelog

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
