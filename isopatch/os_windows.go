package main

import (
	"errors"
	"os/exec"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const canBrowse = true

var (
	comdlg32             = syscall.NewLazyDLL("comdlg32.dll")
	procGetOpenFileName  = comdlg32.NewProc("GetOpenFileNameW")
	procCommDlgExtError  = comdlg32.NewProc("CommDlgExtendedError")
	user32               = syscall.NewLazyDLL("user32.dll")
	procForegroundWindow = user32.NewProc("GetForegroundWindow")
	ole32                = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32.NewProc("CoUninitialize")
)

// OPENFILENAMEW
type openFileName struct {
	structSize    uint32
	owner         uintptr
	instance      uintptr
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	fileTitle     *uint16
	maxFileTitle  uint32
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExtension uint16
	defExt        *uint16
	custData      uintptr
	hook          uintptr
	templateName  *uint16
	reserved      uintptr
	reserved2     uint32
	flagsEx       uint32
}

// browseISO shows Windows' Open dialog for the game's ISO. "" if it was cancelled.
func browseISO() (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if r, _, _ := procCoInitializeEx.Call(0, 2); r == 0 || r == 1 { // COINIT_APARTMENTTHREADED: S_OK or S_FALSE
		defer procCoUninitialize.Call()
	}
	filter := utf16.Encode([]rune("PS2 disc images (*.iso)\x00*.iso\x00All files (*.*)\x00*.*\x00\x00"))
	title, _ := syscall.UTF16PtrFromString("Choose your Burnout Revenge (USA) ISO")
	buf := make([]uint16, 4096)
	owner, _, _ := procForegroundWindow.Call() // the browser, so the dialog comes up in front of it
	ofn := openFileName{owner: owner, filter: &filter[0], filterIndex: 1, file: &buf[0], maxFile: uint32(len(buf)),
		title: title, flags: 0x00080000 | 0x00001000 | 0x00000800 | 0x00000008} // EXPLORER, FILEMUSTEXIST, PATHMUSTEXIST, NOCHANGEDIR
	ofn.structSize = uint32(unsafe.Sizeof(ofn))
	if r, _, _ := procGetOpenFileName.Call(uintptr(unsafe.Pointer(&ofn))); r == 0 {
		if e, _, _ := procCommDlgExtError.Call(); e != 0 {
			return "", errors.New("the file dialog didn't open: type or paste the ISO's path instead")
		}
		return "", nil // cancelled
	}
	return syscall.UTF16ToString(buf), nil
}

func showInFolder(path string) {
	_ = exec.Command("explorer", "/select,", path).Start()
}

func openBrowser(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
