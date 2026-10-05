// nuzlocke_isopatch builds a patched copy of a Burnout Revenge (USA, SLUS-21242) ISO with the Nuzlocke
// mod built in: the dead-car block, the finished-event signal, the pause-menu Retry/Quit block and one
// Harder AI level, optionally with aggressive opponents, widescreen 16:9 and 60 FPS menus (the last two
// by SuperType1/remco). The original ISO is never modified.
//
// Usage: nuzlocke_isopatch.exe [game.iso] [easy|medium|hard] [widescreen] [60fps] [aggressive]
// (or drag the ISO onto the .exe and answer the questions)
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
	err := run(os.Args[1:], bufio.NewReader(os.Stdin))
	if err != nil {
		fmt.Println()
		fmt.Println("Error:", err)
	}
	if len(os.Args) < 3 {
		fmt.Println()
		fmt.Print("Press Enter to close.")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
	if err != nil {
		os.Exit(1)
	}
}

func run(args []string, in *bufio.Reader) error {
	fmt.Println("Burnout Revenge Nuzlocke - ISO patcher v" + version)
	fmt.Println()
	isoPath := ""
	if len(args) > 0 {
		isoPath = args[0]
	} else {
		fmt.Print("Drag your Burnout Revenge ISO here (or type its path), then press Enter: ")
		line, _ := in.ReadString('\n')
		isoPath = line
	}
	isoPath = strings.Trim(strings.TrimSpace(isoPath), `"`)
	if isoPath == "" {
		return errors.New("no ISO given")
	}

	level := -1
	if len(args) > 1 {
		level = levelIndex(args[1])
	}
	for level < 0 {
		fmt.Println("Harder AI level:  1 = Easy   2 = Medium   3 = Hard")
		fmt.Print("Choose 1, 2 or 3: ")
		line, err := in.ReadString('\n')
		level = levelIndex(strings.TrimSpace(line))
		if level < 0 && err != nil {
			return errors.New("no level chosen")
		}
	}

	// Extras: given on the command line, or asked for when the level was chosen interactively.
	var extras []int
	tags := map[string]string{"widescreen": "16-9", "fps60": "60 FPS", "aggressive": "aggressive"}
	for i, o := range options {
		want := false
		if len(args) > 1 {
			for _, a := range args[2:] {
				a = strings.ToLower(a)
				want = want || a == o.key || (o.key == "fps60" && a == "60fps") || (o.key == "widescreen" && a == "ws")
			}
		} else {
			fmt.Printf("Add %s? (y/n): ", o.name)
			line, _ := in.ReadString('\n')
			want = strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "y")
		}
		if want {
			extras = append(extras, i)
		}
	}

	name := "Nuzlocke " + levels[level].name
	for _, i := range extras {
		name += ", " + tags[options[i].key]
	}
	ext := filepath.Ext(isoPath)
	outPath := strings.TrimSuffix(isoPath, ext) + " (" + name + ")" + ext
	if err := patchISO(isoPath, outPath, level, extras); err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("Done:", outPath)
	fmt.Println("Play that ISO. Don't also enable the Nuzlocke .pnach for it.")
	return nil
}

func levelIndex(s string) int {
	switch strings.ToLower(s) {
	case "1", "easy":
		return 0
	case "2", "medium":
		return 1
	case "3", "hard":
		return 2
	}
	return -1
}

// patchISO copies the ISO to outPath and replaces SLUS_212.42 inside the copy with the patched one.
func patchISO(isoPath, outPath string, level int, extras []int) error {
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
	if err := patchELF(elf, level, extras); err != nil {
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
	fmt.Printf("Writing %s (%s AI)...\n", filepath.Base(outPath), levels[level].name)
	if _, err := iso.Seek(0, io.SeekStart); err != nil {
		return fail(err)
	}
	if _, err := io.Copy(out, iso); err != nil {
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

// patchELF applies the chosen level's words and those of the chosen extras. The mod's code goes into unused space inside the game's
// .data section (freeLo-freeHi), so the file's layout stays exactly as it was.
func patchELF(elf []byte, level int, extras []int) error {
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
	words := levels[level].words
	for _, i := range extras {
		words = append(append([]word(nil), words...), options[i].words...)
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
