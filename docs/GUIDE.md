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
     - **one** of **Nuzlocke\Harder AI\Easy / Medium / Hard / Insane** (optional; faster and more aggressive opponents; never more than one. Insane is not meant to be fair)
     - **Nuzlocke\Mode\Limited Selection**, **Nuzlocke\Mode\Revive tokens** and **Nuzlocke\Mode\Event Roulette** (optional, see [Modes](#modes))
     - **Nuzlocke\Widescreen 16:9** and **Nuzlocke\60 FPS menus and crash mode** (optional, by SuperType1/remco)

     It also works from the `cheats` folder and the Cheats tab. Check Windows hasn't added a hidden `.txt` to the file name. Restart the game after changing anything.
   - **Patched ISO.** Run `nuzlocke_isopatch.exe` (or drag your clean Burnout Revenge ISO onto it). It opens the patcher in your browser:
     1. Click **Browse…** and choose your ISO (or paste its path).
     2. Pick what goes in. Every part is a button you switch on or off, and the presets fill them in for you (**Classic Nuzlocke**, **Nuzlocke + Chaos**, **Just for fun**):
        - **Nuzlocke rules:** **Block wrecked cars** (what `nuzlocke.exe` needs) and **No bailing out** (the pause-menu block).
        - **Opponents:** Normal, Easy, Medium, Hard or Insane (Insane shows a warning to confirm).
        - **Modes:** Limited Selection, Revive tokens, Event Roulette and Chaos modifiers. They're run by `nuzlocke.exe`, so they need **Block wrecked cars**.
        - **Extras:** Unlock all cars, Widescreen 16:9 and 60 FPS menus. These work on their own too, without the Nuzlocke.
     3. Click **Patch ISO**. It writes a new ISO next to the original, named after what's in it, for example `Burnout Revenge (Nuzlocke, No retry, Hard AI, 16-9).iso`. **Show in folder** takes you to it.

     Add that ISO to PCSX2 and play it, **without** the `.pnach` enabled. Your original ISO isn't changed. Closing the patcher's page closes the patcher. (From a command prompt, `nuzlocke_isopatch.exe game.iso hard widescreen chaos` makes the full Nuzlocke with those parts, without the page.)
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
| On Hard: an event you've already Gold + Perfected | Locked on the World Tour map for the rest of the run. The game never awards Perfect twice, so it couldn't be won again on Hard (with an older patch that doesn't lock it, playing it costs a life). On Easy and Medium a replay can win, so nothing is locked. |

The medal and the rating are checked separately: on Medium, Gold + Awesome and Silver + Awesome both pass, but Silver + Great doesn't. Taking Gold raises your rating one step (that's how a Perfect happens: Gold with Awesome-level driving), so Gold + Good shows as Gold + Great and passes Easy.

**No bailing out.** With the pause-menu block on, Retry, Restart and Quit don't respond, and Retry on the results screens acts like Continue. Every event you start has to be finished, and the next attempt starts from the garage.

**Loaned cars.** Burning Laps and Preview events lend you a car you haven't unlocked yet. If you fail one, the loaned car loses the life, and the control page and overlay show it as "(loaned)". A loaned car is never blocked, so you can always retry the event, but once it's wrecked each retry you fail costs a Race life from the last car of your own you drove in a Race event (or, if that one is wrecked, the car with the most Race lives left). It doesn't count towards your cars until you really unlock it, so it can never keep a dead run going; once it's yours it joins with whatever lives it has left, and if it was wrecked it stays wrecked.

**Wrecked cars.** A car out of **Race** lives is renamed `[RACE X]` and can't be selected in the garage, but can still be picked in crash junctions. A car out of **Crash** lives is renamed `[CRASH X]` and can't be picked in crash junctions, but can still race. A car out of both is `[WRECKED]`. (Short names get `[R X]`, `[C X]`, `[WRECK]` or `[X]`.) This is reapplied every time the game starts.

**Run's dead.** When every car that can race is wrecked in the Race pool, **or** every car that can crash is wrecked in the Crash pool, the control page and stream overlay show your final stats. Every car, including one the event just unlocked, is then blocked in the garage and in crash junctions (shown as `[WRECKED]`), so nothing more can be played until you choose one of these on the control page. A car unlocked by the event that ended the run doesn't count. You then choose:

- **Shut down game:** closes PCSX2.
- **Grace continue:** keep playing with all cars usable again, but nothing counts any more.
- **Start a new run:** begin again with a new difficulty. Use a fresh memory card or profile too.

**Medals.** Under the current car, the control page and overlay show every event that car has driven, in order, as a medal: gold, silver, bronze or an empty ring for no medal. A red ribbon and slash mark a loss, a "C" marks a Crash junction, and faded medals weren't counted (Grace mode). Click a medal on the control page to see the event, its location, the result and when it was. Each result on the overlay also says where it was, for example "Crash - Dock Fight · Motor City".

**Achievements.** A standard run has nine to unlock (none if you use Limited Selection, Revive tokens, Event Roulette, Unlock all cars or Chaos modifiers, even for part of the run): Hot Streak, Unstoppable and Legend (5, 10 and 25 wins in a row), Perfectionist (5 Gold + Perfects), Junction King (5 Crash junctions won), Collector (15 cars of your own), Survivor (50 events in one run), Certified Insane (10 wins against Insane AI), and Last One Standing (a win with only one usable car left, out of at least three). On the control page, click **Achievements** under the current car to see them all; the overlay announces each one after the event's result.

**Run summary.** When the run ends, the control page shows the run's stats (events won, win rate, best streak, run time, Race and Crash wrecks, Gold + Perfects, Crash junctions won, cars owned, revives, difficulty, AI level and modes), how it ended, every car's events, wins and medals, and your achievements; the overlay's end card adds your best car and Gold + Perfects. The same summary is saved as `nuzlocke_summary_<date>_<time>.txt` next to `nuzlocke.exe`, ready to share.

### Modes

Four optional modes are switched on in the patch, like the AI level: tick them in the Patches tab, or switch them on in the ISO patcher. While they're on, the control page shows the picks and your tokens in one line under the current car, and the overlay adds them in small type.

**Limited Selection.** Before every event the tracker picks 2 of your usable cars for Race events and 2 for Crash junctions. Every other healthy car is blocked and shown as `[BENCHED]` (wrecked cars keep their wrecked names), so you choose between the two. A car has one name for both car selects, so the names follow the event you've chosen: in a crash junction's car select every car but the two crash picks shows `[BENCHED]`, in the garage every car but the two race picks. A new pair is picked after every event, never with a car from the last pair unless there's nothing else left; with fewer usable cars you get what's left. Backing out of the garage or restarting the tracker doesn't reroll. Burning Laps and Preview events always let you drive their car, even one you own that isn't picked, and Grace mode lifts the bench.

**Revive tokens.** Wins in a row earn tokens, harder the higher the difficulty:

| Difficulty | One token every | Most you can hold |
| --- | --- | --- |
| Easy | 3 wins in a row | 3 |
| Medium | 5 wins in a row | 2 |
| Hard | 8 wins in a row | 1 |

Click **revive a car** next to your tokens on the control page to give a wrecked car of your own one life back in the pool it's wrecked in. If an event would end the run while you hold a token, the token is used on the car that was just wrecked, and the run goes on. Tokens can't be used once the run is over or in Grace mode.

**Event Roulette.** The tracker picks the event you play next, at random from every World Tour event: ones you haven't won yet in this run first, and once you've won them all, any event. It never picks the same event twice in a row. The control page and overlay show it, for example "ROULETTE Crash - Dock Fight (Rank 1)". Every other event shows as locked on the World Tour map, so you can only play the one picked (if an event somehow gets played anyway, it counts as a loss for the car you drove). Every rank tab on the map is open. If the roulette picks a Burning Lap or Preview whose car is one of yours and wrecked, it's a freebie: no life lost, and the roulette moves on as soon as that event's garage shows the car. You earn a reroll for every 5 events you win (hold up to 3); click **reroll** on the control page to spend one. Your rank, results and cars are untouched. With the extra option **Mode\Event Roulette - Unlock all cars** (or **Unlock all cars** in the ISO patcher), every car is unlocked from the start too, so a rank 10 event never has to be raced with starter cars; each car joins the run with full lives the first time it shows up in the garage. With Limited Selection on, only the pair for the roulette event's type is shown.

**Chaos modifiers.** Every event gets a modifier, rolled by the tracker after the event before it and shown on the control page and the overlay ("CHAOS Safety Net"). Good ones are yellow, bad ones orange. The same modifier never comes twice in a row (Calm aside).

| Modifier | What it does |
| --- | --- |
| Calm | Nothing: a normal event. |
| Safety Net | A loss costs no life. |
| Second Wind | A win gives a life back to your car with the fewest lives in that pool (a wrecked one first), up to the starting lives. |
| Easy Street | The requirement drops a step: Bronze + Good on Easy, Bronze + Great on Medium, any Gold on Hard. |
| Double or Nothing | A loss costs 2 lives; a win gives a life back like Second Wind. |
| Sudden Death | A loss wrecks the car. |
| Gold or Bust | Only a Gold counts (with the difficulty's rating). Not on Hard, which needs a Gold anyway. |
| Lone Wolf | One car picked at random (from the Limited Selection pair when that's on) is the only one you can drive; the rest are `[BENCHED]`, one for Race events and one for Crash junctions. |

Burning Laps and Previews still let you drive their own car under Lone Wolf.

The stats are events won out of events played, cars wrecked in each pool, run time (time with the game running), your best win streak, and the difficulty and AI level.

## FAQ

**Can I close the tracker between sessions?**
Yes. Your run is saved in `nuzlocke_state.json` next to `nuzlocke.exe`. Start it again before your next event and it carries on.

**Can I quit or retry an event to avoid losing a life?**
Not with the pause-menu block on: those options do nothing. Without it, the tracker only judges events that finish with a result.

**Can I change difficulty mid-run?**
Lives, no: they're locked for the run, so start a new run to change them. The AI level can be changed between sessions, and the overlay shows whichever is active.

**What does Harder AI change?**
Two things, together. Speed: opponents corner faster, keep their pace when out of sight, catch up when far behind you, and ease off when far ahead so they can be caught. Aggression: they go for takedowns more often and from further away, start sooner after the green light, block you for longer and slam harder. On Hard and Insane every opponent attacks.

**What's Insane?**
A fourth AI level for when Hard isn't enough: opponents keep the full speed Hard had before the aggression was added (cornering right at the limit, 257 mph, the strongest catch-up), and attack almost constantly, from the start line and from up to 150 m away, blocking you for up to 15 s and slamming twice as hard. It isn't meant to be fair; with 1 + 1 lives most runs will end within a few events.

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
- The Harder AI levels may still be tuned after more play-testing (the numbers are all in `tools/build_harder_ai.py`).
- Patched ISOs have been checked byte for byte against the `.pnach`, but not yet booted on every setup.
