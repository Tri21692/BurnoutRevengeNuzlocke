# AI catch-up (rubberbanding) – research notes

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

## Tuning block `01C913E8` (read live)

+0x00 10 (far-mode max decel/frame), +0x08 90, +0x0C 88 (speed cap ceiling, m/s), +0x10 20 (min
speed), +0x50 100, +0xB8 10 (duel gap threshold, m), +0xDC 0.999 (duel ease-off factor), +0xF8 15
(duel range). Full dump: 41200000 3F800000 42B40000 42B00000 41A00000 3F666666 41A00000 41200000 …

## Patches: `[Nuzlocke\Harder AI\Easy|Medium|Hard]` in the .pnach (enable one)

| Level | Corner factor (aggressive) | Far-mode schedule offset | Speed cap ceiling |
|---|---|---|---|
| game | 0.8 (0.85) | −4..+4 s | 88 m/s |
| Easy | 0.875 (0.925) | −5..−2 s | 100 m/s |
| Medium | 0.925 (0.975) | −7..−4 s | 100 m/s |
| Hard | 0.975 (1.0) | −9..−6 s | 100 m/s |

- corner factor: `0029EE1C/20`, aggressive `0029EE30/34`
- schedule offset clamp: low `0029A138`, `0029A298`; high `0029A154`, `0029A2B4`
- speed cap ceiling 88 → 100 m/s by reading tuning +0x50 instead of +0x0C: `0029A1A0`, `0029A300`,
  and the flat near-player cap `0029ABE0`

An earlier single test version (0.9/0.95, −6..−3) was confirmed working in game.

## Still to check

- Whether racer+0x24F4 (schedule column 0–7) changes with difficulty or event rank.
- `00298CD0` returns car+0x2E41, which looks like "full physics / near the player": a live log
  showed caps at a flat 88 m/s most of the time, dropping to the schedule cap (54–59 m/s) only
  while an opponent was in far mode. So the schedule only governs cars away from the player, and
  its plain pace (offset 0) is slow, about 55 m/s.
