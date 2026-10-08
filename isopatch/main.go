// nuzlocke_isopatch builds a patched copy of a Burnout Revenge (USA, SLUS-21242) ISO with the Nuzlocke
// mod built in: the dead-car block, the finished-event signal, the pause-menu Retry/Quit block and one
// Harder AI level (faster and more aggressive opponents), optionally with widescreen 16:9 and 60 FPS
// menus (by SuperType1/remco) and the Limited Selection, Revive tokens and Event Roulette modes. The original ISO is
// never modified.
//
// Run it (or drag the ISO onto it) and choose what goes in on the page it opens in your browser. Every
// part can be chosen on its own; the modes need the Nuzlocke rules, since nuzlocke.exe runs them.
//
// Command line: nuzlocke_isopatch.exe game.iso <easy|medium|hard|insane|off> [widescreen] [60fps] [limited]
// [revive] [roulette] [allcars] [chaos] makes the full Nuzlocke (dead-car and pause blocks) with that AI.
package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	sectorSize = 2048
	elfName    = "SLUS_212.42"
	elfSHA1    = "d861e1bfb8b29ed79a2fc78355d79a5aa54ea308" // unmodified US release

)

func main() {
	args := os.Args[1:]
	if len(args) >= 2 {
		// command line: nuzlocke_isopatch.exe game.iso <level> [options...], the full Nuzlocke with that level
		if err := runCLI(args); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		return
	}
	iso := ""
	if len(args) == 1 {
		iso = strings.Trim(strings.TrimSpace(args[0]), `"`) // an ISO dragged onto the .exe
	}
	if err := runGUI(iso); err != nil {
		fmt.Println("Error:", err)
		fmt.Print("Press Enter to close.")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		os.Exit(1)
	}
}

func runCLI(args []string) error {
	fmt.Println("Burnout Revenge Nuzlocke - ISO patcher v" + version)
	isoPath := strings.Trim(strings.TrimSpace(args[0]), `"`)
	sel := Selection{Core: true, Pause: true}
	if !strings.EqualFold(args[1], "off") {
		if levelIndex(args[1]) < 0 {
			return errors.New("unknown AI level " + args[1] + " (easy, medium, hard, insane or off)")
		}
		sel.Level = levels[levelIndex(args[1])].name
	}
	for _, a := range args[2:] {
		a = strings.ToLower(a)
		switch a {
		case "60fps":
			a = "fps60"
		case "ws":
			a = "widescreen"
		}
		sel.Options = append(sel.Options, a)
	}
	if err := sel.Validate(); err != nil {
		return err
	}
	if sel.Level == "Insane" {
		fmt.Println(insaneWarning)
	}
	outPath := sel.OutputPath(isoPath)
	fmt.Println("Writing", filepath.Base(outPath), "...")
	if err := patchISO(isoPath, outPath, sel, nil); err != nil {
		return err
	}
	fmt.Println("Done:", outPath)
	fmt.Println("Play that ISO. Don't also enable the Nuzlocke .pnach for it.")
	return nil
}

const insane = 3 // index of the Insane level

const insaneWarning = `WARNING: Insane is not meant to be fair.
  Opponents have the full speed of the original Hard level (corner at the limit, 257 mph, the
  strongest catch-up), and every one of them attacks you almost all the time: from the start
  line, from up to 150 m away, blocking you for up to 15 s and slamming twice as hard.
  Combined with 1 + 1 lives (Hard in the tracker), most runs will end within a few events.`

func levelIndex(s string) int {
	switch strings.ToLower(s) {
	case "1", "easy":
		return 0
	case "2", "medium":
		return 1
	case "3", "hard":
		return 2
	case "4", "insane":
		return insane
	}
	return -1
}

// patchISO copies the ISO to outPath and replaces SLUS_212.42 inside the copy with the patched one.
// progress, if given, is told how much of the copy is done.
func patchISO(isoPath, outPath string, sel Selection, progress func(done, total int64)) error {
	if err := sel.Validate(); err != nil {
		return err
	}
	iso, err := os.Open(isoPath)
	if err != nil {
		return err
	}
	defer iso.Close()

	lba, size, err := findFile(iso, elfName)
	if err != nil {
		return err
	}
	elf := make([]byte, size)
	if _, err := iso.ReadAt(elf, int64(lba)*sectorSize); err != nil {
		return fmt.Errorf("reading %s: %w", elfName, err)
	}
	sum := sha1.Sum(elf)
	if hex.EncodeToString(sum[:]) != elfSHA1 {
		return fmt.Errorf("%s isn't the unmodified US release (SLUS-21242). Use a clean copy of the game", elfName)
	}
	if err := patchELF(elf, sel.Words()); err != nil {
		return err
	}

	if _, err := os.Stat(outPath); err == nil {
		return fmt.Errorf("%s already exists. Delete or rename it first", outPath)
	}
	// Write to a temporary name and rename at the end, so a stopped run never leaves a broken ISO
	// under the final name.
	tmpPath := outPath + ".part"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	fail := func(err error) error {
		out.Close()
		os.Remove(tmpPath)
		return err
	}
	if _, err := iso.Seek(0, io.SeekStart); err != nil {
		return fail(err)
	}
	var total int64
	if st, err := iso.Stat(); err == nil {
		total = st.Size()
	}
	if _, err := io.Copy(&progressWriter{w: out, total: total, report: progress}, iso); err != nil {
		return fail(err)
	}
	if _, err := out.WriteAt(elf, int64(lba)*sectorSize); err != nil {
		return fail(err)
	}
	if err := out.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, outPath)
}

// findFile looks up a file in the ISO 9660 root directory and returns its first sector and size.
func findFile(r io.ReaderAt, name string) (uint32, uint32, error) {
	pvd := make([]byte, sectorSize)
	if _, err := r.ReadAt(pvd, 16*sectorSize); err != nil {
		return 0, 0, errors.New("that doesn't look like a PS2 DVD image")
	}
	if pvd[0] != 1 || string(pvd[1:6]) != "CD001" {
		return 0, 0, errors.New("that doesn't look like a PS2 DVD image (no ISO 9660 volume)")
	}
	root := pvd[156:190]
	dirLBA := binary.LittleEndian.Uint32(root[2:])
	dirSize := binary.LittleEndian.Uint32(root[10:])
	dir := make([]byte, dirSize)
	if _, err := r.ReadAt(dir, int64(dirLBA)*sectorSize); err != nil {
		return 0, 0, err
	}
	for pos := 0; pos < len(dir); {
		n := int(dir[pos])
		if n == 0 { // records don't cross sectors: skip to the next one
			pos = (pos/sectorSize + 1) * sectorSize
			continue
		}
		if pos+n > len(dir) || n < 34 {
			break
		}
		rec := dir[pos : pos+n]
		id := string(rec[33 : 33+int(rec[32])])
		if i := strings.IndexByte(id, ';'); i >= 0 {
			id = id[:i]
		}
		if strings.EqualFold(id, name) {
			return binary.LittleEndian.Uint32(rec[2:]), binary.LittleEndian.Uint32(rec[10:]), nil
		}
		pos += n
	}
	return 0, 0, fmt.Errorf("%s isn't in this ISO. Is it Burnout Revenge (USA)?", name)
}

type phdr struct{ typ, offset, vaddr, paddr, filesz, memsz, flags, align uint32 }

func readPhdrs(elf []byte) []phdr {
	le := binary.LittleEndian
	off := le.Uint32(elf[0x1C:])
	n := int(le.Uint16(elf[0x2C:]))
	out := make([]phdr, n)
	for i := range out {
		b := elf[int(off)+32*i:]
		out[i] = phdr{le.Uint32(b), le.Uint32(b[4:]), le.Uint32(b[8:]), le.Uint32(b[12:]),
			le.Uint32(b[16:]), le.Uint32(b[20:]), le.Uint32(b[24:]), le.Uint32(b[28:])}
	}
	return out
}

type progressWriter struct {
	w           io.Writer
	done, total int64
	report      func(done, total int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.done += int64(n)
	if p.report != nil {
		p.report(p.done, p.total)
	}
	return n, err
}

// patchELF writes the selection's words. The mod's code goes into unused space inside the game's
// .data section (freeLo-freeHi), so the file's layout stays exactly as it was.
func patchELF(elf []byte, words []word) error {
	le := binary.LittleEndian
	segs := readPhdrs(elf)
	if len(segs) != 2 {
		return errors.New("unexpected program headers")
	}
	lo, ok1 := fileOffset(segs, freeLo)
	hi, ok2 := fileOffset(segs, freeHi-4)
	if !ok1 || !ok2 {
		return errors.New("the space for the mod isn't in the game file")
	}
	for _, b := range elf[lo : hi+4] {
		if b != 0 {
			return errors.New("the space for the mod isn't empty")
		}
	}
	for _, w := range words {
		off, ok := fileOffset(segs, w.addr)
		if !ok {
			return fmt.Errorf("address %08X isn't in the game file", w.addr)
		}
		le.PutUint32(elf[off:], w.value)
	}
	return nil
}

func fileOffset(segs []phdr, addr uint32) (uint32, bool) {
	for _, s := range segs {
		if s.typ == 1 && addr >= s.vaddr && addr+4 <= s.vaddr+s.filesz {
			return s.offset + addr - s.vaddr, true
		}
	}
	return 0, false
}
