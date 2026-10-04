package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

type fakeMem struct{ ram []byte }

func (f *fakeMem) Connected() bool { return true }
func (f *fakeMem) Connect() bool   { return true }
func (f *fakeMem) Drop()           { panic("Drop called: a read failed inside Poll") }
func (f *fakeMem) Read(a uint32, n int) ([]byte, error) {
	return append([]byte(nil), f.ram[a:int(a)+n]...), nil
}
func (f *fakeMem) Write(a uint32, d []byte) error { copy(f.ram[a:], d); return nil }
func (f *fakeMem) Serial() string                 { return serialWanted }
func (f *fakeMem) HasPine() bool                  { return true }
func (f *fakeMem) w32(a uint32, v uint32)         { binary.LittleEndian.PutUint32(f.ram[a:], v) }
func (f *fakeMem) w64(a uint32, v uint64)         { binary.LittleEndian.PutUint64(f.ram[a:], v) }
func (f *fakeMem) text(a uint32, n int) string {
	u := make([]uint16, 0)
	for i := 0; i < n; i++ {
		c := binary.LittleEndian.Uint16(f.ram[int(a)+2*i:])
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u))
}

func TestKnownIDs(t *testing.T) {
	for label, want := range map[string]uint32{"K_01DH1E": 0x289D56A6, "HIGHEUCAR2S1": 0x4FC7E9CC,
		"LOWRASCAR2A": 0x0C15B361, "MEDIASCAR1S1": 0xE95210A2, "US_K1_V1_1E": 0xCB1E0087} {
		if got := textID(label); got != want {
			t.Errorf("textID(%s) = %08X, want %08X", label, got, want)
		}
	}
	if encodeLabel("K_01DH1E") != 0x8B906E4043D21000 || decodeLabel(0x8B906E4050071000) != "K_01DH3E" {
		t.Error("label packing doesn't match the game")
	}
}

func TestFullRun(t *testing.T) { runScenario(t, true) }

func TestFullRunWithoutSignal(t *testing.T) { runScenario(t, false) }

func runScenario(t *testing.T, signal bool) {
	f := &fakeMem{ram: make([]byte, 0x2000000)}
	f.w32(eventCount, 169)
	for i := 0; i < 169; i++ {
		f.ram[eventResults+i] = 0xFF
	}
	f.w64(eventIDs+6*8, encodeLabel("K_01DH1E"))
	f.w64(eventIDs+7*8, encodeLabel("K_01DH3E"))
	for a, v := range patchHooks {
		f.w32(a, v)
	}
	if !signal {
		f.w32(finishHook, 0x0C04CCF4) // the game's original call: no finished-event signal
	}
	pos := uint32(textLo + 0x100)
	add := func(label, text string) uint32 {
		f.w32(pos, textID(label))
		d := append(utf16le(text), 0, 0)
		copy(f.ram[pos+4:], d)
		addr := pos + 4
		pos = (pos + 4 + uint32(len(d)) + 3) &^ 3
		return addr
	}
	nixon := add("HIGHASCAR1S1", "NIXON SPECIAL")
	add("FILLER", "NEXT ENTRY")
	ea := add("HIGHEUCAR2S1", "EA RACER GT")
	add("K_01DH1E", "CRASH - DOCK FIGHT")
	add("K_01DH3E", "CRASH - DECONSTRUCTION SITE")
	f.w32(carouselList+0xBA4, 2)
	f.w64(carouselList, encodeLabel("HIGHASCAR1S1"))
	f.w64(carouselList+8, encodeLabel("HIGHEUCAR2S1"))

	clock := 100.0
	statePath := filepath.Join(t.TempDir(), "state.json")
	tr := NewTracker(f, statePath, func() float64 { return clock })
	deadCalled := false
	tr.OnRunDead = func() { deadCalled = true }
	tr.StartRun("Hard")
	tr.Poll()
	r := tr.run()
	if len(r.Cars) != 2 || r.Cars["HIGHASCAR1S1"].Name != "NIXON SPECIAL" {
		t.Fatalf("cars not registered from the carousel: %+v", r.Cars)
	}

	event := func(car string, idx int, medal, rating uint32, newByte int) {
		f.w64(selectedCar, encodeLabel(car))
		f.w32(resultPos, 0xFFFFFFFF)
		clock += 0.25
		tr.Poll()
		f.w32(lastMedal, medal)
		f.w32(lastRating, rating)
		if newByte >= 0 {
			f.ram[eventResults+idx] = byte(newByte)
		}
		f.w32(resultPos, 2)
		if signal {
			f.w32(finishCounter, binary.LittleEndian.Uint32(f.ram[finishCounter:])+1)
		}
		clock += 0.25
		tr.Poll()
		clock += 1.5
		tr.Poll()
		t.Log("  ->", tr.message)
	}

	event("HIGHASCAR1S1", 6, 2, 3, 2) // Silver + Awesome, Hard = 1 life
	if r.Cars["HIGHASCAR1S1"].Lives != 0 || r.CarsLost != 1 {
		t.Fatal("Nixon should be wrecked")
	}
	clock += 3
	tr.Poll()
	if binary.LittleEndian.Uint32(f.ram[deadTable:]) != 1 || decodeLabel(binary.LittleEndian.Uint64(f.ram[deadTable+8:])) != "HIGHASCAR1S1" {
		t.Fatal("dead table not written")
	}
	if got := f.text(nixon, 14); got != "[WRECKED]" {
		t.Fatalf("name = %q", got)
	}

	event("HIGHEUCAR2S1", 7, 3, 3, 4) // first-time Gold + Perfect
	if r.EventsWon != 1 || r.Cars["HIGHEUCAR2S1"].Lives != 1 {
		t.Fatal("win not counted")
	}
	event("HIGHEUCAR2S1", 7, 3, 3, -1) // replay of a perfected event
	if r.Cars["HIGHEUCAR2S1"].Lives != 0 || !strings.Contains(tr.message, "already perfected") {
		t.Fatalf("replay should cost a life: %s", tr.message)
	}
	if !deadCalled || !r.Dead {
		t.Fatal("run should be dead")
	}
	clock += 3
	tr.Poll()
	if binary.LittleEndian.Uint32(f.ram[deadTable:]) != 2 {
		t.Fatal("cars must stay locked until Grace")
	}

	played := r.EventsPlayed
	for i := 20; i < 40; i++ { // profile load
		f.ram[eventResults+i] = 3
	}
	clock += 0.25
	tr.Poll()
	if r.EventsPlayed != played {
		t.Fatal("profile load counted as an event")
	}

	tr.Grace()
	clock += 3
	tr.Poll()
	if f.text(nixon, 14) != "NIXON SPECIAL" || f.text(ea, 12) != "EA RACER GT" || binary.LittleEndian.Uint32(f.ram[deadTable:]) != 0 {
		t.Fatalf("grace should restore: %q %q", f.text(nixon, 14), f.text(ea, 12))
	}
	if binary.LittleEndian.Uint32(f.ram[nixon+28:]) != textID("FILLER") {
		t.Fatal("next text entry was overwritten")
	}
	event("HIGHEUCAR2S1", 6, 1, 1, 1)
	if r.EventsPlayed != played || !strings.Contains(tr.message, "not counted") {
		t.Fatal("grace events must not count")
	}

	if _, err := os.Stat(statePath); err != nil {
		t.Fatal("state not saved")
	}
	again := NewTracker(f, statePath, func() float64 { return clock })
	if again.run().EventsWon != 1 || !again.run().Grace {
		t.Fatal("state didn't reload")
	}
	snap := again.Snapshot()
	if snap["run"] == nil {
		t.Fatal("snapshot missing run")
	}
}

// The signal arrives a poll AFTER the saved results changed: the event must still be judged right.
func TestSignalAfterResults(t *testing.T) {
	f := &fakeMem{ram: make([]byte, 0x2000000)}
	f.w32(eventCount, 169)
	for i := 0; i < 169; i++ {
		f.ram[eventResults+i] = 0xFF
	}
	for a, v := range patchHooks {
		f.w32(a, v)
	}
	f.w64(eventIDs+7*8, encodeLabel("K_01DH3E"))
	f.w64(selectedCar, encodeLabel("HIGHEUCAR2S1"))
	f.w32(finishCounter, 5)
	clock := 10.0
	tr := NewTracker(f, filepath.Join(t.TempDir(), "s.json"), func() float64 { return clock })
	tr.StartRun("Medium")
	tr.Poll() // first look at the counter: resync only
	f.w32(lastMedal, 3)
	f.w32(lastRating, 3)
	f.ram[eventResults+7] = 4 // results first...
	clock += 0.25
	tr.Poll()
	f.w32(finishCounter, 6) // ...then the signal
	clock += 0.25
	tr.Poll()
	clock += 1.5
	tr.Poll()
	if tr.run().EventsWon != 1 || tr.run().EventsPlayed != 1 {
		t.Fatalf("expected one win, got %d/%d (%s)", tr.run().EventsWon, tr.run().EventsPlayed, tr.message)
	}
	// game restart: counter drops back to 0 -> no event
	f.w32(finishCounter, 0)
	clock += 0.25
	tr.Poll()
	clock += 2
	tr.Poll()
	if tr.run().EventsPlayed != 1 {
		t.Fatal("a game restart was counted as an event")
	}
}

// The text table is somewhere else entirely, and the saved run already holds IDs instead of names.
func TestTextTableMoved(t *testing.T) {
	f := &fakeMem{ram: make([]byte, 0x2000000)}
	f.w32(eventCount, 169)
	for i := 0; i < 169; i++ {
		f.ram[eventResults+i] = 0xFF
	}
	for a, v := range patchHooks {
		f.w32(a, v)
	}
	pos := uint32(0x01234560) // nowhere near the usual 0067xxxx
	add := func(label, text string) uint32 {
		f.w32(pos, textID(label))
		d := append(utf16le(text), 0, 0)
		copy(f.ram[pos+4:], d)
		addr := pos + 4
		pos = (pos + 4 + uint32(len(d)) + 3) &^ 3
		return addr
	}
	add("HIGHUSCAR1A", "FACTORY R160 ST")
	medius := add("MEDIUSCAR4A", "MODIFIED M-TYPE ST")
	add("HIGHEUCAR2S1", "EA RACER GT")
	add("K_01DH3E", "CRASH - DECONSTRUCTION SITE")
	add("K_01CDSR", "RACE - MOTOR CITY")
	f.w32(0x00400000, textID("HIGHEUCAR2S1")) // a lone false match elsewhere must be ignored
	copy(f.ram[0x00400004:], utf16le("AB"))
	f.w32(carouselList+0xBA4, 1)
	f.w64(carouselList, encodeLabel("HIGHUSCAR1A"))
	f.w32(crashCarousel+0xBA4, 1) // this one only shows up in a crash junction
	f.w64(crashCarousel, encodeLabel("MEDIUSCAR4A"))

	clock := 50.0
	path := filepath.Join(t.TempDir(), "s.json")
	tr := NewTracker(f, path, func() float64 { return clock })
	tr.StartRun("Hard")
	r := tr.run()
	// what the broken version saved: IDs everywhere, and one car already wrecked
	r.Cars["HIGHUSCAR1A"] = &Car{Name: "HIGHUSCAR1A", Lives: 1}
	r.Cars["MEDIUSCAR4A"] = &Car{Name: "MEDIUSCAR4A", Lives: 0}
	r.History = []HistoryEntry{
		{Event: "K_01DH3E", Car: "MEDIUSCAR4A", Result: "No medal + -", Counted: true},
		{Event: "K_01CDSR", Car: "HIGHUSCAR1A", Result: "Gold + Perfect", Won: true, Counted: true},
	}
	tr.Poll()
	if tr.textBase == 0 {
		t.Fatal("text table not found")
	}
	if r.Cars["HIGHUSCAR1A"].Name != "FACTORY R160 ST" || r.Cars["MEDIUSCAR4A"].Name != "MODIFIED M-TYPE ST" {
		t.Fatalf("car names not repaired: %+v %+v", r.Cars["HIGHUSCAR1A"], r.Cars["MEDIUSCAR4A"])
	}
	if r.History[0].Event != "CRASH - DECONSTRUCTION SITE" || r.History[0].Car != "MODIFIED M-TYPE ST" ||
		r.History[1].Event != "RACE - MOTOR CITY" || r.History[1].Car != "FACTORY R160 ST" {
		t.Fatalf("history not repaired: %+v", r.History)
	}
	if got := f.text(medius, 20); got != "[WRECKED]" {
		t.Fatalf("wrecked car at the moved table not renamed: %q", got)
	}
	t.Logf("found the text table at %08X; names: %s, %s", tr.textBase, r.Cars["HIGHUSCAR1A"].Name, r.History[0].Event)
}

// The Harder AI level comes from the .pnach's hook and marker, and is "Off" without the hook.
func TestAILevel(t *testing.T) {
	f := &fakeMem{ram: make([]byte, 0x2000000)}
	tr := NewTracker(f, filepath.Join(t.TempDir(), "run.json"), func() float64 { return 0 })
	tr.readAILevel()
	if got := tr.Snapshot()["ai_level"]; got != "Off" {
		t.Fatalf("no hook: ai_level = %v, want Off", got)
	}
	f.w32(aiMarker, 3)
	tr.readAILevel()
	if got := tr.Snapshot()["ai_level"]; got != "Off" {
		t.Fatalf("marker without hook: ai_level = %v, want Off", got)
	}
	f.w32(aiHook, aiHookOn)
	for marker, want := range map[uint32]string{1: "Easy", 2: "Medium", 3: "Hard", 7: "Off"} {
		f.w32(aiMarker, marker)
		tr.readAILevel()
		if got := tr.Snapshot()["ai_level"]; got != want {
			t.Errorf("marker %d: ai_level = %v, want %s", marker, got, want)
		}
	}
}
