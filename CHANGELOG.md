# Changelog

## V2.0

**Upgrading:** replace all three files (`nuzlocke.exe`, the `.pnach` and `nuzlocke_isopatch.exe`), and remake
any patched ISOs to use the new modes. Your current run carries over.

- **Limited Selection mode (optional).** Before every event you get 2 random cars of your own for Race events
  and 2 for Crash junctions; every other healthy car is `[BENCHED]` in the garage and in crash junctions until
  the next event. No car is picked twice in a row (unless nothing else is left), wrecked cars are never
  picked, and the picks don't reroll by leaving the garage or restarting the tracker. A new `.pnach` option
  (`Mode\Limited Selection`) and ISO patcher question.
- **Revive tokens mode (optional).** Win streaks earn tokens: every 3 wins in a row on Easy (hold up to 3),
  5 on Medium (hold 2), 8 on Hard (hold 1). Spend one on the control page to give a wrecked car one life
  back; a held token also saves a run that would otherwise end. A new `.pnach` option (`Mode\Revive tokens`)
  and ISO patcher question.
- **Event Roulette mode (optional).** After every event the tracker picks your next one at random from
  every World Tour event, ones you haven't won yet first, and every other event is locked on the map. One
  reroll to start, one more for your first win in each rank (hold up to 3). Every rank is open, and with the extra `Mode\Event Roulette - Unlock all cars` option every car too. New `.pnach`
  options and ISO patcher questions.
- **Achievements.** Nine per run, from Hot Streak (5 wins in a row) to Certified Insane (10 wins against
  Insane AI), shown on the control page and announced on the overlay after the event's result. They're
  locked for any run that uses Limited Selection, Revive tokens, Event Roulette or Unlock all cars.
- **End-of-run summary.** How the run ended, every car's events, wins and medals, and your achievements, on
  the control page and the overlay's end card, and saved as a text file next to `nuzlocke.exe`.
- **Fixed: crash junctions outside Motor City counted as Race events.** Only the 9 Motor City junctions
  were recognized; the other 41 took a life from the garage car's Race pool instead of the junction car's
  Crash pool. All 50 are recognized now, and each one's location comes from the game's own event names.
- The overlay shows the Limited Selection picks and your revive tokens; messages now queue, so an
  achievement never covers an event's result.

## V1.1.2

Medals + Aggressive AI. **Upgrading:** replace all three files (`nuzlocke.exe`, the `.pnach` and
`nuzlocke_isopatch.exe`), and remake any patched ISOs: the Harder AI levels changed. Your current run
carries over.

- **Medal requirements per difficulty.** Each event needs at least Silver + Great on Easy, Silver +
  Awesome on Medium, and Gold + Perfect (first time) on Hard, as before. The medal and the rating are
  checked separately, and on Easy and Medium a replay can win. The control page shows the requirement.
- **Harder AI leaders ease off.** An opponent far enough ahead of you is held to a share of your speed
  (Easy: from 40 m ahead, 90%; Medium: 70 m, 94%; Hard: 100 m, 97%; never below 90 / 100 / 112 mph),
  so a leader can be caught. Remake patched ISOs to get it; the tracker recognizes old and new ones.
- **Run's dead is final.** Only the cars you had before an event count towards whether it ended the run,
  so a car unlocked by that same event can't save it. While the run is dead, every car is blocked in the
  garage and in crash junctions (the new car too) until you choose Grace, a new run or shut down on the
  control page.
- **Harder AI goes for takedowns.** Each level now also changes the game's own attack settings:
  opponents attack more often, from further away and sooner after the start, block you for longer and
  slam harder (+15 / 30 / 50%). On Hard every opponent attacks. To keep that fair, their speed is a
  little lower: cornering 86.25 / 91.25 / 96.25% (was 87.5 / 92.5 / 97.5%), pace 0.5 s per section
  slower, speed limit 217 / 239 / 251 mph (was 224 / 246 / 257) and catch-up 10% weaker.
- **Insane AI level.** A fourth Harder AI level with Hard's full speed from before it was toned down
  (cornering at 97.5%, 257 mph, catch-up up to +89 mph) and far more aggression: every opponent attacks
  every 0.1–0.5 s, from up to 150 m away and half a second after the start, blocks you for 6–15 s and
  slams twice as hard. Not meant to be fair: the ISO patcher shows a warning and asks you to confirm.
- **Loaned cars.** A car lent to you by a Burning Lap or Preview event (the garage then holds only that
  car) no longer joins the run straight away. A failed event still costs the loaned car its life, but it
  only counts towards your cars, and your wrecked total, once you unlock it, so it can't leave you stuck
  with a run that should be dead. A loaned car is never blocked, so the event can always be retried; once
  it's wrecked, each failed retry costs a Race life from the last car of your own you drove (or the one
  with the most lives left). The control page and overlay mark it "(loaned)".
- **Medal strip.** The control page and the overlay show the current car's events in order as medals
  (gold, silver, bronze or none; red ribbon and slash for a loss, "C" for a Crash junction). Click a
  medal on the control page to see the event, its result, location and time. It replaces the recent
  results list.
- **Locations.** Results show where the event was, e.g. "Crash - Dock Fight · Motor City". Race events
  are already named after their location; for Crash junctions the tracker reads the track the game loaded.

## V1.1.1

Emergency patch. **Upgrading:** replace the `.pnach` (and `nuzlocke_isopatch.exe`, then remake any patched
ISOs). `nuzlocke.exe` only changed its version number. Your current run carries over.

- **Fixed:** pressing Retry on a results screen, then Continue, froze the game (seen after a failed Crash
  junction). V1.1 made Retry do nothing, which left the screen waiting for an action that never came.
  Retry on the results screens now does the same as Continue, so it still can't restart the event with a
  just-wrecked car, but the screen always moves on.

## V1.1

Emergency patch + QOL. **Upgrading from V1.0:** replace all three files (`nuzlocke.exe`, the `.pnach`
and `nuzlocke_isopatch.exe`), and remake any patched ISOs. Your current run carries over.

- **Fixed:** ISOs made with the ISO patcher crashed on boot ("Jump to unmapped recLUT page"). The patcher
  no longer changes the game file's layout; the mod's code now goes into unused space inside the game's
  own data. Make your patched ISOs again with the new patcher.
- The tracker recognizes ISOs made with the new patcher.
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
