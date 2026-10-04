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

## Who gets catch-up: `00291EF0`

Each frame mode is reset to 0, then set to 4 (or 5 if brain+0x192) with reference = a racer found by
`00290F48`, if: global enable `[01ED1630 + 0x170000 + 0x6A28]` and brain+0x191 are set, and the
reference is going at least 100 mph (`00291FBC` lui 0x42C8; speed × 2.2369 compared to 100).
In the dump only one opponent was in mode 4 at a time.

## Still to read

- The tuning block at `01C913E8` (not in the dump): +0x00 max decel/frame, +0x0C, +0x10 min speed,
  +0xB8 gap threshold, +0xDC ease-off factor.
- `00290F48`: how the reference racer is chosen (distance limit?).
- How modes are assigned across the five opponents during a whole race.
