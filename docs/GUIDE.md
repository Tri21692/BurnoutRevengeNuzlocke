# Start-up guide and FAQ

Everything you need to set up and play a Burnout Revenge Nuzlocke run. For what the mod is, see the [README](../README.md).

- [What you need](#what-you-need)
- [Before you start: good practices](#before-you-start-good-practices)
- [Setup, step by step](#setup-step-by-step)
- [Playing a run](#playing-a-run)
- [FAQ](#faq)
- [Troubleshooting](#troubleshooting)
- [Known limitations](#known-limitations)

## What you need

The mod works with one version of the game only: the US release of Burnout Revenge for PS2, serial **SLUS-21242**, running in PCSX2 on Windows.

| Item | What it's for | Where to get it |
| --- | --- | --- |
| Burnout Revenge, SLUS-21242 (v1.00) | The game. Other regions use different memory addresses and won't work. | Your own copy. Check the serial in PCSX2's game list (right-click the game, then Properties). |
| PCSX2 2.x | The emulator. | [pcsx2.net](https://pcsx2.net) |
| `bin/nuzlocke.exe` | The tracker: judges results, counts lives, shows your stats. | This repository |
| `patches/SLUS-21242_D224D348.pnach` **or** `bin/nuzlocke_isopatch.exe` | The game changes, as a PCSX2 patch file or built into a copy of your ISO. | This repository |
| OBS (optional) | Shows the run on stream. | [obsproject.com](https://obsproject.com) |

You don't need Python, Cheat Engine or anything else.

## Before you start: good practices

The most important rule: back up your memory card and give each run its own card, so a run can never damage your main save.

1. **Back up your memory card.** Copy `Mcd001.ps2` from PCSX2's `memcards` folder (usually `Documents\PCSX2\memcards`) somewhere safe.
2. **Use a separate memory card for each run.** In PCSX2's Memory Cards settings, create a new card, put it in slot 1 and start a fresh profile. Starting a new run in the tracker doesn't reset the game's own progress, so a fresh card keeps both in step.
3. **Don't use save states during a run.** Loading a state undoes an event in the game but not in the tracker, so results can be counted twice or not at all. It also defeats the point of a Nuzlocke.
4. **Keep the game's autosave on.** The tracker reads your saved results, and autosave stops a result from being undone.
5. **Start the tracker before your first event** and keep it open for the whole session. If you close it, do so in a menu, not during an event.
6. **Only play World Tour during a run.** The tracker judges every finished event it sees, so other modes may be counted too.
7. **Leave other cheats and patches off.** Widescreen and 60 FPS are included in the mod, so you don't need a separate widescreen patch. Anything that writes to `000FE000`–`000FFFFF` or `00479800`–`00479FFF` will clash with the mod.
8. **Don't edit `nuzlocke_state.json` while the tracker is running.** Copy it if you want to keep a finished run's history.

## Setup, step by step

Setup takes about ten minutes and only needs doing once.

1. **Enable PINE.** In PCSX2, go to Settings → Advanced and turn on PINE. Leave the slot at 28011. The tracker needs it to block wrecked cars.
2. **Add the game changes.** Pick one of these:
   - **Patch file (recommended).** Copy `SLUS-21242_D224D348.pnach` as it is into PCSX2's `patches` folder (usually `Documents\PCSX2\patches`). In the game's Properties, open the **Patches** tab and tick:
     - **Nuzlocke\Block dead cars in garage** (required)
     - **Nuzlocke\Block pause-menu Retry and Quit** (recommended)
     - **one** of **Nuzlocke\Harder AI\Easy / Medium / Hard** (optional; never more than one)
     - **Nuzlocke\Widescreen 16:9** and **Nuzlocke\60 FPS menus and crash mode** (optional, by SuperType1/remco)

     It also works from the `cheats` folder and the Cheats tab. Check Windows hasn't added a hidden `.txt` to the file name. Restart the game after changing anything.
   - **Patched ISO.** Drag your clean Burnout Revenge ISO onto `nuzlocke_isopatch.exe` choose Easy, Medium or Hard, and answer whether you want widescreen and 60 FPS. It writes a new ISO next to the original, for example `Burnout Revenge (Nuzlocke Hard, 16-9).iso`, with everything above built in. Add that ISO to PCSX2 and play it, **without** the `.pnach` enabled. Your original ISO isn't changed.
3. **Set up the tracker.** Put `nuzlocke.exe` in a folder of its own, since it saves your run next to itself. Double-click it.
   - If Windows SmartScreen warns you, click More info, then Run anyway. The program isn't signed.
   - A small console window opens. Keep it open while you play; closing it stops the tracker.
4. **Open the control page.** It opens in your browser automatically, at `http://localhost:8765`.
5. **Boot the game.** The control page should say "Connected to PCSX2" with no warnings, and show the opponents' AI level once a run has started.
6. **Add the stream overlay (optional).** In OBS, add a Browser Source with the URL `http://localhost:8765/overlay`, sized about 800 × 400, and place it in your scene. The control page has a Copy button for the address.

## Playing a run

Every event must reach the difficulty's result (from Silver + Great on Easy up to a first-time Gold + Perfect on Hard); anything less costs the car you drove a life.

**Starting.** On the control page, pick a difficulty. It's locked for the whole run.

| Difficulty | Race lives per car | Crash lives per car | Each event needs at least |
| --- | --- | --- | --- |
| Easy | 3 | 3 | Silver + Great |
| Medium | 2 | 2 | Silver + Awesome |
| Hard | 1 | 1 | Gold + Perfect (first time) |

Every car has two separate pools. **Crash junctions** use the Crash pool; **every other event** (races, Grand Prix, Eliminator, Burning Lap, Road Rage, Traffic Attack, Preview) uses the Race pool. Each car gets its pools in full the first time it appears. Cars only get the pools they can use: the tracker notes whether a car shows up in the garage (it can race) and in a crash junction's car select (it can crash). Crash cars, which only appear in crash junctions, have no Race pool, and the overlay only shows the hearts a car actually has. The opponents' AI level is separate: it comes from the patch or ISO you chose, and the control page and overlay show it.

**How each event is judged.** Every event type counts, including Crash junctions and Preview events.

| Result | What happens |
| --- | --- |
| The difficulty's result or better (both the medal and the rating) | Win. Your win streak goes up. |
| Anything less, for example Bronze, or Silver + Good on Easy | The car you drove loses a life. |
| On Hard: replaying an event you've already Gold + Perfected | The car loses a life. The game never awards Perfect twice, so a replay can't be won on Hard. On Easy and Medium a replay can win. |

The medal and the rating are checked separately: on Medium, Gold + Awesome and Silver + Awesome both pass, but Silver + Great doesn't. Taking Gold raises your rating one step (that's how a Perfect happens: Gold with Awesome-level driving), so Gold + Good shows as Gold + Great and passes Easy.

**No bailing out.** With the pause-menu block on, Retry, Restart and Quit don't respond, and Retry on the results screens acts like Continue. Every event you start has to be finished, and the next attempt starts from the garage.

**Wrecked cars.** A car out of **Race** lives is renamed `[RACE X]` and can't be selected in the garage, but can still be picked in crash junctions. A car out of **Crash** lives is renamed `[CRASH X]` and can't be picked in crash junctions, but can still race. A car out of both is `[WRECKED]`. (Short names get `[R X]`, `[C X]`, `[WRECK]` or `[X]`.) This is reapplied every time the game starts.

**Run's dead.** When every car that can race is wrecked in the Race pool, **or** every car that can crash is wrecked in the Crash pool, the control page and stream overlay show your final stats. You then choose:

- **Shut down game:** closes PCSX2.
- **Grace continue:** keep playing with all cars usable again, but nothing counts any more.
- **Start a new run:** begin again with a new difficulty. Use a fresh memory card or profile too.

The stats are events won out of events played, cars wrecked in each pool, run time (time with the game running), your best win streak, and the difficulty and AI level.

## FAQ

**Can I close the tracker between sessions?**
Yes. Your run is saved in `nuzlocke_state.json` next to `nuzlocke.exe`. Start it again before your next event and it carries on.

**Can I quit or retry an event to avoid losing a life?**
Not with the pause-menu block on: those options do nothing. Without it, the tracker only judges events that finish with a result.

**Can I change difficulty mid-run?**
Lives, no: they're locked for the run, so start a new run to change them. The AI level can be changed between sessions, and the overlay shows whichever is active.

**Which AI level should I pick?**
Easy is already noticeably tougher than the stock game. Hard opponents corner close to the limit and come back very quickly when they fall behind you. Try a few races on each before starting a real run.

**Is my save file safe?**
The mod only changes the game's memory while it runs. Wrecked names and the blocked-car list are never written to your memory card, and your progress saves normally. Back up your card anyway.

**Why does the garage say [WRECKED] but the tracker shows the real name?**
The tracker keeps each car's original name, so stats and history stay readable.

**Are replayed events named?**
Yes. The tracker reads the event the game has loaded, so replays show their real name and count towards the right pool. If a result ever says "Unknown event", the game didn't report one; the life still comes off the Race pool.

**Can I share a patched ISO?**
No: it contains the game. Share `nuzlocke_isopatch.exe` instead; it only contains the mod's changes and works on anyone's own clean copy.

**Why does my antivirus flag nuzlocke.exe?**
The tracker reads and writes PCSX2's memory, which some antivirus programs treat as suspicious. The full source is in [`tracker/`](../tracker) if you want to check it or build it yourself.

## Troubleshooting

Most problems show up as a warning on the control page; the fix for each is below.

| What you see | What to do |
| --- | --- |
| "Waiting for PCSX2" | Start PCSX2 and boot the game. If it still says so, close and reopen `nuzlocke.exe`. |
| "PINE isn't connected" | Turn on PINE in Settings → Advanced, then restart PCSX2. |
| "The Nuzlocke patch isn't active" | Check the `.pnach` is in the `patches` (or `cheats`) folder with its original name and that **Block dead cars in garage** is ticked, or that you're playing the patched ISO. Restart the game. |
| "This isn't Burnout Revenge (SLUS-21242)" | You're running another version. Only SLUS-21242 is supported. |
| "Port 8765 is already in use" | The tracker is probably already running: check the taskbar. Otherwise another program uses that port. |
| The AI level shows "normal AI" | No Harder AI level is active. Tick one in the Patches tab and restart, or play a patched ISO. |
| The ISO patcher says the file isn't the US release | It only patches a clean, unmodified SLUS-21242 ISO. Use your original dump. |
| A patched ISO won't boot | Try with PCSX2's Fast Boot on and off, and report which fails. Meanwhile, use the `.pnach` with your original ISO. |
| A wrecked car can still be selected | Check the control page for patch or PINE warnings, and that the car shows `[WRECKED]`. |
| The OBS overlay is empty | The tracker must be running with a run started. Check the URL ends in `/overlay`, then refresh the Browser Source. |
| An event wasn't counted | The tracker must be running during the event. If it was, note the event type and report it. |
| The garage freezes | Untick the patch, restart the game, and report what you did just before. |

## Known limitations

- Only the US version (SLUS-21242) works, and only on Windows.
- There's no warning yet when you highlight an event you've already perfected.
- The Harder AI levels are new: their numbers may be tuned after more play-testing.
- Patched ISOs have been checked byte for byte against the `.pnach`, but not yet booted on every setup.
