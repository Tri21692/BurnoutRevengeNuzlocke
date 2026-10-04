"""
ps2scan.py - label-based memory search for PCSX2 (Windows).

Instead of Changed/Unchanged scans one step at a time, you take labelled
snapshots of PS2 RAM ("gold", "silver", ...) and the tool finds bytes that are
the same in every snapshot with the same label but different between labels.

Setup:  pip install pymem pefile numpy
        Keep this file next to pine_probe.py and enable PINE in PCSX2.
Run:    python ps2scan.py

Commands:
  snap <label>     take a snapshot of PS2 RAM under a label
  load <slot>      load PCSX2 save state slot (via PINE)
  save <slot>      save PCSX2 save state slot (via PINE)
  find             candidates: constant within each label, different between ALL labels
  find loose       candidates: constant within each label, not identical across labels
  expect A=49 B=41 candidates: every snapshot of each label holds exactly that (hex) byte value
  relative A=0 B=6 C=10  candidates whose values differ between labels by those (hex) amounts,
                   whatever the starting value is (first label is the reference)
  show [n|all]     list candidates (starting at index n, or all) with each label's value
  max <n>          keep only candidates whose value is <= n in every label (run after find)
  frange <A> <B> <min> <max> [lo hi]  floats that are steady within each label, different between
                   them, and between min and max in both (e.g. frange level behind 0.3 3)
  groups [min] [gap]  after frange: values that repeat at evenly spaced addresses (one per racer?),
                   at least <min> times (default 3), spaced more than <gap> bytes apart (hex, default 40)
  fspeed <stopped> <moving> <mph> [<mph> ...]  floats that are ~0 in every <stopped> snapshot and
                   match the speedometer in each <moving> snapshot (mph per snapshot, in order),
                   stored as mph, km/h or m/s
  fnear <label> <addr> [pct] [lo hi] [chg]  floats that match the float at <addr> (within pct %,
                   default 1) in every <label> snapshot, e.g. other cars' speed at the rolling start.
                   chg: only values that change between snapshots, and show the float 0x10 after each
  fzero <zero> <full> [lo hi]  floats that are exactly 0 in every <zero> snapshot and the same
                   non-zero value in every <full> snapshot (e.g. a boost meter: fzero empty full)
  range <lo> <hi>  keep only candidates between two PS2 addresses (hex), e.g. range 100000 1000000
  clusters         group candidates that sit close together, biggest groups first
  peek <addr> [n]  read n 32-bit words (as hex and float); addr is rounded down to a word
  poke32 <addr> <value>   write a 32-bit hex value
  watch <addr> ... log every change to these 32-bit values until Ctrl+C (game keeps running)
  fwatch <addr> ... [every <sec>]  print these addresses as floats, one line every half second
                   (or every <sec> seconds), until Ctrl+C
  fill <addr> <n> <byte>  set n bytes (hex) to one byte value, e.g. fill 01F65020 40 00
  findval <hex>    search live RAM for a 32-bit value (both byte orders), e.g. findval 289D56A6
  findtext <text>  search live RAM for ASCII text (case-sensitive), e.g. findtext K_01DH1E
  dump <addr> [n]  hex + text view of n bytes (default 128), e.g. dump 00C170C0 256
  events [all]     list career events: index, label, result byte (played only, or all)
  evtable [addr] [n]  World Tour event list (default 00C16504, 0x40 bytes each): one line per event
                   with its label and the unexplained fields, to compare across ranks
  ids <addr> <n>   decode every 8-byte value in n bytes as a packed label; show ones that look real
  findids <addr> <count>  take <count> 8-byte labels starting at <addr> (hex) and find every
                   other place in RAM holding one of them, e.g. findids 56BAE8 4F (the car list)
  findids ... <name>  same, but also remember the hits under <name>
  iddiff <a> <b>   show addresses whose label differs between two remembered findids runs
  idcount <a> <b>  show labels that appear more or fewer times in run b than in run a
  deadset <LABEL> ...  write the dead-car list used by the skip-dead-cars patch
  deadlist         show the dead-car list
  pnach <file>     apply the 'word' patch lines from a .pnach file once, directly to RAM (for testing)
  reconnect        re-attach to PCSX2 and PINE (after restarting the game or PCSX2)
  glyphs <addr> <n>  list unusual (non-ASCII) UTF-16 characters used in game text, with an example each
  rename "OLD" "NEW"  rename game text (e.g. a car name) in the text table; NEW must not be longer
  carname <LABEL>  show the name the game displays for a label, e.g. carname HIGHASCAR1S1
  wreck <LABEL>    mark a car dead: add it to the dead-car list and rename it [WRECKED]
  fallback <LABEL> set the car used instead if a dead car is ever committed (fallback 0 = none)
  deadclear        empty the dead-car list
  savesnaps <file> save all snapshots to disk (e.g. savesnaps results1)
  loadsnaps <file> load snapshots back from disk
  list             show labels and snapshot counts
  drop <label>     delete one label's snapshots
  clear            delete all snapshots
  quit
"""
import struct
import time

import numpy as np
import pefile
import pymem
import pymem.process

from pine_probe import Pine

RAM_SIZE = 0x2000000  # 32 MB of PS2 main RAM
PROCESS_NAMES = ["pcsx2-qt.exe", "pcsx2-qtx64-avx2.exe", "pcsx2-qtx64.exe"]


def attach():
    for name in PROCESS_NAMES:
        try:
            pm = pymem.Pymem(name)
        except pymem.exception.ProcessNotFound:
            continue
        mod = pymem.process.module_from_name(pm.process_handle, name)
        pe = pefile.PE(mod.filename, fast_load=True)
        pe.parse_data_directories(
            directories=[pefile.DIRECTORY_ENTRY["IMAGE_DIRECTORY_ENTRY_EXPORT"]])
        exports = getattr(pe, "DIRECTORY_ENTRY_EXPORT", None)
        for sym in (exports.symbols if exports else []):
            if sym.name == b"EEmem":
                return pm, pm.read_ulonglong(mod.lpBaseOfDll + sym.address)
        raise RuntimeError(f"{name} has no EEmem export. Which PCSX2 version is this?")
    raise RuntimeError("PCSX2 isn't running.")


def find(snaps, strict=True):
    if len(snaps) < 2:
        print("Need snapshots for at least 2 labels.")
        return None
    mask = np.ones(RAM_SIZE, dtype=bool)
    reps = []
    for shots in snaps.values():
        group = np.stack(shots)
        mask &= np.all(group == group[0], axis=0)  # stable within a label
        reps.append(group[0])
    if strict:
        for i in range(len(reps)):
            for j in range(i + 1, len(reps)):
                mask &= reps[i] != reps[j]          # every label differs
    else:
        mask &= ~np.all(np.stack(reps) == reps[0], axis=0)
    hits = np.flatnonzero(mask)
    print(f"{len(hits)} candidates")
    return hits


def show(snaps, hits, start=0, count=40):
    if hits is None:
        print("Run find first.")
        return
    labels = list(snaps)
    print("PS2 addr  " + " ".join(f"{l[:8]:>8}" for l in labels))
    for addr in hits[start:start + count]:
        print(f"{addr:08X}  " + " ".join(f"{snaps[l][0][addr]:>8}" for l in labels))
    if len(hits) > start + count:
        print(f"... type 'show {start + count}' for more")


# Career data (Burnout Revenge PS2, found by tracing the medal/rating setter)
PROGRESS_OBJ = 0x0056AF38          # also holds last medal (+0x1A38) and rating (+0x1A3C)
EVENT_COUNT = PROGRESS_OBJ + 0x2B4  # 169 events
EVENT_IDS = PROGRESS_OBJ + 0x318    # one 8-byte packed label per event
CAREER_OBJ = 0x01F64F08
EVENT_RESULTS = CAREER_OBJ + 0x2D3  # one byte per event, FF = not played
ID_ALPHABET = " -/0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_"


DEAD_TABLE = 0x000FF400           # count (4 bytes), pad, then 8-byte labels
DEAD_MAX = 79
FALLBACK_CAR = 0x000FF3F0          # 8-byte packed label, 0 = none


def encode_label(text):
    text = text.upper().ljust(12)
    if len(text) > 12 or any(c not in ID_ALPHABET for c in text):
        raise ValueError(f"'{text.strip()}' can't be packed (max 12 of A-Z 0-9 _ - /)")
    value = 0
    for c in text:
        value = value * 40 + ID_ALPHABET.index(c)
    return value


def decode_label(value):
    """Criterion-style 64-bit ID: up to 12 characters packed in base 40."""
    chars = []
    for _ in range(12):
        chars.append(ID_ALPHABET[value % 40])
        value //= 40
    return "".join(reversed(chars)).strip()


def ee_write(pm, pine, base, addr, data):
    """Write to PS2 RAM. Direct writes fail on pages PCSX2 has write-protected
    (pages holding translated code), so fall back to PINE, which goes through
    the emulator's own memory system."""
    try:
        pm.write_bytes(base + addr, data, len(data))
        return
    except Exception:
        if pine is None:
            raise
    i = 0
    while i + 4 <= len(data):
        pine.write32(addr + i, struct.unpack_from("<I", data, i)[0])
        i += 4
    while i < len(data):
        pine.write8(addr + i, data[i])
        i += 1


TEXT_LO, TEXT_HI = 0x00670000, 0x006A0000   # the game's text table


def _crc_table():
    table = []
    for i in range(256):
        c = i
        for _ in range(8):
            c = (c >> 1) ^ 0xEDB88320 if c & 1 else c >> 1
        table.append(c)
    return table


_CRC_TABLE = _crc_table()


def text_id(label):
    """The game's own label -> text ID hash (function 002F7050): CRC32 table, start FFFFFFFF,
    but with an arithmetic (sign-keeping) shift and no final inversion."""
    def s32(x):
        x &= 0xFFFFFFFF
        return x - (1 << 32) if x & 0x80000000 else x
    crc = -1
    for b in label.encode("ascii"):
        index = (b ^ crc) & 0xFF
        crc = s32(crc) >> 8
        crc ^= s32(_CRC_TABLE[index])
    return crc & 0xFFFFFFFF


def find_text(pm, base, label):
    """Return (address of the text, current text, room in characters) or None."""
    region = pm.read_bytes(base + TEXT_LO, TEXT_HI - TEXT_LO)
    key = struct.pack("<I", text_id(label))
    spot = region.find(key)
    while spot != -1 and spot % 4:
        spot = region.find(key, spot + 1)
    if spot == -1:
        return None
    start = spot + 4
    end = start
    while region[end:end + 2] != b"\x00\x00":
        end += 2
    text = region[start:end].decode("utf-16-le", errors="replace")
    return TEXT_LO + start, text, len(text)


def wrecked_name(room):
    for option in ("[WRECKED]", "[WRECK]", "[X]"):
        if len(option) <= room:
            return option
    return "X"[:room]


def clusters(hits, gap=0x20):
    if hits is None or len(hits) == 0:
        print("No candidates.")
        return
    groups = []
    start = prev = hits[0]
    n = 1
    for a in hits[1:]:
        if a - prev <= gap:
            n += 1
        else:
            groups.append((start, prev, n))
            start, n = a, 1
        prev = a
    groups.append((start, prev, n))
    for s, e, n in sorted(groups, key=lambda g: -g[2])[:30]:
        print(f"{s:08X}-{e:08X}  {n} byte(s)")
    print(f"{len(groups)} clusters total")


def save_snaps(snaps, path):
    if not path.endswith(".npz"):
        path += ".npz"
    arrays = {f"{label}__{i}": arr for label, shots in snaps.items()
              for i, arr in enumerate(shots)}
    np.savez(path, **arrays)
    print(f"Saved {len(arrays)} snapshot(s) to {path}")


def load_snaps(path):
    if not path.endswith(".npz"):
        path += ".npz"
    snaps = {}
    with np.load(path) as data:
        for key in sorted(data.files, key=lambda k: (k.rsplit("__", 1)[0], int(k.rsplit("__", 1)[1]))):
            snaps.setdefault(key.rsplit("__", 1)[0], []).append(data[key])
    print(f"Loaded {sum(len(v) for v in snaps.values())} snapshot(s) from {path}")
    return snaps


def main():
    pm, base = attach()
    print(f"Attached. PS2 RAM at host address {base:X}")
    try:
        pine = Pine()
    except OSError:
        pine = None
        print("PINE not reachable: load/save commands disabled.")

    snaps, hits = {}, None
    frange_last = []
    idsets = {}
    while True:
        try:
            parts = input("> ").split()
        except EOFError:
            break
        if not parts:
            continue
        cmd, args = parts[0].lower(), parts[1:]
        if cmd in ("quit", "exit"):
            break

        try:  # a bad command prints an error instead of crashing and losing snapshots
            if cmd == "snap" and args:
                data = np.frombuffer(pm.read_bytes(base, RAM_SIZE), dtype=np.uint8).copy()
                snaps.setdefault(args[0], []).append(data)
                print(f"Snapshot {len(snaps[args[0]])} for '{args[0]}'")
            elif cmd in ("load", "save") and args:
                if not pine:
                    print("PINE isn't connected.")
                    continue
                op = 0x0A if cmd == "load" else 0x09
                pine._cmd(struct.pack("<BB", op, int(args[0])))
                print(f"{cmd}ed state slot {args[0]}")
            elif cmd == "find":
                hits = find(snaps, strict=not (args and args[0] == "loose"))
            elif cmd == "expect" and args:
                mask = np.ones(RAM_SIZE, dtype=bool)
                for pair in args:
                    label, _, value = pair.partition("=")
                    if label not in snaps:
                        raise ValueError(f"no snapshots under label '{label}'")
                    v = int(value, 16) & 0xFF
                    for shot in snaps[label]:
                        mask &= shot == v
                hits = np.flatnonzero(mask)
                print(f"{len(hits)} candidates")
            elif cmd == "relative" and len(args) >= 2:
                pairs = []
                for pair in args:
                    label, _, value = pair.partition("=")
                    if label not in snaps:
                        raise ValueError(f"no snapshots under label '{label}'")
                    pairs.append((label, int(value, 16)))
                ref_label, ref_off = pairs[0]
                ref = snaps[ref_label][0]
                mask = np.ones(RAM_SIZE, dtype=bool)
                for label, off in pairs:
                    want = np.uint8((off - ref_off) % 256)
                    for shot in snaps[label]:
                        mask &= (shot - ref) == want   # uint8 maths wraps around 256
                hits = np.flatnonzero(mask)
                print(f"{len(hits)} candidates")
            elif cmd == "show":
                if args and args[0] == "all":
                    show(snaps, hits, 0, count=len(hits) if hits is not None else 0)
                else:
                    show(snaps, hits, int(args[0]) if args else 0)
            elif cmd == "frange" and len(args) >= 4:
                la, lb = args[0], args[1]
                vmin, vmax = float(args[2]), float(args[3])
                if la not in snaps or lb not in snaps:
                    print(f"Need snapshots under '{la}' and '{lb}'.")
                    continue
                fa = [s.view("<f4") for s in snaps[la]]
                fb = [s.view("<f4") for s in snaps[lb]]
                with np.errstate(invalid="ignore"):
                    mask = (fa[0] >= vmin) & (fa[0] <= vmax) & (fb[0] >= vmin) & (fb[0] <= vmax)
                    for x in fa[1:]:
                        mask &= x == fa[0]
                    for x in fb[1:]:
                        mask &= x == fb[0]
                    mask &= fa[0] != fb[0]
                idx = np.flatnonzero(mask)
                if len(args) >= 6:
                    lo, hi = int(args[4], 16), int(args[5], 16)
                    idx = idx[(idx * 4 >= lo) & (idx * 4 < hi)]
                print(f"{len(idx)} candidate(s)")
                frange_last = [(int(i) * 4, float(fa[0][i]), float(fb[0][i])) for i in idx]
                for i in idx[:80]:
                    print(f"  {i * 4:08X}  {la} = {fa[0][i]:<10g} {lb} = {fb[0][i]:g}")
                if len(idx) > 80:
                    print("  ... (add an address range to narrow it down)")
            elif cmd == "groups":
                minimum = int(args[0]) if args else 3
                min_gap = int(args[1], 16) if len(args) > 1 else 0x40
                if not frange_last:
                    print("Run frange first.")
                    continue
                by_values = {}
                for addr, a, b in frange_last:
                    by_values.setdefault((a, b), []).append(addr)
                found = []
                for (a, b), addrs in by_values.items():
                    if len(addrs) < minimum:
                        continue
                    addrs.sort()
                    gaps = [y - x for x, y in zip(addrs, addrs[1:])]
                    common = max(set(gaps), key=gaps.count)
                    even = gaps.count(common) + 1
                    if even >= minimum and common > min_gap:
                        found.append((even, a, b, common, addrs))
                found.sort(key=lambda g: (-g[0], g[3]))
                print(f"{len(found)} group(s)")
                for even, a, b, gap, addrs in found[:25]:
                    shown = " ".join(f"{x:08X}" for x in addrs[:8]) + (" ..." if len(addrs) > 8 else "")
                    print(f"  {even} evenly spaced (every {gap:X}): {a:g} -> {b:g}   {shown}")
            elif cmd == "fspeed" and len(args) >= 3:
                ls, lm = args[0], args[1]
                mph = [float(a) for a in args[2:]]
                if ls not in snaps or lm not in snaps:
                    print(f"Need snapshots under '{ls}' and '{lm}'.")
                    continue
                if len(mph) != len(snaps[lm]):
                    print(f"Give one mph value per '{lm}' snapshot ({len(snaps[lm])} snapshot(s)).")
                    continue
                stopped = [s.view("<f4") for s in snaps[ls]]
                moving = [s.view("<f4") for s in snaps[lm]]
                found = []
                for unit, k in (("mph", 1.0), ("km/h", 1.609344), ("m/s", 0.44704), ("ft/s", 1.46667)):
                    with np.errstate(invalid="ignore"):
                        mask = np.ones(len(moving[0]), dtype=bool)
                        for s in stopped:
                            mask &= np.abs(s) < 0.5 * k
                        for m, v in zip(moving, mph):
                            target = v * k
                            mask &= np.abs(np.abs(m) - target) <= max(1.5 * k, 0.04 * target)
                    for i in np.flatnonzero(mask):
                        found.append((int(i) * 4, unit, [float(m[i]) for m in moving]))
                print(f"{len(found)} candidate(s)")
                for addr, unit, vals in found[:60]:
                    print(f"  {addr:08X}  {unit:<5} " + "  ".join(f"{v:.2f}" for v in vals))
                if len(found) > 60:
                    print("  ... (take one more moving snapshot at a different speed to narrow it down)")
            elif cmd == "fnear" and len(args) >= 2:
                changing = "chg" in args
                args = [a for a in args if a != "chg"]
                label, addr = args[0], int(args[1], 16) & ~3
                pct = float(args[2]) / 100 if len(args) > 2 else 0.01
                if label not in snaps:
                    print(f"No snapshots under '{label}'.")
                    continue
                views = [s.view("<f4") for s in snaps[label]]
                refs = [float(v[addr // 4]) for v in views]
                if any(abs(r) < 1 for r in refs):
                    print(f"The value at {addr:08X} is ~0 in a snapshot ({refs}); is the car moving?")
                    continue
                mask = np.ones(len(views[0]), dtype=bool)
                with np.errstate(invalid="ignore"):
                    for v, r in zip(views, refs):
                        mask &= np.abs(v - r) <= pct * abs(r)
                if changing:
                    if len(views) < 2:
                        print(f"chg needs at least two '{label}' snapshots.")
                        continue
                    moved = np.zeros(len(views[0]), dtype=bool)
                    for v in views[1:]:
                        moved |= v != views[0]
                    mask &= moved
                idx = np.flatnonzero(mask) * 4
                if len(args) >= 5:
                    lo, hi = int(args[3], 16), int(args[4], 16)
                    idx = idx[(idx >= lo) & (idx < hi)]
                print(f"reference {addr:08X} = " + ", ".join(f"{r:.2f}" for r in refs))
                print(f"{len(idx)} match(es)")
                prev = None
                for a in idx[:80]:
                    gap = f"(+{a - prev:X})" if prev is not None else ""
                    extra = ""
                    if changing and (a + 0x10) // 4 < len(views[0]):
                        extra = "   +10: " + "  ".join(f"{float(v[(a + 0x10) // 4]):.2f}" for v in views)
                    print(f"  {a:08X}  " + "  ".join(f"{float(v[a // 4]):.2f}" for v in views) + f"  {gap}{extra}")
                    prev = a
                if len(idx) > 80:
                    print("  ... (add an address range to narrow it down)")
            elif cmd == "fzero" and len(args) >= 2:
                zl, nl = args[0], args[1]
                if zl not in snaps or nl not in snaps:
                    print(f"Need snapshots under '{zl}' and '{nl}'.")
                    continue
                zeros = [s.view("<f4") for s in snaps[zl]]
                fulls = [s.view("<f4") for s in snaps[nl]]
                ref = fulls[0]
                with np.errstate(invalid="ignore"):
                    mask = np.isfinite(ref) & (ref > 0) & (ref < 1e6)
                    for z in zeros:
                        mask &= z == 0
                    for f in fulls[1:]:
                        mask &= f == ref
                idx = np.flatnonzero(mask)
                if len(args) >= 4:
                    lo, hi = int(args[2], 16), int(args[3], 16)
                    idx = idx[(idx * 4 >= lo) & (idx * 4 < hi)]
                print(f"{len(idx)} candidate(s)")
                for i in idx[:60]:
                    print(f"  {i * 4:08X}  {nl} = {ref[i]:g}")
                if len(idx) > 60:
                    print("  ... (add a range to narrow it down)")
            elif cmd == "evtable":
                start = int(args[0], 16) if args else 0x00C16504
                n = int(args[1], 0) if len(args) > 1 else 200
                data = pm.read_bytes(base + start, n * 0x40)
                shown = 0
                print("index  addr      label        +04  +08  +0C   +10(f)   +14      +18(f)     +1C(f)")
                for i in range(n):
                    e = data[i * 0x40:(i + 1) * 0x40]
                    w = struct.unpack("<16I", e)
                    f = struct.unpack("<16f", e)
                    if w[0] != 3:                       # only event entries (type 3)
                        continue
                    ptr = w[0x34 // 4]
                    label = "?"
                    if 0x00100000 <= ptr < RAM_SIZE - 16:
                        raw = pm.read_bytes(base + ptr, 16).split(b"\0")[0]
                        label = raw.decode("ascii", errors="replace")[:12]
                    print(f"{i:5d}  {start + i * 0x40:08X}  {label:<12} {w[1]:4X} {w[2]:4X} {w[3]:4X}"
                          f"  {f[4]:8.4f} {w[5]:8X}  {f[6]:10.3g} {f[7]:9.4f}")
                    shown += 1
                print(f"{shown} event entries")
            elif cmd == "max" and args:
                if hits is None:
                    print("Run find first.")
                    continue
                limit = int(args[0], 0)
                vals = np.stack([shots[0][hits] for shots in snaps.values()])
                hits = hits[np.all(vals <= limit, axis=0)]
                print(f"{len(hits)} candidates")
            elif cmd == "range" and len(args) >= 2:
                if hits is None:
                    print("Run find first.")
                    continue
                lo, hi = int(args[0], 16), int(args[1], 16)
                hits = hits[(hits >= lo) & (hits < hi)]
                print(f"{len(hits)} candidates")
            elif cmd == "clusters":
                clusters(hits)
            elif cmd == "peek" and args:
                addr = int(args[0], 16) & ~3
                count = int(args[1]) if len(args) > 1 else 4
                for i in range(count):
                    a = addr + i * 4
                    if a >= RAM_SIZE:
                        break
                    raw = pm.read_bytes(base + a, 4)
                    u = struct.unpack("<I", raw)[0]
                    f = struct.unpack("<f", raw)[0]
                    print(f"{a:08X}: {u:08X}   float {f:g}")
            elif cmd == "glyphs" and len(args) >= 2:
                start = int(args[0], 16) & ~1
                n = min(int(args[1], 16), RAM_SIZE - start) & ~1
                data = pm.read_bytes(base + start, n)
                units = struct.unpack(f"<{n // 2}H", data)
                seen = {}
                for i, u in enumerate(units):
                    if u <= 0x7E or u in seen:
                        continue
                    # only count it if it sits inside readable text (letters around it)
                    window = units[max(0, i - 6):i + 7]
                    letters = sum(1 for w in window if 0x41 <= w <= 0x5A or 0x61 <= w <= 0x7A)
                    if letters < 3:
                        continue
                    text = "".join(chr(w) if 0x20 <= w <= 0x7E else f"[{w:04X}]" for w in window)
                    seen[u] = (start + i * 2, text)
                print(f"{len(seen)} unusual character(s)")
                for u, (addr, text) in sorted(seen.items()):
                    print(f"  {u:04X}  at {addr:08X}  ...{text}...")
            elif cmd == "rename" and args:
                import shlex
                words = shlex.split(" ".join(args))
                if len(words) != 2:
                    print('Usage: rename "NIXON SPECIAL" "[WRECKED]"')
                    continue
                old_text, new_text = words
                if len(new_text) > len(old_text):
                    print("The new text can't be longer than the old one.")
                    continue
                lo, hi = 0x00670000, 0x006A0000          # the game's text table
                region = pm.read_bytes(base + lo, hi - lo)
                needle = old_text.encode("utf-16-le") + b"\x00\x00"
                spot = region.find(needle)
                hits = []
                while spot != -1:
                    if spot % 2 == 0:
                        hits.append(lo + spot)
                    spot = region.find(needle, spot + 2)
                if not hits:
                    print(f"'{old_text}' not found in the text table (it's case-sensitive).")
                    continue
                replacement = new_text.encode("utf-16-le")
                replacement += b"\x00" * (len(needle) - len(replacement))   # end marker + padding
                for addr in hits:
                    ee_write(pm, pine, base, addr, replacement)
                    print(f"{addr:08X}: '{old_text}' -> '{new_text}'")
            elif cmd == "carname" and args:
                label = args[0].upper()
                found = find_text(pm, base, label)
                if found:
                    addr, text, room = found
                    print(f"{label} -> text ID {text_id(label):08X} at {addr:08X}: '{text}' ({room} characters)")
                else:
                    print(f"No text found for {label} (ID {text_id(label):08X}).")
            elif cmd == "wreck" and args:
                label = args[0].upper()
                # 1. add to the dead-car list (keeping whatever is already on it)
                count = struct.unpack("<I", pm.read_bytes(base + DEAD_TABLE, 4))[0]
                count = count if count <= DEAD_MAX else 0
                raw = pm.read_bytes(base + DEAD_TABLE + 8, count * 8) if count else b""
                values = [struct.unpack_from("<Q", raw, i * 8)[0] for i in range(count)]
                v = encode_label(label)
                if v not in values:
                    values.append(v)
                payload = struct.pack("<II", len(values), 0) + b"".join(struct.pack("<Q", x) for x in values)
                ee_write(pm, pine, base, DEAD_TABLE, payload)
                # 2. rename it
                found = find_text(pm, base, label)
                if not found:
                    print(f"{label} is now unselectable, but its name wasn't found to rename.")
                    continue
                addr, text, room = found
                new_text = wrecked_name(room)
                data = new_text.encode("utf-16-le")
                data += b"\x00" * (room * 2 + 2 - len(data))
                ee_write(pm, pine, base, addr, data)
                print(f"{label} ('{text}') is now '{new_text}' and unselectable. {len(values)} dead car(s).")
            elif cmd == "fallback" and args:
                value = 0 if args[0] == "0" else encode_label(args[0])
                ee_write(pm, pine, base, FALLBACK_CAR, struct.pack("<Q", value))
                print("Fallback car cleared." if value == 0 else f"Fallback car: {args[0].upper()}")
            elif cmd == "reconnect":
                pm, base = attach()
                print(f"Attached. PS2 RAM at host address {base:X}")
                try:
                    pine = Pine()
                    print("PINE connected.")
                except OSError:
                    pine = None
                    print("PINE not reachable: is it enabled in PCSX2's settings?")
            elif cmd == "watch" and args:
                addrs = [int(a, 16) & ~3 for a in args]
                read = lambda a: struct.unpack("<I", pm.read_bytes(base + a, 4))[0]
                last = {a: read(a) for a in addrs}
                for a in addrs:
                    print(f"{a:08X} = {last[a]:08X}")
                print("Watching... press Ctrl+C to stop")
                try:
                    while True:
                        for a in addrs:
                            v = read(a)
                            if v != last[a]:
                                print(f"[{time.strftime('%H:%M:%S')}] {a:08X}: {last[a]:08X} -> {v:08X}")
                                last[a] = v
                        time.sleep(0.01)
                except KeyboardInterrupt:
                    print("Stopped watching.")
            elif cmd == "fwatch" and args:
                interval = 0.5
                if "every" in args:
                    i = args.index("every")
                    interval = max(0.02, float(args[i + 1]))
                    args = args[:i] + args[i + 2:]
                addrs = [int(a, 16) & ~3 for a in args]
                readf = lambda a: struct.unpack("<f", pm.read_bytes(base + a, 4))[0]
                print("time        " + " ".join(f"{a:>9X}" for a in addrs))
                print("Press Ctrl+C to stop")
                try:
                    while True:
                        stamp = time.strftime("%H:%M:%S") + f".{int(time.time() * 10) % 10}"
                        print(stamp + "  " + " ".join(f"{readf(a):9.2f}" for a in addrs))
                        time.sleep(interval)
                except KeyboardInterrupt:
                    print("Stopped watching.")
            elif cmd == "fill" and len(args) >= 3:
                addr, n, value = int(args[0], 16), int(args[1], 16), int(args[2], 16) & 0xFF
                if not 0 < n <= 0x1000 or addr + n > RAM_SIZE:
                    print("Refusing: n must be 1..1000 (hex) and stay inside PS2 RAM.")
                    continue
                ee_write(pm, pine, base, addr, bytes([value]) * n)
                print(f"{addr:08X}..{addr + n - 1:08X} <- {value:02X}")
            elif cmd == "poke32" and len(args) >= 2:
                addr = int(args[0], 16) & ~3
                ee_write(pm, pine, base, addr, struct.pack("<I", int(args[1], 16)))
                print(f"{addr:08X} <- {int(args[1], 16):08X}")
            elif cmd == "findval" and args:
                val = int(args[0], 16) & 0xFFFFFFFF
                words = np.frombuffer(pm.read_bytes(base, RAM_SIZE), dtype="<u4")
                swapped = int.from_bytes(val.to_bytes(4, "little"), "big")
                for label, v in (("as-is", val), ("byte-swapped", swapped)):
                    found = np.flatnonzero(words == v) * 4
                    print(f"{label} {v:08X}: {len(found)} hit(s)")
                    for a in found[:40]:
                        print(f"  {a:08X}")
                    if len(found) > 40:
                        print("  ...")
            elif cmd == "findtext" and args:
                needle = " ".join(args).encode("ascii")
                data = pm.read_bytes(base, RAM_SIZE)
                found, i = [], data.find(needle)
                while i != -1 and len(found) < 40:
                    found.append(i)
                    i = data.find(needle, i + 1)
                print(f"'{needle.decode()}': {len(found)} hit(s){' (first 40)' if len(found) == 40 else ''}")
                for a in found:
                    context = data[max(0, a - 8):a + len(needle) + 8]
                    shown = "".join(chr(c) if 32 <= c < 127 else "." for c in context)
                    print(f"  {a:08X}  {shown}")
            elif cmd == "dump" and args:
                addr = int(args[0], 16) & ~0xF
                n = int(args[1], 0) if len(args) > 1 else 128
                n = min(n, RAM_SIZE - addr)
                data = pm.read_bytes(base + addr, n)
                for off in range(0, n, 16):
                    row = data[off:off + 16]
                    hexpart = " ".join(f"{b:02X}" for b in row)
                    text = "".join(chr(b) if 32 <= b < 127 else "." for b in row)
                    print(f"{addr + off:08X}  {hexpart:<47}  {text}")
            elif cmd == "events":
                count = struct.unpack("<I", pm.read_bytes(base + EVENT_COUNT, 4))[0]
                if not 0 < count <= 512:
                    print(f"Event count looks wrong ({count}); is the World Tour loaded?")
                    continue
                ids = pm.read_bytes(base + EVENT_IDS, count * 8)
                results = pm.read_bytes(base + EVENT_RESULTS, count)
                show_all = bool(args and args[0] == "all")
                for i in range(count):
                    label = decode_label(struct.unpack_from("<Q", ids, i * 8)[0])
                    r = results[i]
                    if show_all or r != 0xFF:
                        status = "not played" if r == 0xFF else f"{r:02X}"
                        print(f"{i:3d}  {label:<12}  {status}")
            elif cmd == "ids" and len(args) >= 2:
                start = int(args[0], 16) & ~7
                n = min(int(args[1], 16), RAM_SIZE - start) & ~7
                data = pm.read_bytes(base + start, n)
                shown = 0
                for off in range(0, n, 8):
                    v = struct.unpack_from("<Q", data, off)[0]
                    if v == 0 or v >= 40 ** 12:
                        continue
                    label = decode_label(v)
                    # looks real: 4+ chars, starts with a letter, no gaps, only A-Z 0-9 _
                    if (len(label) >= 4 and label[0].isalpha() and " " not in label
                            and all(c.isalnum() or c == "_" for c in label)):
                        print(f"{start + off:08X}  {label}")
                        shown += 1
                print(f"{shown} label(s)")
            elif cmd == "findids" and len(args) >= 2:
                list_addr = int(args[0], 16) & ~7
                count = int(args[1], 16)
                wanted = np.frombuffer(pm.read_bytes(base + list_addr, count * 8), dtype="<u8")
                ram = pm.read_bytes(base, RAM_SIZE)
                found = []
                for shift in (0, 4):  # 8-byte aligned, and 4-byte aligned just in case
                    words = np.frombuffer(ram[shift:RAM_SIZE - 8 + shift], dtype="<u8")
                    for idx in np.flatnonzero(np.isin(words, wanted)):
                        addr = idx * 8 + shift
                        if not list_addr <= addr < list_addr + count * 8:
                            found.append((addr, int(words[idx])))
                found.sort()
                if len(args) >= 3:
                    idsets[args[2]] = {a: v for a, v in found}
                    print(f"Remembered as '{args[2]}'")
                print(f"{len(found)} hit(s) outside the list")
                for addr, v in found[:80]:
                    pos = int(np.flatnonzero(wanted == v)[0])
                    print(f"  {addr:08X}  {decode_label(v):<12}  (list index {pos})")
                if len(found) > 80:
                    print("  ...")
            elif cmd == "idcount" and len(args) >= 2:
                a, b = idsets.get(args[0]), idsets.get(args[1])
                if a is None or b is None:
                    print("Run findids with those names first.")
                    continue
                from collections import Counter
                ca, cb = Counter(a.values()), Counter(b.values())
                changed = sorted(set(ca) | set(cb), key=lambda v: cb[v] - ca[v])
                changed = [v for v in changed if ca[v] != cb[v]]
                print(f"{len(changed)} label(s) changed count")
                for v in changed:
                    print(f"  {decode_label(v):<12}  {ca[v]} -> {cb[v]}")
            elif cmd == "iddiff" and len(args) >= 2:
                a, b = idsets.get(args[0]), idsets.get(args[1])
                if a is None or b is None:
                    print("Run findids with those names first.")
                    continue
                diffs = sorted(addr for addr in set(a) | set(b) if a.get(addr) != b.get(addr))
                print(f"{len(diffs)} address(es) differ")
                for addr in diffs[:80]:
                    la = decode_label(a[addr]) if addr in a else "-"
                    lb = decode_label(b[addr]) if addr in b else "-"
                    print(f"  {addr:08X}  {la:<12} -> {lb}")
                if len(diffs) > 80:
                    print("  ...")
            elif cmd in ("deadset", "deadclear"):
                labels = [] if cmd == "deadclear" else args
                if len(labels) > DEAD_MAX:
                    print(f"At most {DEAD_MAX} cars.")
                    continue
                values = [encode_label(l) for l in labels]
                payload = struct.pack("<II", len(values), 0)
                payload += b"".join(struct.pack("<Q", v) for v in values)
                ee_write(pm, pine, base, DEAD_TABLE, payload)
                print(f"Dead-car list: {len(values)} car(s)")
            elif cmd == "pnach" and args:
                path = " ".join(args)
                writes = []
                with open(path, encoding="utf-8", errors="replace") as f:
                    for line in f:
                        line = line.split("//")[0].strip()
                        if not line.lower().startswith("patch="):
                            continue
                        parts = [p.strip() for p in line[6:].split(",")]
                        if len(parts) == 5 and parts[1].upper() == "EE" and parts[3].lower() == "word":
                            writes.append((int(parts[2], 16), int(parts[4], 16)))
                # write the hook (jump into the wrapper) last, after the wrapper exists
                writes.sort(key=lambda w: w[0] >= 0x00100000)
                for addr, value in writes:
                    ee_write(pm, pine, base, addr, struct.pack("<I", value))
                print(f"Applied {len(writes)} word patch(es) from {path}")
            elif cmd == "deadlist":
                count = struct.unpack("<I", pm.read_bytes(base + DEAD_TABLE, 4))[0]
                if count > DEAD_MAX:
                    print(f"Count is {count}; the list hasn't been set up.")
                    continue
                raw = pm.read_bytes(base + DEAD_TABLE + 8, count * 8) if count else b""
                print(f"{count} dead car(s)")
                for i in range(count):
                    print("  " + decode_label(struct.unpack_from("<Q", raw, i * 8)[0]))
            elif cmd == "savesnaps" and args:
                save_snaps(snaps, args[0])
            elif cmd == "loadsnaps" and args:
                snaps, hits = load_snaps(args[0]), None
            elif cmd == "list":
                for label, shots in snaps.items():
                    print(f"{label}: {len(shots)} snapshot(s)")
            elif cmd == "drop" and args:
                snaps.pop(args[0], None)
                hits = None
            elif cmd == "clear":
                snaps.clear()
                hits = None
            else:
                print(__doc__)
        except Exception as e:
            print(f"Error: {e}  (your snapshots are still there)")
            if isinstance(e, (ConnectionError, OSError)):
                print("Looks like the connection to PCSX2 dropped. Type 'reconnect' and try again.")


if __name__ == "__main__":
    main()
