//go:build windows

package main

import (
	"encoding/binary"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	user32            = syscall.NewLazyDLL("user32.dll")
	procOpenProcess   = kernel32.NewProc("OpenProcess")
	procReadMem       = kernel32.NewProc("ReadProcessMemory")
	procWriteMem      = kernel32.NewProc("WriteProcessMemory")
	procCloseHandle   = kernel32.NewProc("CloseHandle")
	procSnapshot      = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcFirst     = kernel32.NewProc("Process32FirstW")
	procProcNext      = kernel32.NewProc("Process32NextW")
	procModFirst      = kernel32.NewProc("Module32FirstW")
	procExitCode      = kernel32.NewProc("GetExitCodeProcess")
	procEnumWindows   = user32.NewProc("EnumWindows")
	procWindowPid     = user32.NewProc("GetWindowThreadProcessId")
	procWindowVisible = user32.NewProc("IsWindowVisible")
	procPostMessage   = user32.NewProc("PostMessageW")
)

var processNames = []string{"pcsx2-qt.exe", "pcsx2-qtx64-avx2.exe", "pcsx2-qtx64.exe"}

type processEntry struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

type moduleEntry struct {
	Size         uint32
	ModuleID     uint32
	ProcessID    uint32
	GlblcntUsage uint32
	ProccntUsage uint32
	ModBaseAddr  uintptr
	ModBaseSize  uint32
	HModule      uintptr
	Module       [256]uint16
	ExePath      [260]uint16
}

type Game struct {
	handle uintptr
	pid    uint32
	eeBase uintptr
	pine   *Pine
	serial string
}

func NewGame() *Game { return &Game{} }

func (g *Game) Connected() bool {
	if g.handle == 0 {
		return false
	}
	var code uint32
	r, _, _ := procExitCode.Call(g.handle, uintptr(unsafe.Pointer(&code)))
	if r == 0 || code != 259 { // STILL_ACTIVE
		g.Drop()
		return false
	}
	return true
}

func findPcsx2() uint32 {
	snap, _, _ := procSnapshot.Call(0x2, 0) // TH32CS_SNAPPROCESS
	if snap == uintptr(syscall.InvalidHandle) {
		return 0
	}
	defer procCloseHandle.Call(snap)
	var e processEntry
	e.Size = uint32(unsafe.Sizeof(e))
	for ok, _, _ := procProcFirst.Call(snap, uintptr(unsafe.Pointer(&e))); ok != 0; ok, _, _ = procProcNext.Call(snap, uintptr(unsafe.Pointer(&e))) {
		name := strings.ToLower(syscall.UTF16ToString(e.ExeFile[:]))
		for _, want := range processNames {
			if name == want {
				return e.ProcessID
			}
		}
	}
	return 0
}

func (g *Game) rawRead(addr uintptr, n int) ([]byte, error) {
	buf := make([]byte, n)
	var done uintptr
	r, _, err := procReadMem.Call(g.handle, addr, uintptr(unsafe.Pointer(&buf[0])), uintptr(n), uintptr(unsafe.Pointer(&done)))
	if r == 0 || int(done) != n {
		return nil, err
	}
	return buf, nil
}

// findEEmem reads the export table of the PCSX2 executable straight from its memory.
func (g *Game) findEEmem(base uintptr) uintptr {
	dos, err := g.rawRead(base, 0x40)
	if err != nil {
		return 0
	}
	nt := base + uintptr(binary.LittleEndian.Uint32(dos[0x3C:]))
	opt, err := g.rawRead(nt+24, 0x80)
	if err != nil || binary.LittleEndian.Uint16(opt) != 0x20B {
		return 0
	}
	expRVA := binary.LittleEndian.Uint32(opt[112:])
	exp, err := g.rawRead(base+uintptr(expRVA), 40)
	if err != nil {
		return 0
	}
	count := binary.LittleEndian.Uint32(exp[24:])
	funcs := base + uintptr(binary.LittleEndian.Uint32(exp[28:]))
	names := base + uintptr(binary.LittleEndian.Uint32(exp[32:]))
	ords := base + uintptr(binary.LittleEndian.Uint32(exp[36:]))
	for i := uint32(0); i < count; i++ {
		nr, err := g.rawRead(names+uintptr(4*i), 4)
		if err != nil {
			return 0
		}
		name, err := g.rawRead(base+uintptr(binary.LittleEndian.Uint32(nr)), 6)
		if err != nil || string(name) != "EEmem\x00" {
			continue
		}
		ob, err := g.rawRead(ords+uintptr(2*i), 2)
		if err != nil {
			return 0
		}
		fb, err := g.rawRead(funcs+uintptr(4*uint32(binary.LittleEndian.Uint16(ob))), 4)
		if err != nil {
			return 0
		}
		ptr, err := g.rawRead(base+uintptr(binary.LittleEndian.Uint32(fb)), 8)
		if err != nil {
			return 0
		}
		return uintptr(binary.LittleEndian.Uint64(ptr))
	}
	return 0
}

func (g *Game) Connect() bool {
	pid := findPcsx2()
	if pid == 0 {
		return false
	}
	h, _, _ := procOpenProcess.Call(0x0010|0x0020|0x0008|0x0400, 0, uintptr(pid))
	if h == 0 {
		return false
	}
	g.handle, g.pid = h, pid
	snap, _, _ := procSnapshot.Call(0x8|0x10, uintptr(pid)) // TH32CS_SNAPMODULE | SNAPMODULE32
	if snap == uintptr(syscall.InvalidHandle) {
		g.Drop()
		return false
	}
	var m moduleEntry
	m.Size = uint32(unsafe.Sizeof(m))
	ok, _, _ := procModFirst.Call(snap, uintptr(unsafe.Pointer(&m)))
	procCloseHandle.Call(snap)
	if ok == 0 {
		g.Drop()
		return false
	}
	g.eeBase = g.findEEmem(m.ModBaseAddr)
	if g.eeBase == 0 {
		g.Drop()
		return false
	}
	g.connectPine()
	return true
}

func (g *Game) connectPine() {
	if g.pine != nil {
		return
	}
	if p, err := dialPine(); err == nil {
		g.pine = p
		g.serial = p.Serial()
	}
}

func (g *Game) Drop() {
	if g.handle != 0 {
		procCloseHandle.Call(g.handle)
	}
	if g.pine != nil {
		g.pine.Close()
	}
	*g = Game{}
}

func (g *Game) Read(addr uint32, n int) ([]byte, error) {
	if g.handle == 0 {
		return nil, errNotConnected
	}
	b, err := g.rawRead(g.eeBase+uintptr(addr), n)
	if b == nil {
		if err == nil {
			err = errNotConnected
		}
		return nil, err
	}
	return b, nil
}

// Write tries a direct write first; pages with translated code are protected, so fall back to PINE.
func (g *Game) Write(addr uint32, data []byte) error {
	if g.handle == 0 {
		return errNotConnected
	}
	var done uintptr
	r, _, _ := procWriteMem.Call(g.handle, g.eeBase+uintptr(addr), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&done)))
	if r != 0 && int(done) == len(data) {
		return nil
	}
	g.connectPine()
	if g.pine == nil {
		return errNeedPine
	}
	if err := g.pine.WriteBytes(addr, data); err != nil {
		g.pine.Close()
		g.pine = nil
		return err
	}
	return nil
}

func (g *Game) Serial() string { return g.serial }
func (g *Game) HasPine() bool  { return g.pine != nil }

// CloseWindows asks PCSX2 to close (like clicking its X button).
func (g *Game) CloseWindows() {
	pid := g.pid
	if pid == 0 {
		pid = findPcsx2()
	}
	if pid == 0 {
		return
	}
	cb := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var owner uint32
		procWindowPid.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
		if owner == pid {
			if v, _, _ := procWindowVisible.Call(hwnd); v != 0 {
				procPostMessage.Call(hwnd, 0x0010, 0, 0) // WM_CLOSE
			}
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
}

func openBrowser(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
