"""Builds the "run's dead" results-screen lock used by the .pnach (part 4 of the main group).

Each results screen handler starts with a jump to a stub. When the tracker has marked the run dead
(word at 000FE120 = 1), the stub returns straight away for the screen's Continue/Retry messages, as
if the button wasn't pressed: the screen stays put, nothing half-starts, and it works again once the
flag is cleared (Grace or a new run). Otherwise it runs the handler's first two instructions and
jumps back in.

Run: python tools/build_deadlock.py  (needs: pip install rabbitizer) to print the code.
"""
import os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from build_catchup import Asm, R0, AT, SP, RA

DEAD_FLAG = 0x000FE120
# handler, its first two instructions, the messages to refuse while the run is dead, stub address
HANDLERS = [
    (0x0019AD98, (0x27BDFFC0, 0x24020007), (7, 8), 0x000FF200),  # race results: 7 continue, 8 retry
    (0x00191B58, (0x27BDFF80, 0x24030003), (5,), 0x000FF240),    # end-of-event screen: 5 = selection
    (0x00191FA8, (0x27BDFFC0, 0x24030003), (5,), 0x000FF280),    # end-of-event screen: 5 = selection
]
A1 = 5

def stub(handler, first, messages, base):
    a = Asm(base)
    a.lui(AT, (DEAD_FLAG + 0x8000) >> 16)
    a.lw(AT, DEAD_FLAG & 0xFFFF, AT)            # run dead?
    a.br("beq", AT, R0, "go"); a.nop()
    for m in messages:
        a.addiu(AT, R0, m)
        a.br("beq", A1, AT, "refuse"); a.nop()
    a.L("go")
    a.e(first[0]); a.e(first[1])                # the handler's own first two instructions
    a.e(0x08000000 | ((handler + 8) >> 2)); a.nop()  # j handler+8
    a.L("refuse")
    a.e(0x03E00008); a.nop()                    # jr ra: as if nothing was pressed
    words = a.words()
    assert len(words) * 4 <= 0x40, "stub too big"
    return words

def patches():
    out = []
    for handler, first, messages, base in HANDLERS:
        out += [(base + 4 * i, w) for i, w in enumerate(stub(handler, first, messages, base))]
        out += [(handler, 0x08000000 | (base >> 2)), (handler + 4, 0)]  # j stub; nop
    return out

if __name__ == "__main__":
    import rabbitizer as R, re
    for a, w in patches():
        i = R.Instruction(w, vram=a, category=R.InstrCategory.R5900); t = i.disassemble()
        if i.isBranch(): t = re.sub(r"\. \+ 4 \+ \(.*\)$", f"{i.getBranchVramGeneric():08X}", t)
        if i.isJumpWithAddress(): t = re.sub(r"func_\w+", f"{i.getInstrIndexAsVram():08X}", t)
        print(f"{a:08X} {w:08X} {t}")
