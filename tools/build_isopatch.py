"""Generates isopatch/patches.go from the .pnach, for the ISO patcher.

The .pnach keeps the mod's code below the game (000FF000-000FFFFF), which isn't part of the game file.
In the patched ISO each block of that code moves into unused space inside the game's own .data
section: the end of a 40 KB block of zeros (00470258-0047A25F) that nothing in the game refers to
and that is still all zeros mid-race. The file's layout (headers, sections) is left alone. Each block
is moved as a whole (the code is position independent), and every j/jal into the old area is
retargeted. The .pnach's difficulty marker (000FE110) is replaced by a level word in the same space.

Run: python tools/build_isopatch.py
"""
import re, sys, os

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PNACH = os.path.join(ROOT, "patches", "SLUS-21242_D224D348.pnach")
OUT = os.path.join(ROOT, "isopatch", "patches.go")

# (old start, old end, new start): the code blocks of the .pnach and where they go in the game file
BLOCKS = [
    (0x000FF000, 0x000FF100, 0x00479D00),  # dead-car garage block (0x8C bytes)
    (0x000FF100, 0x000FF140, 0x00479DA0),  # finished-event signal (0x24)
    (0x000FF140, 0x000FF200, 0x00479DD0),  # crash junction block (0x88)
    (0x000FF200, 0x000FF300, 0x00479C00),  # run's dead results-screen lock (3 stubs, 0x40 each)
    (0x000FFA00, 0x000FFC00, 0x00479800),  # AI catch-up wrapper (about 0x1B8; before V1.1.2 at 00479E80)
]
FREE_LO, FREE_HI = 0x00479800, 0x0047A000  # the space used; checked to be zero by the patcher
LEVEL_ADDR = 0x00479FF0
MARKER = 0x000FE110

# Widescreen values that live in .bss (not in the file) are rewritten every frame, as PCSX2 does for
# the .pnach: the main loop's call to 00185E60 at 001044E0 goes through a small writer routine.
WRITER_ADDR = 0x00479B00
FRAME_CALL, FRAME_CALL_TARGET = 0x001044E0, 0x00185E60
# The Aggressive AI settings are loaded from the game's data at run time, so they are rewritten every
# frame too, by a second writer on the main loop's call to 0034A688 at 0010454C.
AGGR_WRITER = 0x00479E60
AGGR_CALL, AGGR_CALL_TARGET = 0x0010454C, 0x0034A688
FILE_RANGES = [(0x00100000, 0x004A34F8), (0x004A7500, 0x004EFBCC)]  # loaded from the file

def in_file(a):
    return any(lo <= a < hi for lo, hi in FILE_RANGES)

def writer(words):
    """lui at,hi / lui v1,val / ori v1,lo / sw v1,lo(at) per word, then j 00185E60 (at and v1 only)."""
    code = []
    for a, v in words:
        hi = ((a + 0x8000) >> 16) & 0xFFFF
        code.append(0x3C010000 | hi)                       # lui at, hi
        code.append(0x3C030000 | (v >> 16))                # lui v1, value hi
        if v & 0xFFFF:
            code.append(0x34630000 | (v & 0xFFFF))         # ori v1, v1, value lo
        code.append(0xAC230000 | (a & 0xFFFF))             # sw v1, lo(at)
    code.append(0x08000000 | (FRAME_CALL_TARGET >> 2))     # j 00185E60 (ra still points at the caller)
    code.append(0)                                          # nop
    return [(WRITER_ADDR + 4 * i, w) for i, w in enumerate(code)]

def option_patches(g, name):
    direct, runtime = {}, []
    for a, w in g[name]:
        (direct.__setitem__(a, w) if in_file(a) else runtime.append((a, w)))
    if runtime:
        for a, w in writer(runtime):
            direct[a] = w
        assert WRITER_ADDR + 4 * len(writer(runtime)) <= 0x00479C00, "writer runs into the mod code"
        direct[FRAME_CALL] = 0x0C000000 | (WRITER_ADDR >> 2)  # jal writer (delay slot unchanged)
    return sorted(direct.items())

def aggr_writer(words):
    """Like writer(), sharing lui at between neighbouring addresses; ends with j 0034A688."""
    code, hi_at = [], None
    for a, v in sorted(words):
        hi = ((a + 0x8000) >> 16) & 0xFFFF
        if hi != hi_at:
            code.append(0x3C010000 | hi); hi_at = hi              # lui at, hi
        if v == 0:
            code.append(0xAC200000 | (a & 0xFFFF))               # sw zero, lo(at)
            continue
        code.append(0x3C030000 | (v >> 16))                      # lui v1, value hi
        if v & 0xFFFF:
            code.append(0x34630000 | (v & 0xFFFF))               # ori v1, v1, value lo
        code.append(0xAC230000 | (a & 0xFFFF))                   # sw v1, lo(at)
    code.append(0x08000000 | (AGGR_CALL_TARGET >> 2))            # j 0034A688
    code.append(0)
    out = [(AGGR_WRITER + 4 * i, w) for i, w in enumerate(code)]
    assert AGGR_WRITER + 4 * len(code) <= LEVEL_ADDR, "aggression writer runs into the level word"
    return out

def aggr_patches(g, level_name):
    words = dict(aggr_writer(g["Nuzlocke\\Aggressive AI\\" + level_name]))
    words[AGGR_CALL] = 0x0C000000 | (AGGR_WRITER >> 2)          # jal writer (delay slot is a nop)
    return sorted(words.items())

def reloc_addr(a):
    for lo, hi, new in BLOCKS:
        if lo <= a < hi:
            return new + (a - lo)
    return a

def reloc_word(w):
    op = w >> 26
    if op in (2, 3):  # j / jal
        target = (w & 0x3FFFFFF) << 2
        new = reloc_addr(target)
        if new != target:
            return (op << 26) | (new >> 2)
    return w

def groups(text):
    out, name = {}, None
    for line in text.splitlines():
        m = re.match(r"\[(.+)\]", line.strip())
        if m:
            name = m.group(1); out[name] = []
            continue
        m = re.match(r"patch=1,EE,([0-9A-Fa-f]{8}),word,([0-9A-Fa-f]{8})", line.strip())
        if m and name:
            out[name].append((int(m.group(1), 16), int(m.group(2), 16)))
    return out

def level_patches(g, level_name, level_num):
    words = {}
    for name in ("Nuzlocke\\Block dead cars in garage", "Nuzlocke\\Block pause-menu Retry and Quit",
                 "Nuzlocke\\Harder AI\\" + level_name):
        for a, w in g[name]:
            if a == MARKER:
                continue
            na, nw = reloc_addr(a), reloc_word(w)
            if na < 0x00100000:
                sys.exit(f"{name}: {a:08X} is below the game and not in a relocated block")
            if FREE_LO <= na < FREE_HI and na >= LEVEL_ADDR:
                sys.exit(f"{name}: {a:08X} overflows the free space")
            words[na] = nw
    # blocks must not run into each other
    used = sorted(a for a in words if FREE_LO <= a < FREE_HI)
    starts = sorted([b[2] for b in BLOCKS] + [WRITER_ADDR, AGGR_WRITER, LEVEL_ADDR])
    for lo, hi, new in BLOCKS:
        nxt = min(a for a in starts if a > new)
        top = max([a for a in used if new <= a < nxt] or [new])
        assert top + 4 <= nxt, f"block at {new:08X} runs into {nxt:08X}"
    words[LEVEL_ADDR] = level_num
    return sorted(words.items())

def main():
    g = groups(open(PNACH).read())
    lines = ["// Code generated by tools/build_isopatch.py from patches/SLUS-21242_D224D348.pnach. DO NOT EDIT.", "",
             "package main", "",
             f"const freeLo, freeHi = 0x{FREE_LO:08X}, 0x{FREE_HI:08X} // must be zero in the original file",
             f"const levelAddr = 0x{LEVEL_ADDR:08X}", "",
             "type word struct{ addr, value uint32 }", "",
             "var levels = []struct {", "\tname  string", "\twords []word", "}{"]
    for num, name in enumerate(("Easy", "Medium", "Hard"), 1):
        lines.append(f'\t{{"{name}", []word{{')
        for a, w in level_patches(g, name, num):
            lines.append(f"\t\t{{0x{a:08X}, 0x{w:08X}}},")
        lines.append("\t}},")
    lines.append("}")
    lines += ["", "var options = []struct {", "\tkey, name string", "\twords     []word", "}{"]
    for key, name in (("widescreen", "Widescreen 16:9"), ("fps60", "60 FPS menus and crash mode")):
        lines.append(f'\t{{"{key}", "{name}", []word{{')
        for a, w in option_patches(g, "Nuzlocke\\" + name):
            lines.append(f"\t\t{{0x{a:08X}, 0x{w:08X}}},")
        lines.append("\t}},")
    lines.append("}")
    lines += ["", "var aggression = []struct {", "\tname  string", "\twords []word", "}{"]
    for name in ("Easy", "Medium", "Hard"):
        lines.append(f'\t{{"{name}", []word{{')
        for a, w in aggr_patches(g, name):
            lines.append(f"\t\t{{0x{a:08X}, 0x{w:08X}}},")
        lines.append("\t}},")
    lines.append("}")
    open(OUT, "w").write("\n".join(lines) + "\n")
    print("wrote", OUT)

if __name__ == "__main__":
    main()
