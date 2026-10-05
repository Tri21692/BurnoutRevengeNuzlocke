# AI catch-up and pause menu: research notes

Burnout Revenge, SLUS-21242. Found from a RAM dump taken mid-race (code 00100000-005FFFFF,
car/racer data 01DD0000-01EFFFFF). All addresses are PS2 EE addresses.

## Data layout

| What | Player | Opponents | Notes |
|---|---|---|---|
| Car (physics) object | `01DD2490` | `01DE0950` + n × `0x2E80` | velocity xyz/speed at +0xA0, direction at +0xB0, top speed (mph) at +0x1354, racer pointer at +0x2E5C |
| Racer object | `01ED8070` | `01EDF6D0` + n × `0x37B0` | car pointer at +0x3780, state object at +0x2CB0, AI "brain" at +0x2D00 |
| AI brain (racer + 0x2D00) | | | catch-up mode +0x7D0, catch-up reference racer +0x7D4 (the player), own racer +0x7E0, racing-line speed +0x7C0, **target speed +0xA04** (= racer + 0x3704) |

Speeds are metres per second. The opponents' addresses held across a race but may move between events.

## How an opponent's speed is decided (each frame)

1. `002998E0` (brain): computes the **target speed** into brain+0xA04.
   - base = `0029A320` (racing-line speed, slowed for steering/corners)
   - before the race starts (racer+0x2500 == 0): target = min(base, 22.35) – the 50 mph rolling start
   - during the race: target = `0029A3B8`(brain, base) – **catch-up**
   - floor: max(target, `[01C913F8]`)
2. Near the player (full physics), `001DC0AC…` steers throttle/brake toward the target
   (throttle if target − speed > 1, brake if < −2.235).
3. Far from the player ("far mode", no physics), `002971E8` sets the velocity directly toward the
   target via `001EF000` (set forward speed) → `001EEE90` (set velocity).
   Max change per frame: down by `[01C913E8]`, up by top speed (mph) / 9.

## The catch-up function `0029A3B8`

Uses brain+0x7D0 (mode) and brain+0x7D4 (reference racer, the player):

| Mode | Behaviour |
|---|---|
| 0 | no catch-up, racing-line speed |
| 1 / 2 | copy the player's speed (depending on who's ahead) |
| 3 | copy the player's speed |
| 4 | **main race catch-up** (below) |
| 5 | approach (player speed × (1 + gap/100)) by at most ±1 m/s per frame |

Mode 4 (track progress from `002C78C0`; gap = player progress − AI progress, positive = AI behind):

```
lead   = gap + 2·(v_player − v_ai) + 20        // aims to sit ~20 m ahead of the player
target = max(v_player, v_ai + 0.5·lead)        // never slower than the player
if |gap| > [01C914A0]:
    if gap > 0:  target = 2·target             // far behind: double it   (0029A6BC add.s f21,f21,f21)
    else:        target = max(target·[01C914C4], base − 20)   // far ahead: ease off
```
If racer+0x3790 == 1, the AI car's top speed is also set to the player's top speed + 30 mph.

Constants in code: 20 m lead (`0029A5D8` lui 0x41A0), gain 0.5 (`0029A674` lui 0x3F00),
ease-off floor −20 (`0029A6C0` lui 0x41A0).

## Pace schedule and speed cap (`0029AB60`, `0029A060` / `0029A1C0`, `00298E90`)

This is the main thing that decides how fast opponents go, and it doesn't look at the player.
Each opponent has a planned time for each of 8 track sections (brain+0x800 normal, brain+0x900
alternative; index = section × 8 + racer+0x24F4, which was 0 for all). The 5th opponent has the
quickest schedule and the 1st the slowest (13.5–19.6 s per section).

- `00298E90` (brain update) keeps a running planned time and compares it with the race clock
  (racer+0x24EC): brain+0xA40 = seconds ahead (+) or behind (−) schedule.
- Speed cap brain+0xA44 (= racer+0x3744) = section length / (planned section time + clamp(A40, −4, +4)),
  clamped to [`[01C913F8]` 20, `[01C913F4]` 88] m/s. Ahead of schedule → slower, behind → faster.
- The racing-line speed (`0029EDD0` → brain+0x1D0 → brain+0x7C0) is line limit × 0.8 (0.85 when
  brain+0x201 is set) plus corrections, then capped by the speed cap above.

In the dump the caps were 50–52 m/s for three opponents (ahead of schedule) and 88 m/s for two.

## Who gets catch-up: `00291EF0`

Each frame mode is reset to 0, then set to 4 (or 5 if brain+0x192) with reference = a racer found by
`00290F48`, if: global enable `[01ED1630 + 0x170000 + 0x6A28]` and brain+0x191 are set, and the
reference is going at least 100 mph (`00291FBC` lui 0x42C8; speed × 2.2369 compared to 100).
In the dump only one opponent was in mode 4 at a time.

The reference racer is only kept while the two cars are within 10 mph of each other and close
(`00290F98`: progress gap between racer+0x3788 and that + 15 m), so modes 4/5 are a short-range
"duel with the player", not a whole-race rubberband. A live log showed modes 4/5 switching on and
off for all five opponents, mostly in the pack at the start, then for whoever is near.

## The aggressive flag (brain+0x201)

Not a personality setting: the racing line lookup `0029B878` returns it per stretch of the line
(result byte +0x30), and `0029FA84` stores it every frame (cleared at `0029E644`). Where it is set:

- corner factor 0.85 instead of 0.8 (`0029EE18`);
- the target speed ceiling in `0029A378` is 1.45 x (`3FB9999A`) the computed speed instead of 1 x;
- the line look-ahead in `002A0C48` adds `[01C9149C]`.

`[Nuzlocke\Aggressive opponents]` loads 1 instead of the returned byte (`0029FA6C` `lbu v1,0x30(sp)`
becomes `addiu v1,zero,1`), so every stretch counts as aggressive. Nothing found so far decides
ramming or takedown attempts; that would be elsewhere in the AI.

## Tuning block `01C913E8` (read live)

+0x00 10 (far-mode max decel/frame), +0x08 90, +0x0C 88 (speed cap ceiling, m/s), +0x10 20 (min
speed), +0x50 100, +0xB8 10 (duel gap threshold, m), +0xDC 0.999 (duel ease-off factor), +0xF8 15
(duel range). Full dump: 41200000 3F800000 42B40000 42B00000 41A00000 3F666666 41A00000 41200000 …

## Patches: `[Nuzlocke\Harder AI\Easy|Medium|Hard]` in the .pnach (enable one)

| Level | Corner factor (aggressive) | Far-mode schedule offset | Speed cap ceiling |
|---|---|---|---|
| game | 0.8 (0.85) | −4..+4 s | 88 m/s |
| Easy | 0.875 (0.925) | −5..−2 s | 100 m/s |
| Medium | 0.925 (0.975) | −7..−4 s | 110 m/s |
| Hard | 0.975 (1.0) | −9..−6 s | 115 m/s |

- corner factor: `0029EE1C/20`, aggressive `0029EE30/34`
- schedule offset clamp: low `0029A138`, `0029A298`; high `0029A154`, `0029A2B4`
- speed cap ceiling (all levels use the same patched instructions, so switching levels can't leave a
  mix behind): a constant. In both cap functions the `addiu v0,v0,0x13E8` slot
  becomes `lui at,cap`, then `mtc1 at,f3` / `lwc1 f1,0x13F8(v0)` / `max.s` / `min.s f0,f0,f3`
  (`0029A190`–`0029A1A4`, `0029A2F0`–`0029A304`). Near-player cap: `0029ABE0` `lui at,cap`,
  delay slot `0029ABE8` `sw at,0xA44(s0)`. No branches target the changed instructions.

### Catch-up wrapper (000FFA00)

`00298FEC` (`jal 002998E0` in the brain update) becomes `jal 000FFA00`. The wrapper calls `002998E0`,
then, if the player exists (`[01ED8058]`), this isn't the player's own racer (`01ED8070`) and the
race is running (racer+0x2500), compares track progress (`002C78C0` on racer+0x24E0, player
racer+0x24E0 = `01EDA550`). With d = gap − start distance and boost = min(k·d, max):

- target (brain+0xA04) = max(target, player speed + boost) (player car = `[01EDB7F0]`, speed +0xAC)
- top speed car+0x1354 (mph) = stock (`[[car+0x1384]+0x1C0]`) + boost × 2.2369; reset to stock when
  not far enough behind

| Level | start distance | k (m/s per m) | max boost | eases off when ahead by | to (x player speed) | but at least |
|---|---|---|---|---|---|---|
| Easy | 40 m | 0.25 | 20 m/s | 40 m | 0.90 | 40 m/s |
| Medium | 25 m | 0.4 | 30 m/s | 70 m | 0.94 | 45 m/s |
| Hard | 10 m | 0.6 | 40 m/s | 100 m | 0.97 | 50 m/s |

Since V1.1.2 the wrapper also eases leaders off: when the opponent is further ahead of the player than
the ease-off distance, its target becomes min(target, max(player speed x factor, minimum)). The wrapper
is 0x1B8 bytes; in patched ISOs it sits at 00479800 (00479E80 before V1.1.2).

Source: [`tools/build_catchup.py`](../../tools/build_catchup.py). Each level also writes its number (1–3) to `000FE110` for the tracker.

An earlier single test version (0.9/0.95, −6..−3) was confirmed working in game.

## Still to check

- Whether racer+0x24F4 (schedule column 0–7) changes with difficulty or event rank.
- `00298CD0` returns car+0x2E41, which looks like "full physics / near the player": a live log
  showed caps at a flat 88 m/s most of the time, dropping to the schedule cap (54–59 m/s) only
  while an opponent was in far mode. So the schedule only governs cars away from the player, and
  its plain pace (offset 0) is slow, about 55 m/s.

## Pause menu (not AI, kept here for now)

Strings `$PRRetryRace`, `$REALLYRESTART`, `$QUIT`, `$REALLYQUIT`, `$REALLYRESTARTJUNCTION` at
`004B7BD8`–`004B7D30` are used by two pause handlers:

- around `00190800`: state at +0x10; on a selection, item −1 → Retry confirm (`001908C8`), −2 → Quit
  confirm (`001908F8`), 0 → resume. Patch: nop the branches at `0019088C` and `0019089C`.
- around `001910DC`: jump table `004B7D70` by item + 1: [0] Quit `001911F8`, [1] `00191148`,
  [2] Retry `00191110`, [3] `0019115C`, [4] none, [5] Restart junction `00191230`, [6] Retry
  `00191268`. Patch: entries 0, 2, 5, 6 → exit `00191580`.

## Patched ISOs (`isopatch/`, `nuzlocke_isopatch.exe`)

The patched game file keeps its original layout (ELF header, program headers, sections); only words
change, like the .pnach does in memory. The mod's code blocks, which the .pnach keeps below the game,
move into unused space inside `.data`: the end of a 40 KB block of zeros (00470258–0047A25F) that no
code, data or heap pointer refers to and that was all zeros mid-race.

| Block | .pnach | Patched ISO |
|---|---|---|
| dead-car garage block | 000FF000 | 00479D00 |
| finished-event signal | 000FF100 | 00479DA0 |
| crash junction block | 000FF140 | 00479DD0 |
| AI catch-up wrapper | 000FFA00 | 00479800 (00479E80 before V1.1.2) |
| difficulty level | 000FE110 (marker) | 00479FF0 (word) |
| widescreen HUD writer (optional) | — (.pnach writes the values) | 00479B00 |

j/jal hooks into the old area are retargeted. The widescreen patch's HUD values live in `.bss`, so in a
patched ISO a writer routine at 00479B00 stores them every frame: the main loop's `jal 00185E60` at
001044E0 becomes `jal 00479B00`, which writes the values using only `at` and `v1` and then `j 00185E60`. Data the tracker uses (000FE100 counter, 000FF400
dead-car table) stays in low RAM.

The first patcher (V1.0) instead added a third program header for a new section at 004A3500 and moved
the program header table to make room. PCSX2 crashed on boot with it ("Jump to unmapped recLUT page,
PC 0x02000000"), most likely because the loader doesn't handle a moved program header table.

`tools/build_isopatch.py` generates `isopatch/patches.go` from the .pnach; `tools/verify_isopatch.py`
checks a patched ISO's loaded memory image against the original plus the .pnach (all three levels: 0
mismatches; ELF headers and section table unchanged).

## Tracker data used for the two life pools

- Current car and event: `[01C10C18]` points to an object with the loaded car's label at +0x00 (the
  crash junction Select handler uses it as "the car currently loaded") and the event's label at +0x18
  (the career update `00133760` looks it up in the event list to pick the result slot). The tracker
  reads it when an event starts or its result is stored, so replays get a name and the right pool.
- Crash junctions are the events whose label has `DH` after the number (`K_01DH1E` = Crash - Dock Fight).
- Dead-car tables: `000FF400` (Race pool, garage block) and `000FF700` (Crash pool, crash junction
  block; `ori t4,t4,0xF700` at `000FF180`, `00479E10` in a patched ISO). Patches older than V1.1 read
  `000FF400` in both places, so with those the tracker puts cars wrecked in either pool in it.
- Results-screen Retry: race results handler `0019AD98` command 8, and item 1 in the end-of-event handlers
  `00191B58` and `00191FA8`, all call the restart method (vtable +0x44 at
  `[[004F3300+0x585E8]+4]+0x8000+0x315C`). Simply skipping them (V1.1) froze the screen: the handlers'
  exit sets a busy flag (screen +0x50, `01C0F909`) that only a real action clears. Since V1.1.1 Retry is
  redirected to Continue's code instead: `0019ADC4` b `0019AED8` (command 7) with `daddu s0,a0` in the
  delay slot; `00191CC0` b `00191C38` (item 0) with `lui v0,0x1F6`; `0019206C` b `001921A0` (item 0/-2)
  with `lui v1,0x4F`.

## Run's dead lock (V1.1.2)

The tracker writes 1 to `000FE120` while the run is dead. The three results handlers (`0019AD98`,
`00191B58`, `00191FA8`) start with `j` to stubs at `000FF200`/`240`/`280` (`00479C00`/`40`/`80` in a
patched ISO) that return straight away for message 7/8 (race results: Continue/Retry) or 5 (the other two:
a selection) while the flag is set, before the handler sets its busy flag; otherwise they run the two
replaced instructions and jump back. Source: `tools/build_deadlock.py`. (The front end's `01C0A2D0+0x5639`
byte is a "go to the next screen" request, not an input lock, so it isn't usable for this.)
