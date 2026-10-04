"""Builds the catch-up wrapper (000FFA00) used by the Harder AI .pnach groups.

Run: python tools/build_catchup.py  (needs: pip install rabbitizer) to print the code with a disassembly.
"""
import struct, rabbitizer as R
R0,AT,V0,V1,A0,T0,T1,T2,T3,S0,S1,SP,RA=0,1,2,3,4,8,9,10,11,16,17,29,31
BASE=0x000FFA00
def fbits(x): return struct.unpack(">I",struct.pack(">f",x))[0]
class Asm:
    def __init__(s,base): s.base=base; s.ins=[]; s.labels={}
    def pc(s): return s.base+4*len(s.ins)
    def L(s,n): s.labels[n]=s.pc()
    def e(s,x): s.ins.append(x)
    def i(s,op,rs,rt,imm): s.e((op<<26)|(rs<<21)|(rt<<16)|(imm&0xFFFF))
    def addiu(s,rt,rs,imm): s.i(0x09,rs,rt,imm)
    def lui(s,rt,imm): s.i(0x0F,0,rt,imm)
    def ori(s,rt,rs,imm): s.i(0x0D,rs,rt,imm)
    def lw(s,rt,off,b): s.i(0x23,b,rt,off)
    def sd(s,rt,off,b): s.i(0x3F,b,rt,off)
    def ld(s,rt,off,b): s.i(0x37,b,rt,off)
    def sq(s,rt,off,b): s.i(0x1F,b,rt,off)
    def lq(s,rt,off,b): s.i(0x1E,b,rt,off)
    def lwc1(s,ft,off,b): s.i(0x31,b,ft,off)
    def swc1(s,ft,off,b): s.i(0x39,b,ft,off)
    def daddu(s,rd,rs,rt): s.e((rs<<21)|(rt<<16)|(rd<<11)|0x2D)
    def jal(s,t): s.e((3<<26)|((t>>2)&0x3FFFFFF))
    def jr_ra(s): s.e(0x03E00008)
    def nop(s): s.e(0)
    def br(s,kind,rs,rt,lab): s.ins.append(("br",kind,rs,rt,lab,s.pc()))
    def bc1(s,t,lab): s.ins.append(("bc1",t,lab,s.pc()))
    def mtc1(s,rt,fs): s.e(0x44800000|(rt<<16)|(fs<<11))
    def fop(s,funct,fd,fs,ft=0): s.e(0x46000000|(ft<<16)|(fs<<11)|(fd<<6)|funct)
    def add_s(s,fd,fs,ft): s.fop(0,fd,fs,ft)
    def sub_s(s,fd,fs,ft): s.fop(1,fd,fs,ft)
    def mul_s(s,fd,fs,ft): s.fop(2,fd,fs,ft)
    def mov_s(s,fd,fs): s.fop(6,fd,fs)
    def clt(s,fs,ft): s.fop(0x34,0,fs,ft)
    def lif(s,freg,val):  # load float constant via at
        b=fbits(val); s.lui(AT,b>>16)
        if b&0xFFFF: s.ori(AT,AT,b&0xFFFF)
        s.mtc1(AT,freg); s.nop()
    def words(s):
        out=[]
        for x in s.ins:
            if isinstance(x,tuple):
                if x[0]=="br":
                    _,kind,rs,rt,lab,pc=x; off=(s.labels[lab]-(pc+4))>>2
                    out.append(({"beq":4,"bne":5}[kind]<<26)|(rs<<21)|(rt<<16)|(off&0xFFFF))
                else:
                    _,t,lab,pc=x; off=(s.labels[lab]-(pc+4))>>2
                    out.append(0x45000000|(t<<16)|(off&0xFFFF))
            else: out.append(x)
        return out

def build(d0,k,bmax):
    a=Asm(BASE)
    a.addiu(SP,SP,-0x40); a.sd(RA,0,SP); a.sq(S0,0x10,SP); a.sq(S1,0x20,SP)
    a.swc1(20,0x30,SP); a.swc1(21,0x34,SP)
    a.jal(0x002998E0); a.daddu(S0,A0,R0)          # original call, s0 = brain
    a.lui(T0,0x01EE); a.lw(T1,-0x7FA8,T0)         # [01ED8058] player present
    a.br("beq",T1,R0,"done"); a.nop()
    a.lw(S1,0x7E0,S0)                             # own racer
    a.lui(T0,0x01EE); a.addiu(T0,T0,-0x7F90)      # 01ED8070 player racer
    a.br("beq",S1,T0,"done"); a.nop()
    a.lw(T2,0x2500,S1)                            # race running
    a.br("beq",T2,R0,"done"); a.nop()
    a.jal(0x002C78C0); a.addiu(A0,S1,0x24E0)      # own progress
    a.mov_s(20,0)
    a.lui(A0,0x01EE); a.jal(0x002C78C0); a.addiu(A0,A0,-0x5AB0)  # player progress (01EDA550)
    a.sub_s(21,0,20)                              # gap = player - own (+ = AI behind)
    a.lif(1,d0); a.sub_s(21,21,1)                 # d = gap - start distance
    a.mtc1(R0,1); a.nop(); a.clt(1,21); a.nop()   # 0 < d ?
    a.bc1(1,"behind"); a.nop()
    a.mov_s(21,1)                                 # not far enough behind: boost = 0
    a.br("beq",R0,R0,"top"); a.nop()
    a.L("behind")
    a.lif(1,k); a.mul_s(21,21,1)                  # boost = k * d
    a.lif(1,bmax); a.clt(1,21); a.nop()           # bmax < boost ?
    a.bc1(0,"cap_ok"); a.nop()
    a.mov_s(21,1)
    a.L("cap_ok")
    a.lui(T0,0x01EE); a.lw(T1,-0x4810,T0)         # player car = [01EDB7F0]
    a.lwc1(2,0xAC,T1)                             # player speed
    a.add_s(2,2,21)                               # player speed + boost
    a.lwc1(3,0xA04,S0)                            # AI target speed
    a.clt(3,2); a.nop()
    a.bc1(0,"top"); a.nop()
    a.swc1(2,0xA04,S0)                            # raise target
    a.L("top")
    a.lw(T2,0x3780,S1); a.lw(T3,0x1384,T2)        # own car, its parameters
    a.lwc1(4,0x1C0,T3)                            # stock top speed (mph)
    a.lif(5,2.2369); a.mul_s(5,21,5); a.add_s(4,4,5)
    a.swc1(4,0x1354,T2)                           # top speed = stock + boost in mph
    a.L("done")
    a.ld(RA,0,SP); a.lq(S0,0x10,SP); a.lq(S1,0x20,SP)
    a.lwc1(20,0x30,SP); a.lwc1(21,0x34,SP)
    a.jr_ra(); a.addiu(SP,SP,0x40)
    return a.words()

if __name__=="__main__":
    ws=build(30,0.25,25)
    for n,w in enumerate(ws):
        x=BASE+4*n; ins=R.Instruction(w,vram=x,category=R.InstrCategory.R5900)
        t=ins.disassemble()
        if ins.isBranch(): t+=f"   -> {ins.getBranchVramGeneric():08X}"
        print(f"{x:08X} {w:08X} {t}")
    print(len(ws),"words, ends",hex(BASE+4*len(ws)))
