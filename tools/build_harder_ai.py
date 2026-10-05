"""Writes the [Nuzlocke\\Harder AI\\Easy|Medium|Hard|Insane] groups of the .pnach from the table below: the
opponents' speed (cornering, pace, speed cap, catch-up, ease-off) and their aggression (takedown
attempts, blocking, slam force) in one option per level. Run it after changing a number here, then run
build_isopatch.py. See docs/research/ai-catchup.md and docs/research/ai-aggression.md.
"""
import os, re, struct, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import build_catchup as C

PNACH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "patches", "SLUS-21242_D224D348.pnach")

def fbits(x): return struct.unpack("<I", struct.pack("<f", x))[0]

# Speed. corner: racing-line factor (normal, aggressive stretches; game 0.8, 0.85). pace: far-mode
# offset to each section's planned time, s (game -4..+4). cap: speed cap ceiling, m/s (game 88).
# catch-up from d0 m behind, k m/s of boost per m, up to bmax m/s. ease-off: more than `ahead` m in
# front of the player, held to factor x the player's speed, at least minspeed m/s.
SPEED = {
    "Easy":   dict(corner=(0.8625, 0.9125), pace=(-4.5, -1.5), cap=97,  d0=40, k=0.225, bmax=18,
                   ahead=40,  factor=0.90, minspeed=40),
    "Medium": dict(corner=(0.9125, 0.9625), pace=(-6.5, -3.5), cap=107, d0=25, k=0.36,  bmax=27,
                   ahead=70,  factor=0.94, minspeed=45),
    "Hard":   dict(corner=(0.9625, 0.9875), pace=(-8.5, -5.5), cap=112, d0=10, k=0.54,  bmax=36,
                   ahead=100, factor=0.97, minspeed=50),
    # Insane: Hard's speed from before it was toned down for the aggression (V1.1.2's first Hard)
    "Insane": dict(corner=(0.975, 1.0),     pace=(-9, -6),     cap=115, d0=10, k=0.6,   bmax=40,
                   ahead=100, factor=0.97, minspeed=50),
}

# Aggression: the game's own AI/Aggressive Driving settings and the AI slam/shunt forces.
# (address, what, stock, Easy, Medium, Hard, Insane)
AGGRESSION = [
    (0x01C914AC, "Min. aggression before attacking", 0.2, 0.1, 0.05, 0.0, 0),
    (0x01C914B0, "Min. time between attacks (s)", 0.38, 0.3, 0.25, 0.2, 0.1),
    (0x01C914B4, "Max. time between attacks (s)", 3, 2.25, 1.5, 1.0, 0.5),
    (0x01C914B8, "Max. distance to start an attack when ahead (m)", 40, 50, 60, 75, 100),
    (0x01C914BC, "Max. distance to start an attack when behind (m)", 70, 85, 100, 120, 150),
    (0x01C914C0, "Min. speed to consider attacking (mph)", 70, 60, 50, 40, 30),
    (0x01C914D0, "Wait at the start before attacking (s)", 3, 2, 1.5, 1, 0.5),
    (0x01C914D8, "Min. time to block you (s)", 3, 3.5, 4, 5, 6),
    (0x01C914DC, "Max. time to block you (s)", 8, 9, 10, 12, 15),
    (0x01C914E0, "Max. distance ahead to start blocking you (m)", 15, 20, 25, 30, 40),
    (0x01C914E8, "Max. time to get into slamming position (s)", 15, 18, 20, 25, 30),
    (0x01C914FC, "Slam time (s)", 1.5, 1.75, 2.0, 2.25, 2.5),
    (0x01C91508, "Time a rubbed opponent goes blind (s)", 1, 0.75, 0.5, 0.3, 0.15),
    (0x0045F108, "AI neutral slam force", 30, 34.5, 39, 45, 60),
    (0x0045F10C, "AI level 1 slam force", 35, 40.25, 45.5, 52.5, 70),
    (0x0045F110, "AI level 2 slam force", 40, 46, 52, 60, 80),
    (0x0045F114, "AI neutral shunt force", 30, 34.5, 39, 45, 60),
    (0x0045F118, "AI level 1 shunt force", 32, 36.8, 41.6, 48, 64),
    (0x0045F11C, "AI level 2 shunt force", 35, 40.25, 45.5, 52.5, 70),
]
SLAM_MORE = {"Easy": 15, "Medium": 30, "Hard": 50, "Insane": 100}
LEVELS = ("Easy", "Medium", "Hard", "Insane")

def lui(f):
    b = fbits(f); assert b & 0xFFFF == 0, f"{f} needs more than lui"
    return b >> 16

def group(level):
    n = LEVELS.index(level) + 1
    s = SPEED[level]
    cn, ca = s["corner"]; lo, hi = s["pace"]; cap = s["cap"]
    p = lambda a, v: f"patch=1,EE,{a:08X},word,{v:08X}"
    L = [f"[Nuzlocke\\Harder AI\\{level}]", "author=Nuzlocke mod",
         f"description=Harder opponents, {level} level: faster and more aggressive. Enable only one Harder AI level. "
         + ("WARNING: Insane is not meant to be fair; most runs will end within a few events. " if level == "Insane" else "")
         + f"Corners at {cn:g} of the racing line's limit ({ca:g} on aggressive stretches); far from the player, "
         f"{-hi:g}-{-lo:g} s per section quicker than their pace schedule; catch-up from {s['d0']} m behind the player "
         f"(up to +{s['bmax']} m/s); ease-off from {s['ahead']} m ahead of the player (to {s['factor']:g} x the "
         f"player's speed); speed cap {cap} m/s. Opponents go for takedowns more often, from further away and sooner "
         f"after the start, block you for longer and slam {SLAM_MORE[level]}% harder"
         + (", and every opponent attacks" if level in ("Hard", "Insane") else "") + ". See docs/research/ai-catchup.md and ai-aggression.md.",
         "// Corner speed factor (game: 0.8, aggressive 0.85)"]
    for a, f in ((0x0029EE1C, cn), (0x0029EE30, ca)):
        b = fbits(f)
        L += [p(a, 0x3C010000 | b >> 16), p(a + 4, 0x34210000 | b & 0xFFFF)]
    L.append(f"// Far-mode pace: offset to each section's planned time, clamped to {lo:g}..{hi:g} s (game: -4..+4)")
    L += [p(0x0029A138, 0x3C010000 | lui(lo)), p(0x0029A298, 0x3C010000 | lui(lo)),
          p(0x0029A154, 0x3C010000 | lui(hi)), p(0x0029A2B4, 0x3C010000 | lui(hi))]
    L += [f"// Speed cap ceiling 88 -> {cap} m/s ({round(cap * 2.2369)} mph), loaded as a constant instead of tuning value 01C913F4.",
          "// Far from the player (two copies): lui at,cap / mtc1 at,f3 / min speed via lwc1 f1,0x13F8(v0) / clamp."]
    for base in (0x0029A190, 0x0029A2F0):
        L += [p(base, 0x3C010000 | lui(cap)), p(base + 8, 0x44811800), p(base + 12, 0xC44113F8),
              p(base + 16, 0x46010028), p(base + 20, 0x46030029)]
    L += ["// Near the player (flat cap): lui at,cap / sw at,0xA44(s0) in the branch delay slot.",
          p(0x0029ABE0, 0x3C010000 | lui(cap)), p(0x0029ABE8, 0xAE010A44),
          "// Catch-up: the brain update's call to the target-speed function (00298FEC) goes through a wrapper at",
          f"// 000FFA00. When this opponent is more than {s['d0']} m behind the player (track progress), boost =",
          f"// min({s['k']:g} x (gap - {s['d0']}), {s['bmax']}) m/s: its target speed becomes at least the player's speed + boost, and",
          "// its top speed (car+0x1354, mph) is set to stock + boost. Otherwise top speed is reset to stock.",
          f"// When it is more than {s['ahead']} m AHEAD of the player, its target speed is held to {s['factor']:g} x the player's speed",
          f"// (but at least {s['minspeed']} m/s), so a leader can be caught.",
          p(0x00298FEC, 0x0C000000 | (C.BASE >> 2))]
    ws = C.build(s["d0"], s["k"], s["bmax"], s["ahead"], s["factor"], s["minspeed"])
    L += [p(C.BASE + 4 * i, w) for i, w in enumerate(ws)]
    L.append("// Aggression: the game's own attack settings (AI/Aggressive Driving in its tuning data) and the AI's")
    L.append("// slam and shunt forces. They're loaded from the game's data, so they're written every frame.")
    for a, what, stock, *vals in AGGRESSION:
        L += [f"// {what}: {stock:g} -> {vals[n - 1]:g}", p(a, fbits(vals[n - 1]))]
    L += ["// Difficulty marker for the tracker: 000FE110 = 1 Easy, 2 Medium, 3 Hard, 4 Insane", p(0x000FE110, n)]
    return "\n".join(L) + "\n"

def main():
    text = open(PNACH).read()
    # drop every Harder AI and Aggressive AI group, then put the new Harder AI groups where the first was
    parts = re.split(r"(?m)^(?=\[)", text)
    keep, at = [], None
    for part in parts:
        if re.match(r"\[Nuzlocke\\(Harder|Aggressive) AI\\", part):
            if at is None:
                at = len(keep)
            continue
        keep.append(part)
    assert at is not None, "no Harder AI group in the .pnach"
    new = "\n".join(group(l) for l in LEVELS)
    if at < len(keep):
        new += "\n"
    keep.insert(at, new)
    open(PNACH, "w", newline="\n").write("".join(keep))
    print("wrote the Harder AI groups to", os.path.normpath(PNACH))

if __name__ == "__main__":
    main()
