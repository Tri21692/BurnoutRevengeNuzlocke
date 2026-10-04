"""
pine_probe.py - minimal PCSX2 PINE client for poking at Burnout Revenge (PS2).

Enable PINE in PCSX2: Settings > Advanced > PINE (default slot 28011).

Usage:
  python pine_probe.py info                      # game title / serial / version
  python pine_probe.py peek 0x00123450 [count]   # read 32-bit words
  python pine_probe.py watch 0x00123450 0x...    # log whenever any address changes
  python pine_probe.py poke32 0x00123450 0x1     # write a 32-bit value

Addresses are PS2 EE addresses (0x00000000-0x01FFFFFF).
"""
import os
import socket
import struct
import sys
import time

OP_READ8, OP_READ32 = 0x00, 0x02
OP_WRITE8, OP_WRITE32 = 0x04, 0x06
OP_TITLE, OP_ID, OP_GAMEVER = 0x0B, 0x0C, 0x0E


class Pine:
    def __init__(self, slot=28011):
        if sys.platform == "win32":
            self.s = socket.create_connection(("127.0.0.1", slot))
        else:
            base = os.environ.get("XDG_RUNTIME_DIR") or os.environ.get("TMPDIR") or "/tmp"
            name = "pcsx2.sock" if slot == 28011 else f"pcsx2.sock.{slot}"
            self.s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            self.s.connect(os.path.join(base, name))

    def _recv(self, n):
        buf = b""
        while len(buf) < n:
            chunk = self.s.recv(n - len(buf))
            if not chunk:
                raise ConnectionError("PCSX2 closed the connection")
            buf += chunk
        return buf

    def _cmd(self, payload):
        # Message = u32 total size (incl. this field) + payload, little-endian.
        self.s.sendall(struct.pack("<I", len(payload) + 4) + payload)
        size = struct.unpack("<I", self._recv(4))[0]
        body = self._recv(size - 4)
        if body[0] != 0x00:
            raise RuntimeError("PINE command failed (is a game running?)")
        return body[1:]

    def read8(self, addr):
        return self._cmd(struct.pack("<BI", OP_READ8, addr))[0]

    def read32(self, addr):
        return struct.unpack("<I", self._cmd(struct.pack("<BI", OP_READ32, addr)))[0]

    def write8(self, addr, val):
        self._cmd(struct.pack("<BIB", OP_WRITE8, addr, val & 0xFF))

    def write32(self, addr, val):
        self._cmd(struct.pack("<BII", OP_WRITE32, addr, val & 0xFFFFFFFF))

    def _string(self, op):
        data = self._cmd(struct.pack("<B", op))
        n = struct.unpack("<I", data[:4])[0]
        return data[4:4 + n].rstrip(b"\0").decode(errors="replace")

    def title(self):   return self._string(OP_TITLE)
    def serial(self):  return self._string(OP_ID)
    def version(self): return self._string(OP_GAMEVER)


def main(argv):
    if not argv:
        print(__doc__)
        return
    p = Pine()
    cmd, args = argv[0], [int(a, 0) for a in argv[1:]]

    if cmd == "info":
        print(f"Title:   {p.title()}\nSerial:  {p.serial()}\nVersion: {p.version()}")
    elif cmd == "peek":
        addr, count = args[0], (args[1] if len(args) > 1 else 4)
        for i in range(count):
            a = addr + i * 4
            print(f"{a:08X}: {p.read32(a):08X}")
    elif cmd == "poke32":
        p.write32(args[0], args[1])
        print(f"{args[0]:08X} <- {args[1]:08X}")
    elif cmd == "watch":
        last = {a: p.read32(a) for a in args}
        print("Watching... Ctrl+C to stop")
        while True:
            for a in args:
                v = p.read32(a)
                if v != last[a]:
                    print(f"[{time.strftime('%H:%M:%S')}] {a:08X}: {last[a]:08X} -> {v:08X}")
                    last[a] = v
            time.sleep(0.05)
    else:
        print(__doc__)


if __name__ == "__main__":
    main(sys.argv[1:])
