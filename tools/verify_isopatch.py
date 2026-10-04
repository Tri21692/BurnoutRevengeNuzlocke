"""Checks ISOs made by nuzlocke_isopatch against the .pnach.

Loads the patched SLUS_212.42 by its program headers, like the PS2 does, and compares the memory image
with the original file's image plus the .pnach's changes (relocated). Needs: pip install pycdlib

Run: python tools/verify_isopatch.py <original SLUS_212.42> <patched ISO> <Easy|Medium|Hard>
"""
import io, struct, sys, os
import pycdlib
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import build_isopatch as B

def load(elf):
    phoff, = struct.unpack_from("<I", elf, 0x1C); n, = struct.unpack_from("<H", elf, 0x2C)
    mem = bytearray(0x2000000)
    for i in range(n):
        t, off, va, pa, fs, ms, fl, al = struct.unpack_from("<8I", elf, phoff + 32 * i)
        if t == 1:
            mem[va:va + fs] = elf[off:off + fs]
    return mem

def main(orig_path, iso_path, name):
    orig = open(orig_path, "rb").read()
    iso = pycdlib.PyCdlib(); iso.open(iso_path); buf = io.BytesIO()
    iso.get_file_from_iso_fp(buf, iso_path="/SLUS_212.42;1"); iso.close()
    elf = buf.getvalue()
    num = ("Easy", "Medium", "Hard").index(name) + 1
    exp = load(orig)
    g = B.groups(open(B.PNACH).read())
    for grp in ("Nuzlocke\\Block dead cars in garage", "Nuzlocke\\Block pause-menu Retry and Quit",
                "Nuzlocke\\Harder AI\\" + name):
        for a, w in g[grp]:
            if a != B.MARKER:
                struct.pack_into("<I", exp, B.reloc_addr(a), B.reloc_word(w))
    struct.pack_into("<I", exp, B.LEVEL_ADDR, num)
    mem = load(elf)
    bad = [i for i in range(0, len(mem), 4) if mem[i:i + 4] != exp[i:i + 4]]
    print(f"{name}: {len(bad)} mismatching words" + ("" if bad else " - OK"))
    return not bad

if __name__ == "__main__":
    sys.exit(0 if main(*sys.argv[1:4]) else 1)
