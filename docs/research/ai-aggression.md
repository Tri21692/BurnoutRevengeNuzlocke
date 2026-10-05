# AI aggression: attacks, slams and blocking

Burnout Revenge, SLUS-21242. Found from the developer tuning menu's names, which are still in the game
(`AI/Aggressive Driving`, `AI/Aggressive Driving/Slam`, `AI/Aggressive Driving/Reactions`). The values
are loaded at run time from `../Data/Export/ValueDB/AI/defaults.cfg` and `Physics.cfg`, so they're in RAM
only, not in SLUS_212.42: a patch has to keep writing them (as a `.pnach` does).

## AI driving block `01C913E8` (registered by `00292010`)

The same block the speed research uses ("Out of range speed decrease rate" +0x00, "Top speed mps"
+0x0C, "Min speed mps" +0x10, speed matching distance +0xB8). The attack settings:

| Offset | Address | Name in the game |
|---|---|---|
| +0x7C | 01C91464 | Aggression time variation factor |
| +0x80 | 01C91468 | Aggression dist variation factor |
| +0x84 | 01C9146C | Aggression time variation offset |
| +0x88 | 01C91470 | Aggression dist variation offset |
| +0xB8 | 01C914A0 | How close the target car must be to start proper speed matching (m) |
| +0xBC | 01C914A4 | Maximum distance from player to start doing sticky speed matching (m) |
| +0xC0 | 01C914A8 | Maximum difference in speeds for sticky speed matching (mph) |
| +0xC4 | 01C914AC | Min. aggression before we start attacking |
| +0xC8 | 01C914B0 | Min. time to wait between attacks (s) |
| +0xCC | 01C914B4 | Max. time to wait between attacks (s) |
| +0xD0 | 01C914B8 | Max. distance apart to begin attacking when ahead of target (m) |
| +0xD4 | 01C914BC | Max. distance apart to begin attacking when behind target (m) |
| +0xD8 | 01C914C0 | Min. target speed to consider attacking (mph) |
| +0xDC | 01C914C4 | How much slower than the player to drive while speed matching to attack position |
| +0xE8 | 01C914D0 | How long to wait at the start of the race before doing aggressive driving (s) |
| +0xEC | 01C914D4 | How long after hitting something to disable immunity (s) |
| +0xF0 | 01C914D8 | Min time to try and block you for (s) |
| +0xF4 | 01C914DC | Max time to try and block you for (s) |
| +0xF8 | 01C914E0 | Max distance ahead to start blocking you (m) |
| +0xFC | 01C914E4 | Preferred car separation when getting into slam position (m) |
| +0x100 | 01C914E8 | Max. time to try and get into slamming position (s) |
| +0x104 | 01C914EC | Max. distance between cars, when ahead (m) |
| +0x108 | 01C914F0 | Max. difference in speeds (mph) |
| +0x10C | 01C914F4 | Steer out distance (m) |
| +0x110 | 01C914F8 | Steer out time (s) |
| +0x114 | 01C914FC | Slam time (s) |
| +0x118 | 01C91500 | Max. cos angle off lane to stop attack |
| +0x11C | 01C91504 | How soon after starting to slam is the AI car committed and can't stop |
| +0x120 | 01C91508 | How long a rubbed AI car goes blind for (s) |

+0xB8 (10 m) and +0xDC (0.999) are also used by the speed catch-up (`0029A3B8`), so changing them
changes the Harder AI levels too.

## Slam and shunt forces `0045F0E8` (registered by `00201100`, from `Physics.cfg`)

Values as read live in a race:

| Address | Name | Value |
|---|---|---|
| 0045F0F0 / F4 / F8 | Player Neutral / Level 1 / Level 2 Slam Force | 10 / 15 / 20 |
| 0045F0FC / 100 / 104 | Player Neutral / Level 1 / Level 2 Shunt Force | 10 / 15 / 20 |
| 0045F108 / 10C / 110 | AI Neutral / Level 1 / Level 2 Slam Force | 30 / 35 / 40 |
| 0045F114 / 118 / 11C | AI Neutral / Level 1 / Level 2 Shunt Force | 30 / 32 / 35 |
| 0045F120 | Max Slam Shunt Speed | 60 |
| 0045F124 | Slam Attackers Reaction | 0.2 |
| 0045F128 | Slam Application Factor | 0.22 |
| 0045F12C | Shunt Application Factor | 0.18 |
| 0045F130 | Post Shunt Breaking Factor | 0.65 |

## Still to do

- Read the live values of the attack block (`peek 01C914AC 24` in ps2scan during a race).
- Find where each opponent's own aggression is kept (compared with +0xC4).
