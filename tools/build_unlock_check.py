"""Writes part 4 of [Nuzlocke\\Block dead cars in garage] in the .pnach: the event unlock check at
000FF200, from the assembly below. Run it after changing the routine, then run build_isopatch.py.

The game's unlock check (00134568) asks whether an event can be played: its rank against yours, then
the race and crash chains in that rank. Its first test, lbu v0,0x400(s0) (the profile's debug
"everything unlocked" byte), becomes j 000FF200, which decides first:
  1. Event Roulette's pick (label at 000FE140, written by the tracker): only that event is unlocked.
  2. Hard run (000FE3FC = 1, written by the tracker): an event whose saved result is Gold + Perfect
     (04 in the results, 01F651DB + its place in the event list) is locked. The routine reads the
     result itself, so an event is locked the moment the game saves the Perfect, even though the game
     works out what's unlocked before the tracker has seen the result.
  3. The lock table at 000FE400 (8-byte labels, ending with 0, written by the tracker): locked.
  4. Event Roulette on (marker 000FE138, or 00479FFC in a patched ISO): unlocked.
  5. Otherwise the game's own check.
a0 = s0 = the profile, a1 = the event (its label at +0x18). Only t0-t5 are used.
"""
import os, re

PNACH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "patches", "SLUS-21242_D224D348.pnach")
BASE = 0x000FF200
HOOK = 0x00134580  # lbu v0,0x400(s0) in the game's unlock check
UNLOCKED, LOCKED, GAME_CHECK = 0x0013458C, 0x00134610, 0x00134594  # b 134610 with v0=1 / return / the rank test

R = {"zero": 0, "at": 1, "v0": 2, "v1": 3, "a0": 4, "a1": 5, "t0": 8, "t1": 9, "t2": 10, "t3": 11, "t4": 12,
     "t5": 13, "s0": 16, "s1": 17}

# (label or None, instruction)
ASM = """
        lui   at, 0x0010
        ld    t1, 0x18(a1)          # the event's label
        ld    t0, -0x1EC0(at)       # 000FE140: Event Roulette's pick
        beq   t0, zero, hard
        nop
        bne   t0, t1, locked        # a pick: only it is unlocked
        nop
        j     UNLOCKED
        nop
hard:   lw    t0, -0x1C04(at)       # 000FE3FC: 1 on a Hard run
        beq   t0, zero, table
        lui   t2, 0x0056
        ori   t2, t2, 0xB250        # 0056B250: the event list's labels
        lui   t3, 0x0057
        lw    t3, -0x4E14(t3)       # 0056B1EC: how many events
        lui   t4, 0x01F6
        ori   t4, t4, 0x51DB        # 01F651DB: the saved results, one byte per event
find:   blez  t3, table
        nop
        ld    t5, 0(t2)
        bne   t5, t1, next
        nop
        lbu   t5, 0(t4)
        addiu t5, t5, -4
        beq   t5, zero, locked      # Gold + Perfect: locked
        nop
        b     table
        nop
next:   daddiu t2, t2, 8
        addiu t4, t4, 1
        b     find
        addiu t3, t3, -1
table:  lui   t2, 0x0010
        ori   t2, t2, 0xE400        # 000FE400: the tracker's lock table
loop:   ld    t3, 0(t2)
        beq   t3, zero, mode
        nop
        beq   t3, t1, locked
        daddiu t2, t2, 8
        b     loop
        nop
mode:   lw    t0, -0x1EC8(at)       # 000FE138: Event Roulette on (.pnach)
        lui   t3, 0x0048
        lw    t3, -0x6004(t3)       # 00479FFC: Event Roulette on (patched ISO)
        or    t0, t0, t3
        bne   t0, zero, unlocked
        nop
        lbu   v0, 0x400(s0)         # the game's own check, from the instruction the hook replaced
        bne   v0, zero, unlocked
        daddu s1, a1, zero
        j     GAME_CHECK
        nop
unlocked: j   UNLOCKED
        nop
locked: j     LOCKED
        daddu v0, zero, zero
"""

def assemble(src, base):
    lines = []
    for raw in src.strip().splitlines():
        code = raw.split("#")[0].strip()
        label = None
        m = re.match(r"(\w+):\s*(.*)", code)
        if m:
            label, code = m.group(1), m.group(2)
        lines.append((label, code))
    addr = {l: base + 4 * i for i, (l, _) in enumerate(lines) if l}
    consts = {"UNLOCKED": UNLOCKED, "LOCKED": LOCKED, "GAME_CHECK": GAME_CHECK}
    out = []
    for i, (_, code) in enumerate(lines):
        pc = base + 4 * i
        op, _, args = code.partition(" ")
        a = [x.strip() for x in args.split(",")] if args.strip() else []
        imm = lambda s: int(s, 0) & 0xFFFF
        def mem(s):
            off, reg = re.match(r"(-?\w+)\((\w+)\)", s).groups()
            return int(off, 0) & 0xFFFF, R[reg]
        def br(target):
            off = (addr[target] - (pc + 4)) // 4
            assert -0x8000 <= off < 0x8000
            return off & 0xFFFF
        if op == "nop": w = 0
        elif op == "lui": w = 0x0F << 26 | R[a[0]] << 16 | imm(a[1])
        elif op == "ori": w = 0x0D << 26 | R[a[1]] << 21 | R[a[0]] << 16 | imm(a[2])
        elif op == "addiu": w = 0x09 << 26 | R[a[1]] << 21 | R[a[0]] << 16 | imm(a[2])
        elif op == "daddiu": w = 0x19 << 26 | R[a[1]] << 21 | R[a[0]] << 16 | imm(a[2])
        elif op in ("ld", "lw", "lbu"):
            o, b = mem(a[1]); w = {"ld": 0x37, "lw": 0x23, "lbu": 0x24}[op] << 26 | b << 21 | R[a[0]] << 16 | o
        elif op in ("beq", "bne"): w = {"beq": 4, "bne": 5}[op] << 26 | R[a[0]] << 21 | R[a[1]] << 16 | br(a[2])
        elif op == "blez": w = 6 << 26 | R[a[0]] << 21 | br(a[1])
        elif op == "b": w = 4 << 26 | br(a[0])
        elif op == "j": w = 2 << 26 | ((consts[a[0]] if a[0] in consts else addr[a[0]]) >> 2)
        elif op == "or": w = R[a[1]] << 21 | R[a[2]] << 16 | R[a[0]] << 11 | 0x25
        elif op == "daddu": w = R[a[1]] << 21 | R[a[2]] << 16 | R[a[0]] << 11 | 0x2D
        else: raise SystemExit("unknown instruction " + code)
        out.append(w)
    return out

def main():
    words = assemble(ASM, BASE)
    assert BASE + 4 * len(words) <= 0x000FF300, "the routine runs out of its block"
    doc = [l for l in __doc__.split("\n\n")[1].splitlines()]
    block = ["// --- Part 4: the event unlock check (v3, written by tools/build_unlock_check.py) ---"]
    block += ["// " + l.strip() if l.strip() else "//" for l in doc]
    block += [f"patch=1,EE,{BASE + 4 * i:08X},word,{w:08X}" for i, w in enumerate(words)]
    block += [f"patch=1,EE,{HOOK:08X},word,{0x08000000 | BASE >> 2:08X}", f"patch=1,EE,{HOOK + 4:08X},word,00000000"]
    text = open(PNACH).read()
    new, n = re.subn(r"// --- Part 4: the event unlock check.*?patch=1,EE,00134584,word,00000000\n",
                     "\n".join(block) + "\n", text, flags=re.S)
    assert n == 1, "part 4 not found in the .pnach"
    open(PNACH, "w", newline="\n").write(new)
    print(f"wrote the unlock check ({len(words)} words) to", os.path.normpath(PNACH))

if __name__ == "__main__":
    main()
