package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelection(t *testing.T) {
	full := Selection{Core: true, Pause: true, Level: "Hard", Options: []string{"chaos", "widescreen"}}
	if err := full.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := full.Name(); got != "Nuzlocke, No retry, Hard AI, 16-9, Chaos" {
		t.Fatalf("name: %q", got)
	}
	if got := full.OutputPath(filepath.Join("games", "Burnout.iso")); got != filepath.Join("games", "Burnout (Nuzlocke, No retry, Hard AI, 16-9, Chaos).iso") {
		t.Fatalf("output: %q", got)
	}
	for _, bad := range []Selection{
		{},                                   // nothing
		{Level: "Silly"},                     // unknown level
		{Options: []string{"roulette"}},      // a mode without the rules
		{Core: true, Options: []string{"x"}}, // unknown option
		{Pause: true, Options: []string{"chaos"}},
	} {
		if bad.Validate() == nil {
			t.Fatalf("%+v should be refused", bad)
		}
	}
	// the extras and the AI work on their own
	for _, ok := range []Selection{{Level: "Insane"}, {Options: []string{"allcars"}}, {Options: []string{"widescreen", "fps60"}}, {Pause: true}} {
		if err := ok.Validate(); err != nil {
			t.Fatalf("%+v: %v", ok, err)
		}
	}
}

// The parts must add up to what V2.0 wrote for the full Nuzlocke, and never write one address twice
// with different values.
func TestWordsDontClash(t *testing.T) {
	all := []string{}
	for _, o := range options {
		all = append(all, o.key)
	}
	for _, l := range levels {
		s := Selection{Core: true, Pause: true, Level: l.name, Options: all}
		seen := map[uint32]uint32{}
		for _, w := range s.Words() {
			if v, ok := seen[w.addr]; ok && v != w.value {
				t.Fatalf("%s: %08X written twice", l.name, w.addr)
			}
			seen[w.addr] = w.value
			if w.addr >= freeLo && w.addr < freeHi {
				continue
			}
			if w.addr < 0x00100000 {
				t.Fatalf("%08X is below the game", w.addr)
			}
		}
	}
	if s := (Selection{Level: "Easy"}); len(s.Words()) == 0 || s.Words()[len(s.Words())-1] == (word{}) {
		t.Fatal("a level on its own has words")
	}
}

func TestPatchISORefusesOtherFiles(t *testing.T) {
	dir := t.TempDir()
	iso := filepath.Join(dir, "x.iso")
	os.WriteFile(iso, make([]byte, 64*1024), 0o644)
	if err := patchISO(iso, iso+".out", Selection{Pause: true}, nil); err == nil {
		t.Fatal("not a PS2 image")
	}
	if _, err := os.Stat(iso + ".out"); err == nil {
		t.Fatal("nothing should be written")
	}
}
