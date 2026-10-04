"""
Burnout Revenge (PS2, SLUS-21242) - Nuzlocke tracker

Runs next to PCSX2 and enforces the Nuzlocke rules:
  * every car has lives (Easy 3, Medium 2, Hard 1), chosen once per run
  * an event only counts as a win with a first-time Gold + Perfect; anything else costs
    the car you used a life (replaying an already-perfected event always costs a life)
  * a car with no lives left is renamed [WRECKED] and can't be selected (needs the
    "Block dead cars in garage" patch from burnout_revenge_nuzlocke.pnach)
  * when every car you have is wrecked, the run is dead: see your stats, then shut the
    game down or continue in Grace mode (nothing counts any more)

Requirements (Windows):  pip install pymem pefile
PCSX2: enable PINE (Settings > Advanced) and the Nuzlocke cheat (game Properties > Cheats).
Run:   python nuzlocke_tracker.py
The run is saved to nuzlocke_state.json next to this script.
"""
import ctypes
import json
import os
import socket
import struct
import time
import threading
import tkinter as tk
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from ctypes import wintypes
from datetime import datetime

try:
    import pefile
    import pymem
    import pymem.process
except ImportError:  # allows the logic to be imported/tested without the Windows libraries
    pefile = pymem = None

# --------------------------------------------------------------------------------------------
# Game addresses (SLUS-21242 v1.00)
# --------------------------------------------------------------------------------------------
SERIAL = "SLUS-21242"
RAM_SIZE = 0x2000000
PROCESS_NAMES = ["pcsx2-qt.exe", "pcsx2-qtx64-avx2.exe", "pcsx2-qtx64.exe"]

EVENT_COUNT = 0x0056B1EC          # 169
EVENT_IDS = 0x0056B250            # 8-byte packed label per event
EVENT_RESULTS = 0x01F651DB        # 1 byte per event, FF = not played, 04 = Gold + Perfect
LAST_MEDAL = 0x0056C970           # 3 Gold, 2 Silver, 1 Bronze, 0 none
LAST_RATING = 0x0056C974          # rating tier before the Gold bump (3 + Gold = Perfect)
RESULT_POS = 0x0052CA9C           # FFFFFFFF while an event is running
SELECTED_CAR = 0x01EDACD0         # 8-byte packed label of the car in use
CAROUSEL_LIST = 0x01C94990        # garage carousel: 8-byte labels
CAROUSEL_COUNT = 0x01C95534
DEAD_TABLE = 0x000FF400           # read by the patch: count, pad, then labels
DEAD_MAX = 79
TEXT_LO, TEXT_HI = 0x00670000, 0x006A0000
PATCH_HOOKS = {0x002ACE00: 0x0C03FC00, 0x002A69DC: 0x0C03FC00}

DIFFICULTIES = {"Easy": 3, "Medium": 2, "Hard": 1}
MEDALS = {3: "Gold", 2: "Silver", 1: "Bronze", 0: "No medal"}
RATINGS = ["-", "Good", "Great", "Awesome", "Perfect"]   # shown tier = stored tier (+1 with Gold)

HERE = os.path.dirname(os.path.abspath(__file__))
STATE_FILE = os.path.join(HERE, "nuzlocke_state.json")
OVERLAY_HTML = os.path.join(HERE, "nuzlocke_overlay.html")
STREAM_PORT = 8765

# --------------------------------------------------------------------------------------------
# Labels and text
# --------------------------------------------------------------------------------------------
ID_ALPHABET = " -/0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_"


def decode_label(value):
    chars = []
    for _ in range(12):
        chars.append(ID_ALPHABET[value % 40])
        value //= 40
    return "".join(reversed(chars)).strip()


def encode_label(text):
    value = 0
    for c in text.upper().ljust(12):
        value = value * 40 + ID_ALPHABET.index(c)
    return value


def _crc_table():
    table = []
    for i in range(256):
        c = i
        for _ in range(8):
            c = (c >> 1) ^ 0xEDB88320 if c & 1 else c >> 1
        table.append(c)
    return table


_CRC = _crc_table()


def text_id(label):
    """The game's label -> text ID hash (002F7050): CRC32 table, arithmetic shift, no final xor."""
    def s32(x):
        x &= 0xFFFFFFFF
        return x - (1 << 32) if x & 0x80000000 else x
    crc = -1
    for b in label.encode("ascii"):
        index = (b ^ crc) & 0xFF
        crc = (s32(crc) >> 8) ^ s32(_CRC[index])
    return crc & 0xFFFFFFFF


def find_text(region, label):
    """In a copy of the text table, find a label's text: (address, text, room) or None."""
    key = struct.pack("<I", text_id(label))
    spot = region.find(key)
    while spot != -1 and spot % 4:
        spot = region.find(key, spot + 1)
    if spot == -1:
        return None
    start = end = spot + 4
    while end + 1 < len(region) and region[end:end + 2] != b"\x00\x00":
        end += 2
    text = region[start:end].decode("utf-16-le", errors="replace")
    return TEXT_LO + start, text, len(text)


def wrecked_name(room):
    for option in ("[WRECKED]", "[WRECK]", "[X]"):
        if len(option) <= room:
            return option
    return "X"[:room]


WRECKED_NAMES = {"[WRECKED]", "[WRECK]", "[X]", "X"}


def text_capacity(region, addr):
    """Characters that fit at addr: everything up to the next entry (next non-zero 4-byte word
    after the current text), minus one character for the end marker."""
    pos = addr - TEXT_LO
    end = pos
    while end + 1 < len(region) and region[end:end + 2] != b"\x00\x00":
        end += 2
    end += 2                                   # past the end marker
    while end % 4:
        end += 1
    while end + 4 <= len(region) and region[end:end + 4] == b"\x00\x00\x00\x00":
        end += 4
    return (end - pos) // 2 - 1


def fmt_time(seconds):
    seconds = int(seconds)
    return f"{seconds // 3600}:{seconds // 60 % 60:02d}:{seconds % 60:02d}"


# --------------------------------------------------------------------------------------------
# Connection to PCSX2
# --------------------------------------------------------------------------------------------
class Pine:
    def __init__(self, slot=28011):
        self.s = socket.create_connection(("127.0.0.1", slot), timeout=2)

    def _recv(self, n):
        buf = b""
        while len(buf) < n:
            chunk = self.s.recv(n - len(buf))
            if not chunk:
                raise ConnectionError("PINE closed")
            buf += chunk
        return buf

    def _cmd(self, payload):
        self.s.sendall(struct.pack("<I", len(payload) + 4) + payload)
        size = struct.unpack("<I", self._recv(4))[0]
        body = self._recv(size - 4)
        if body[0] != 0:
            raise RuntimeError("PINE command failed")
        return body[1:]

    def write32(self, addr, value):
        self._cmd(struct.pack("<BII", 0x06, addr, value & 0xFFFFFFFF))

    def write8(self, addr, value):
        self._cmd(struct.pack("<BIB", 0x04, addr, value & 0xFF))

    def serial(self):
        data = self._cmd(struct.pack("<B", 0x0C))
        n = struct.unpack("<I", data[:4])[0]
        return data[4:4 + n].rstrip(b"\0").decode(errors="replace")

    def close(self):
        try:
            self.s.close()
        except OSError:
            pass


class Game:
    def __init__(self):
        self.pm = self.base = self.pine = None
        self.pid = None
        self.serial = None

    @property
    def connected(self):
        return self.pm is not None

    def connect(self):
        for name in PROCESS_NAMES:
            try:
                pm = pymem.Pymem(name)
            except Exception:
                continue
            mod = pymem.process.module_from_name(pm.process_handle, name)
            pe = pefile.PE(mod.filename, fast_load=True)
            pe.parse_data_directories(
                directories=[pefile.DIRECTORY_ENTRY["IMAGE_DIRECTORY_ENTRY_EXPORT"]])
            exports = getattr(pe, "DIRECTORY_ENTRY_EXPORT", None)
            for sym in (exports.symbols if exports else []):
                if sym.name == b"EEmem":
                    self.pm = pm
                    self.pid = pm.process_id
                    self.base = pm.read_ulonglong(mod.lpBaseOfDll + sym.address)
                    self.connect_pine()
                    return True
        return False

    def connect_pine(self):
        if self.pine is not None:
            return
        try:
            self.pine = Pine()
            self.serial = self.pine.serial()
        except Exception:
            self.pine = None

    def drop(self):
        if self.pine:
            self.pine.close()
        self.pm = self.base = self.pine = self.pid = self.serial = None

    def read(self, addr, n):
        return self.pm.read_bytes(self.base + addr, n)

    def u32(self, addr):
        return struct.unpack("<I", self.read(addr, 4))[0]

    def u64(self, addr):
        return struct.unpack("<Q", self.read(addr, 8))[0]

    def write(self, addr, data):
        """Direct write; pages holding translated code are protected, so fall back to PINE."""
        try:
            self.pm.write_bytes(self.base + addr, data, len(data))
            return
        except Exception:
            self.connect_pine()
            if self.pine is None:
                raise RuntimeError("PINE is needed for this write (enable it in PCSX2)")
        try:
            i = 0
            while i + 4 <= len(data):
                self.pine.write32(addr + i, struct.unpack_from("<I", data, i)[0])
                i += 4
            while i < len(data):
                self.pine.write8(addr + i, data[i])
                i += 1
        except Exception:
            self.pine.close()
            self.pine = None
            raise


# --------------------------------------------------------------------------------------------
# The run
# --------------------------------------------------------------------------------------------
def new_run(difficulty):
    return {
        "active": True, "difficulty": difficulty, "lives_start": DIFFICULTIES[difficulty],
        "started": datetime.now().isoformat(timespec="seconds"),
        "play_seconds": 0.0, "grace": False, "dead": False,
        "cars": {},            # label -> {"name", "lives"}
        "events_played": 0, "events_won": 0, "cars_lost": 0,
        "streak": 0, "best_streak": 0,
        "history": [],
    }


class Tracker:
    def __init__(self, game):
        self.game = game
        self.state = self.load()
        self.message = ""
        self.message_kind = "info"
        self.message_id = 0
        self.message_until = 0.0
        self.event_active = False
        self.event_start_results = None
        self.pending_finish_at = None
        self.known_results = None
        self.patch_ok = None
        self.wrong_game = False
        self.on_run_dead = None        # set by the UI
        self.last_slow = 0.0
        self.last_tick = time.time()
        self.originals = {}            # car label -> original name, used to undo renames

    # ---- state on disk ----
    def load(self):
        try:
            with open(STATE_FILE, encoding="utf-8") as f:
                return json.load(f)
        except (OSError, ValueError):
            return {"run": None}

    def save(self):
        tmp = STATE_FILE + ".tmp"
        with open(tmp, "w", encoding="utf-8") as f:
            json.dump(self.state, f, indent=1)
        os.replace(tmp, STATE_FILE)

    @property
    def run(self):
        return self.state.get("run")

    def counting(self):
        r = self.run
        return bool(r and r["active"] and not r["grace"] and not r["dead"])

    def say(self, text, seconds=15, kind="info"):
        self.message = text
        self.message_kind = kind
        self.message_id = getattr(self, "message_id", 0) + 1
        self.message_until = time.time() + seconds

    # ---- run control ----
    def start_run(self, difficulty):
        if self.run:
            self.restore_names(clear_table=True)
        self.state["run"] = new_run(difficulty)
        self.originals = {}
        self.save()
        self.say(f"New {difficulty} run: {DIFFICULTIES[difficulty]} lives per car.")

    def grace(self):
        if self.run:
            self.run["grace"] = True
            self.save()
            self.restore_names(clear_table=True)
            self.say("Grace mode: nothing counts any more. All cars are usable again.")

    # ---- polling (called from the UI loop) ----
    def poll(self):
        now = time.time()
        dt, self.last_tick = now - self.last_tick, now
        g = self.game
        if not g.connected:
            if not g.connect():
                return
            self.event_active, self.known_results, self.pending_finish_at = False, None, None
        try:
            self._poll_connected(now, dt)
        except Exception:
            g.drop()

    def _poll_connected(self, now, dt):
        g = self.game
        count = g.u32(EVENT_COUNT)
        if g.serial and g.serial != SERIAL:
            self.wrong_game = True
            return
        if count != 169:            # not in the game yet (booting, BIOS, other game)
            self.wrong_game = g.serial is not None and g.serial != SERIAL
            return
        self.wrong_game = False
        r = self.run
        if r and r["active"] and not r["dead"]:
            r["play_seconds"] += min(dt, 5.0)

        results = g.read(EVENT_RESULTS, count)
        if self.known_results is None or len(self.known_results) != len(results):
            self.known_results = results

        # event start / finish
        in_event = g.u32(RESULT_POS) == 0xFFFFFFFF
        if in_event and not self.event_active:
            self.event_active = True
            self.event_start_results = self.known_results
        elif not in_event and self.event_active:
            self.event_active = False
            self.pending_finish_at = now + 1.0       # let the game finish writing the result
        if self.pending_finish_at and now >= self.pending_finish_at:
            self.pending_finish_at = None
            self.finish_event(self.event_start_results or self.known_results)
            self.known_results = g.read(EVENT_RESULTS, count)
        elif not self.event_active and not self.pending_finish_at and results != self.known_results:
            changed = sum(a != b for a, b in zip(results, self.known_results))
            if changed == 1:                          # a result we didn't see start
                self.finish_event(self.known_results)
            self.known_results = results              # many changes = a profile was loaded

        if now - self.last_slow >= 2.0:
            self.last_slow = now
            self.slow_checks()

    def finish_event(self, before):
        g, r = self.game, self.run
        count = g.u32(EVENT_COUNT)
        after = g.read(EVENT_RESULTS, count)
        medal, rating = g.u32(LAST_MEDAL), g.u32(LAST_RATING)
        car = decode_label(g.u64(SELECTED_CAR))
        diffs = [(i, a, b) for i, (a, b) in enumerate(zip(before, after)) if a != b]
        event_label = None
        if diffs:
            event_label = decode_label(g.u64(EVENT_IDS + diffs[0][0] * 8))
        perfect_now = any(b == 0x04 and a != 0x04 for _, a, b in diffs)
        won = medal == 3 and rating == 3 and perfect_now

        region = g.read(TEXT_LO, TEXT_HI - TEXT_LO)
        event_name = self.name_of(region, event_label) if event_label else "Unknown / replayed event"
        car_name = self.car_name(region, car)
        shown_rating = RATINGS[min(4, rating + (1 if medal == 3 else 0))] if rating <= 3 else "?"
        result_text = f"{MEDALS.get(medal, '?')} + {shown_rating}"

        if not r or not r["active"]:
            return
        entry = {"time": datetime.now().isoformat(timespec="seconds"), "event": event_name,
                 "car": car_name, "result": result_text, "won": won, "counted": self.counting()}
        r["history"] = (r["history"] + [entry])[-200:]
        if not self.counting():
            self.say(f"{event_name}: {result_text} (not counted)")
            self.save()
            return

        self.register_car(car, car_name)
        r["events_played"] += 1
        if won:
            r["events_won"] += 1
            r["streak"] += 1
            r["best_streak"] = max(r["best_streak"], r["streak"])
            self.say(f"{event_name}: Gold + Perfect", kind="win")
        else:
            r["streak"] = 0
            c = r["cars"][car]
            c["lives"] = max(0, c["lives"] - 1)
            reason = "already perfected" if (medal == 3 and rating == 3 and not perfect_now) else result_text
            if c["lives"] == 0:
                r["cars_lost"] += 1
                self.say(f"{car_name} is wrecked ({event_name}: {reason})", 25, kind="loss")
            else:
                lives = c["lives"]
                self.say(f"{car_name} lost a life, {lives} left ({event_name}: {reason})", 20, kind="loss")
        self.save()
        self.slow_checks()
        if self.all_dead():
            r["dead"] = True
            self.save()
            if self.on_run_dead:
                self.on_run_dead()

    # ---- cars ----
    def name_of(self, region, label):
        found = find_text(region, label) if label else None
        return found[1] if found else (label or "?")

    def car_name(self, region, label):
        c = self.run["cars"].get(label) if self.run else None
        if c and c.get("name") and c["name"] != "?":
            return c["name"]
        name = self.name_of(region, label)
        return label if name in WRECKED_NAMES else name

    def register_car(self, label, name):
        r = self.run
        if not r or "CAR" not in label:
            return
        if label not in r["cars"]:
            r["cars"][label] = {"name": name, "lives": r["lives_start"]}
        elif r["cars"][label].get("name") in (None, "?", label) and name not in WRECKED_NAMES:
            r["cars"][label]["name"] = name

    def all_dead(self):
        cars = self.run["cars"] if self.run else {}
        return bool(cars) and all(c["lives"] == 0 for c in cars.values())

    def dead_labels(self):
        r = self.run
        if not r or not r["active"] or r["grace"]:
            return []
        return sorted(l for l, c in self.run["cars"].items() if c["lives"] == 0)

    # ---- every 2 seconds: register cars, enforce the dead list and names, check the patch ----
    def slow_checks(self):
        g, r = self.game, self.run
        region = g.read(TEXT_LO, TEXT_HI - TEXT_LO)
        if r and r["active"]:
            n = g.u32(CAROUSEL_COUNT)
            if 0 < n <= DEAD_MAX:
                raw = g.read(CAROUSEL_LIST, n * 8)
                labels = [decode_label(struct.unpack_from("<Q", raw, i * 8)[0]) for i in range(n)]
                if all("CAR" in l for l in labels):
                    before = len(r["cars"])
                    for l in labels:
                        self.register_car(l, self.car_name(region, l))
                    if len(r["cars"]) != before:
                        self.save()

        # the patch's dead-car list
        dead = self.dead_labels()
        want = struct.pack("<II", len(dead), 0) + b"".join(struct.pack("<Q", encode_label(l)) for l in dead)
        if g.read(DEAD_TABLE, len(want)) != want:
            try:
                g.write(DEAD_TABLE, want)
            except Exception as e:
                self.say(f"Couldn't update the dead-car list: {e}", 10)

        # names: dead cars show [WRECKED], everything else its real name
        for label in dead:
            found = find_text(region, label)
            if not found:
                continue
            addr, text, room = found
            if text not in WRECKED_NAMES:
                self.originals[label] = text
                if r["cars"][label].get("name") in (None, "?", label):
                    r["cars"][label]["name"] = text
            new = wrecked_name(room)
            if text != new:
                data = new.encode("utf-16-le")
                g.write(addr, data + b"\x00" * (room * 2 + 2 - len(data)))
        if not dead:
            self.restore_names(region)

        # is the patch active?
        self.patch_ok = all(g.u32(a) == v for a, v in PATCH_HOOKS.items())

    def restore_names(self, region=None, clear_table=False):
        """Put real names back on wrecked cars. Only writes a name we know for sure, and only
        if it fits the slot's original space (the zero bytes left behind when it was renamed)."""
        g = self.game
        if not g.connected or not self.run:
            return
        try:
            region = region or g.read(TEXT_LO, TEXT_HI - TEXT_LO)
            for label, c in self.run["cars"].items():
                original = self.originals.get(label) or c.get("name")
                if not original or original in WRECKED_NAMES or original == label:
                    continue
                found = find_text(region, label)
                if not found or found[1] not in WRECKED_NAMES:
                    continue
                addr = found[0]
                if len(original) <= text_capacity(region, addr):
                    g.write(addr, original.encode("utf-16-le") + b"\x00\x00")
            if clear_table:
                g.write(DEAD_TABLE, struct.pack("<II", 0, 0))
        except Exception:
            pass

    def shut_down_game(self):
        if self.game.pid:
            for hwnd in top_windows(self.game.pid):
                user32.PostMessageW(hwnd, 0x0010, 0, 0)     # WM_CLOSE



# --------------------------------------------------------------------------------------------
# Stream overlay: http://localhost:8765 for an OBS Browser Source
# --------------------------------------------------------------------------------------------
class StreamServer:
    def __init__(self):
        self.snapshot = {"run": None}
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                if self.path.startswith("/state"):
                    body = json.dumps(owner.snapshot).encode()
                    ctype = "application/json"
                else:
                    try:
                        with open(OVERLAY_HTML, "rb") as f:
                            body = f.read()
                    except OSError:
                        body = b"nuzlocke_overlay.html is missing next to nuzlocke_tracker.py"
                    ctype = "text/html; charset=utf-8"
                self.send_response(200)
                self.send_header("Content-Type", ctype)
                self.send_header("Cache-Control", "no-store")
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *args):
                pass

        try:
            self.httpd = ThreadingHTTPServer(("127.0.0.1", STREAM_PORT), Handler)
            threading.Thread(target=self.httpd.serve_forever, daemon=True).start()
            self.ok = True
        except OSError:
            self.ok = False

# --------------------------------------------------------------------------------------------
# Windows helpers
# --------------------------------------------------------------------------------------------
user32 = ctypes.windll.user32 if os.name == "nt" else None


def top_windows(pid):
    found = []
    if not user32:
        return found
    proc = ctypes.WINFUNCTYPE(wintypes.BOOL, wintypes.HWND, wintypes.LPARAM)

    def cb(hwnd, _):
        owner = wintypes.DWORD()
        user32.GetWindowThreadProcessId(hwnd, ctypes.byref(owner))
        if owner.value == pid and user32.IsWindowVisible(hwnd):
            found.append(hwnd)
        return True
    user32.EnumWindows(proc(cb), 0)
    return found


def game_window_rect(pid):
    """Largest visible window of PCSX2 (the game view), or None if minimised/missing."""
    best, best_area = None, 0
    for hwnd in top_windows(pid):
        if user32.IsIconic(hwnd):
            continue
        rect = wintypes.RECT()
        user32.GetWindowRect(hwnd, ctypes.byref(rect))
        area = (rect.right - rect.left) * (rect.bottom - rect.top)
        if area > best_area:
            best, best_area = rect, area
    return best


def make_click_through(win):
    if not user32:
        return
    hwnd = user32.GetParent(win.winfo_id())
    style = user32.GetWindowLongW(hwnd, -20)
    user32.SetWindowLongW(hwnd, -20, style | 0x80000 | 0x20 | 0x80 | 0x08000000)


# --------------------------------------------------------------------------------------------
# User interface
# --------------------------------------------------------------------------------------------
BG, FG, DIM, GOOD, BAD, ACCENT = "#14110d", "#f2e6c9", "#8a7f6a", "#7bd88f", "#ff6a4d", "#f5b82e"
FONT = "Segoe UI"


class App:
    def __init__(self):
        self.game = Game()
        self.tracker = Tracker(self.game)
        self.tracker.on_run_dead = lambda: self.root.after(0, self.show_run_dead)
        self.stream = StreamServer()

        self.root = tk.Tk()
        self.root.title("Burnout Nuzlocke")
        self.root.configure(bg=BG)
        self.root.resizable(False, False)
        self.status = tk.Label(self.root, text="", bg=BG, fg=FG, font=(FONT, 10), justify="left",
                               anchor="w", width=46)
        self.status.pack(padx=12, pady=(12, 6), fill="x")
        bar = tk.Frame(self.root, bg=BG)
        bar.pack(padx=12, pady=(0, 12), fill="x")
        for text, cmd in (("New run", self.ask_difficulty), ("Run stats", self.show_stats),
                          ("Desktop overlay on/off", self.toggle_overlay)):
            tk.Button(bar, text=text, command=cmd, font=(FONT, 9)).pack(side="left", padx=(0, 6))
        url = f"http://localhost:{STREAM_PORT}"
        stream_text = (f"Stream overlay (OBS Browser Source): {url}" if self.stream.ok
                       else f"Stream overlay unavailable: port {STREAM_PORT} is in use")
        srow = tk.Frame(self.root, bg=BG)
        srow.pack(padx=12, pady=(0, 12), fill="x")
        tk.Label(srow, text=stream_text, bg=BG, fg=DIM, font=(FONT, 9)).pack(side="left")
        if self.stream.ok:
            tk.Button(srow, text="Copy", font=(FONT, 8),
                      command=lambda: (self.root.clipboard_clear(), self.root.clipboard_append(url))
                      ).pack(side="left", padx=6)

        self.overlay_on = True
        self.overlay = tk.Toplevel(self.root)
        self.overlay.overrideredirect(True)
        self.overlay.attributes("-topmost", True)
        self.overlay.attributes("-alpha", 0.86)
        self.overlay.configure(bg=BG)
        self.ov_text = tk.Label(self.overlay, text="", bg=BG, fg=FG, font=(FONT, 11), justify="left",
                                anchor="w")
        self.ov_text.pack(padx=12, pady=10)
        self.overlay.update_idletasks()
        make_click_through(self.overlay)
        self.overlay.withdraw()

        r = self.tracker.run
        if not r or not r["active"]:
            self.root.after(300, self.ask_difficulty)
        elif r["dead"] and not r["grace"]:
            self.root.after(300, self.show_run_dead)
        self.root.after(250, self.loop)

    # ---- main loop ----
    def loop(self):
        self.tracker.poll()
        self.refresh()
        self.root.after(250, self.loop)

    def refresh(self):
        t, g, r = self.tracker, self.game, self.tracker.run
        lines = []
        if not g.connected:
            lines.append("Waiting for PCSX2...")
        elif t.wrong_game:
            lines.append(f"This isn't Burnout Revenge {SERIAL}.")
        elif t.patch_ok is False:
            lines.append("! Nuzlocke patch not active: enable the cheat and restart the game.")
        if g.connected and g.pine is None:
            lines.append("! PINE not connected: enable it in PCSX2 (needed to block cars).")
        if r:
            alive = sum(1 for c in r["cars"].values() if c["lives"] > 0)
            mode = "GRACE (not counting)" if r["grace"] else ("RUN'S DEAD" if r["dead"] else r["difficulty"])
            lines.append(f"{mode}   Won {r['events_won']}/{r['events_played']}   "
                         f"Cars {alive} alive / {r['cars_lost']} lost   {fmt_time(r['play_seconds'])}")
        self.status.config(text="\n".join(lines) or "Connected.")
        self.update_overlay(r)
        self.stream.snapshot = self.build_snapshot(r)

    def build_snapshot(self, r):
        t, g = self.tracker, self.game
        if not r or not r["active"]:
            return {"run": None, "connected": g.connected}
        car = None
        try:
            if g.connected:
                label = decode_label(g.u64(SELECTED_CAR))
                c = r["cars"].get(label)
                if c:
                    car = {"name": c["name"], "lives": c["lives"]}
        except Exception:
            pass
        message = None
        if t.message and time.time() < t.message_until:
            message = {"text": t.message, "kind": t.message_kind, "id": t.message_id}
        return {
            "connected": g.connected,
            "run": {
                "difficulty": r["difficulty"], "lives_start": r["lives_start"],
                "grace": r["grace"], "dead": r["dead"],
                "won": r["events_won"], "played": r["events_played"],
                "cars_total": len(r["cars"]), "cars_lost": r["cars_lost"],
                "time": fmt_time(r["play_seconds"]), "best_streak": r["best_streak"],
                "streak": r["streak"], "started": r["started"].replace("T", " "),
            },
            "car": car,
            "message": message,
        }

    def update_overlay(self, r):
        g, t = self.game, self.tracker
        rect = game_window_rect(g.pid) if (g.connected and self.overlay_on) else None
        if not rect or not r:
            self.overlay.withdraw()
            return
        rows = []
        title = "NUZLOCKE  " + ("GRACE" if r["grace"] else ("RUN'S DEAD" if r["dead"] else r["difficulty"].upper()))
        rows.append(title)
        rows.append(f"Won {r['events_won']}/{r['events_played']}   Lost {r['cars_lost']} cars")
        try:
            car = decode_label(g.u64(SELECTED_CAR))
        except Exception:
            car = None
        c = r["cars"].get(car) if car else None
        if c:
            hearts = "♥" * c["lives"] + "♡" * (r["lives_start"] - c["lives"])
            rows.append(f"{c['name']}  {hearts}" if c["lives"] else f"{c['name']}  WRECKED")
        if t.patch_ok is False:
            rows.append("! patch not active")
        if t.message and time.time() < t.message_until:
            rows.append("")
            rows.append(t.message)
        text = "\n".join(rows)
        if self.ov_text.cget("text") != text:
            showing_loss = t.message_kind == "loss" and t.message and t.message in text
            colour = BAD if (r["dead"] or "WRECKED" in text or showing_loss) else FG
            self.ov_text.config(text=text, fg=colour, wraplength=380)
        self.overlay.update_idletasks()
        w, h = self.overlay.winfo_reqwidth(), self.overlay.winfo_reqheight()
        x, y = rect.right - w - 24, rect.top + 70
        self.overlay.geometry(f"{w}x{h}+{x}+{y}")
        self.overlay.deiconify()
        self.overlay.attributes("-topmost", True)

    def toggle_overlay(self):
        self.overlay_on = not self.overlay_on

    # ---- dialogs ----
    def dialog(self, title):
        d = tk.Toplevel(self.root)
        d.title(title)
        d.configure(bg=BG)
        d.attributes("-topmost", True)
        d.resizable(False, False)
        d.transient(self.root)
        return d

    def ask_difficulty(self):
        r = self.tracker.run
        d = self.dialog("New Nuzlocke run")
        tk.Label(d, text="NEW RUN", bg=BG, fg=ACCENT, font=(FONT, 18, "bold")).pack(padx=24, pady=(18, 4))
        msg = "Lives per car for this run (can't be changed later)."
        if r and r["active"] and not r["dead"]:
            msg += "\nThis ends your current run."
        tk.Label(d, text=msg, bg=BG, fg=FG, font=(FONT, 10)).pack(padx=24, pady=(0, 12))
        row = tk.Frame(d, bg=BG)
        row.pack(padx=24, pady=(0, 20))
        for name, lives in DIFFICULTIES.items():
            tk.Button(row, text=f"{name}\n{lives} {'life' if lives == 1 else 'lives'}", width=10,
                      font=(FONT, 11), command=lambda n=name: (self.tracker.start_run(n), d.destroy())
                      ).pack(side="left", padx=6)

    def stats_text(self, r):
        rate = f" ({100 * r['events_won'] // r['events_played']}%)" if r["events_played"] else ""
        return (f"Difficulty: {r['difficulty']} ({r['lives_start']} per car)\n"
                f"Events won: {r['events_won']} / {r['events_played']}{rate}\n"
                f"Best win streak: {r['best_streak']}\n"
                f"Cars lost: {r['cars_lost']} of {len(r['cars'])}\n"
                f"Total time: {fmt_time(r['play_seconds'])}\n"
                f"Started: {r['started'].replace('T', ' ')}")

    def show_stats(self):
        r = self.tracker.run
        if not r:
            return
        d = self.dialog("Run stats")
        tk.Label(d, text="RUN STATS", bg=BG, fg=ACCENT, font=(FONT, 16, "bold")).pack(padx=24, pady=(16, 6))
        tk.Label(d, text=self.stats_text(r), bg=BG, fg=FG, font=(FONT, 11), justify="left"
                 ).pack(padx=24, pady=(0, 10))
        recent = [h for h in r["history"] if h["counted"]][-8:]
        if recent:
            log = "\n".join(f"{'WIN ' if h['won'] else 'LOSS'}  {h['event']}  -  {h['car']}  ({h['result']})"
                            for h in reversed(recent))
            tk.Label(d, text=log, bg=BG, fg=DIM, font=(FONT, 9), justify="left").pack(padx=24, pady=(0, 16))

    def show_run_dead(self):
        r = self.tracker.run
        if not r:
            return
        d = self.dialog("Run's Dead")
        tk.Label(d, text="RUN'S DEAD", bg=BG, fg=BAD, font=(FONT, 26, "bold")).pack(padx=40, pady=(22, 4))
        tk.Label(d, text="Every car you have is wrecked.", bg=BG, fg=FG, font=(FONT, 11)).pack(pady=(0, 14))
        tk.Label(d, text=self.stats_text(r), bg=BG, fg=FG, font=(FONT, 12), justify="left").pack(padx=40)
        row = tk.Frame(d, bg=BG)
        row.pack(padx=24, pady=22)

        def shut_down():
            self.tracker.shut_down_game()
            d.destroy()

        def grace():
            self.tracker.grace()
            d.destroy()
        tk.Button(row, text="Shut down game", width=16, font=(FONT, 11), command=shut_down).pack(side="left", padx=8)
        tk.Button(row, text="Grace continue\n(doesn't count)", width=16, font=(FONT, 11),
                  command=grace).pack(side="left", padx=8)
        tk.Button(d, text="New run", font=(FONT, 9), command=lambda: (d.destroy(), self.ask_difficulty())
                  ).pack(pady=(0, 16))

    def run(self):
        self.root.mainloop()


if __name__ == "__main__":
    try:
        ctypes.windll.shcore.SetProcessDpiAwareness(1)   # keep overlay coordinates in real pixels
    except Exception:
        pass
    App().run()
