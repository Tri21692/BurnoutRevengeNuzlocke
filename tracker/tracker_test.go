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
	f.w32(crashTableRef, crashTableRefOn) // patch with separate Race and Crash tables
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

	const obj = 0x01D00000 // the game's current car/event object
	f.w32(currentEvent, obj)
	event := func(car string, idx int, medal, rating uint32, newByte int) {
		f.w64(selectedCar, encodeLabel(car))
		f.w64(obj, encodeLabel(car))
		copy(f.ram[obj+0x18:obj+0x20], f.ram[eventIDs+uint32(idx)*8:eventIDs+uint32(idx)*8+8])
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

	// events 6 and 7 are Crash junctions (K_01DH1E, K_01DH3E): they use the Crash pool
	event("HIGHASCAR1S1", 6, 2, 3, 2) // Silver + Awesome, Hard = 1 life
	if c := r.Cars["HIGHASCAR1S1"]; c.CrashLives != 0 || c.Lives != 1 || r.CrashLost != 1 || r.CarsLost != 0 {
		t.Fatalf("Nixon should be wrecked for Crash only: %+v", c)
	}
	clock += 3
	tr.Poll()
	if binary.LittleEndian.Uint32(f.ram[crashTable:]) != 1 || decodeLabel(binary.LittleEndian.Uint64(f.ram[crashTable+8:])) != "HIGHASCAR1S1" {
		t.Fatal("crash table not written")
	}
	if binary.LittleEndian.Uint32(f.ram[deadTable:]) != 0 {
		t.Fatal("a Crash wreck must not block the car in the garage")
	}
	if got := f.text(nixon, 14); got != "[CRASH X]" {
		t.Fatalf("name = %q", got)
	}

	event("HIGHEUCAR2S1", 7, 3, 3, 4) // first-time Gold + Perfect
	if r.EventsWon != 1 || r.Cars["HIGHEUCAR2S1"].CrashLives != 1 {
		t.Fatal("win not counted")
	}
	event("HIGHEUCAR2S1", 7, 3, 3, -1) // replay of a perfected event
	if r.Cars["HIGHEUCAR2S1"].CrashLives != 0 || !strings.Contains(tr.message, "already perfected") {
		t.Fatalf("replay should cost a life: %s", tr.message)
	}
	if !deadCalled || !r.Dead {
		t.Fatal("run should be dead: every car is wrecked in the Crash pool")
	}
	clock += 3
	tr.Poll()
	if binary.LittleEndian.Uint32(f.ram[crashTable:]) != 2 {
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
	if f.text(nixon, 14) != "NIXON SPECIAL" || f.text(ea, 12) != "EA RACER GT" || binary.LittleEndian.Uint32(f.ram[crashTable:]) != 0 {
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
	// patched ISO: different hook, level word in the mod's section
	f.w32(aiMarker, 0)
	f.w32(aiHook, aiHookISO)
	f.w32(aiLevelISO, 2)
	tr.readAILevel()
	if got := tr.Snapshot()["ai_level"]; got != "Medium" {
		t.Errorf("patched ISO: ai_level = %v, want Medium", got)
	}
}

// The patch counts as active with either the .pnach's hooks or the patched ISO's.
func TestPatchDetection(t *testing.T) {
	for name, hooks := range map[string]map[uint32]uint32{"pnach": patchHooks, "iso": isoHooks} {
		f := &fakeMem{ram: make([]byte, 0x2000000)}
		tr := NewTracker(f, filepath.Join(t.TempDir(), "run.json"), func() float64 { return 0 })
		if tr.hooksMatch(patchHooks) || tr.hooksMatch(isoHooks) {
			t.Fatalf("%s: empty memory counted as patched", name)
		}
		for a, v := range hooks {
			f.w32(a, v)
		}
		if !tr.hooksMatch(hooks) {
			t.Errorf("%s: hooks not recognised", name)
		}
	}
}

// Race events use the Race pool, replays are named from the game's current event, and the car names
// show which pool a car is wrecked in.
func TestPoolsAndReplayNames(t *testing.T) {
	f := &fakeMem{ram: make([]byte, 0x2000000)}
	f.w32(eventCount, 169)
	for i := 0; i < 169; i++ {
		f.ram[eventResults+i] = 0xFF
	}
	f.w64(eventIDs+3*8, encodeLabel("K_01RDSF")) // a race
	for a, v := range patchHooks {
		f.w32(a, v)
	}
	f.w32(crashTableRef, crashTableRefOn)
	const obj = 0x01D00000 // the game's current car/event object
	f.w32(currentEvent, obj)
	clock := 100.0
	tr := NewTracker(f, filepath.Join(t.TempDir(), "s.json"), func() float64 { return clock })
	tr.StartRun("Medium")
	r := tr.run()
	tr.registerCar("HIGHASCAR1S1", "NIXON SPECIAL")
	tr.registerCar("HIGHEUCAR2S1", "EA RACER GT")
	finish := func(car, event string, idx, newByte int) {
		f.w64(selectedCar, encodeLabel(car))
		f.w64(obj, encodeLabel(car))
		f.w64(obj+0x18, encodeLabel(event))
		f.w32(lastMedal, 2)
		f.w32(lastRating, 2)
		if newByte >= 0 {
			f.ram[eventResults+idx] = byte(newByte)
		}
		f.w32(finishCounter, binary.LittleEndian.Uint32(f.ram[finishCounter:])+1)
		clock += 0.25
		tr.Poll()
		clock += 1.5
		tr.Poll()
	}
	clock += 0.25
	tr.Poll()                                // first look: sync the counter
	finish("HIGHASCAR1S1", "K_01RDSF", 3, 2) // race, first time: result byte changes
	if c := r.Cars["HIGHASCAR1S1"]; c.Lives != 1 || c.CrashLives != 2 {
		t.Fatalf("race should cost a Race life: %+v", c)
	}
	finish("HIGHASCAR1S1", "K_01RDSF", 3, -1) // replay: no result change, named from the current event
	h := r.History[len(r.History)-1]
	if h.Event != "K_01RDSF" || h.Crash {
		t.Fatalf("replay should be named and counted as a race: %+v", h)
	}
	if r.Cars["HIGHASCAR1S1"].Lives != 0 || r.CarsLost != 1 || r.Dead {
		t.Fatalf("Nixon should be wrecked for races only, run still alive: %+v", r)
	}
	finish("HIGHEUCAR2S1", "K_02DH1E", 9, -1) // a crash junction replay
	if c := r.Cars["HIGHEUCAR2S1"]; c.CrashLives != 1 || c.Lives != 2 || !r.History[len(r.History)-1].Crash {
		t.Fatalf("crash replay should cost a Crash life: %+v", c)
	}
	clock += 3
	tr.Poll()
	if n := binary.LittleEndian.Uint32(f.ram[deadTable:]); n != 1 {
		t.Fatalf("garage table should hold Nixon only, has %d", n)
	}
	if n := binary.LittleEndian.Uint32(f.ram[crashTable:]); n != 0 {
		t.Fatalf("crash table should be empty, has %d", n)
	}
	finish("HIGHEUCAR2S1", "K_01RDSF", 3, -1)
	finish("HIGHEUCAR2S1", "K_01RDSF", 3, -1)
	if !r.Dead {
		t.Fatal("every car wrecked in the Race pool: the run should be dead")
	}
}

// With a patch from before V1.1 the crash junction block reads the garage table, so the garage table
// must hold the cars wrecked in either pool.
func TestOldPatchSharesTable(t *testing.T) {
	f := &fakeMem{ram: make([]byte, 0x2000000)}
	tr := NewTracker(f, filepath.Join(t.TempDir(), "s.json"), func() float64 { return 0 })
	tr.StartRun("Hard")
	tr.registerCar("HIGHASCAR1S1", "NIXON SPECIAL")
	tr.run().Cars["HIGHASCAR1S1"].CrashLives = 0
	tr.slowChecks()
	if n := binary.LittleEndian.Uint32(f.ram[deadTable:]); n != 1 || !tr.patchOld {
		t.Fatalf("old patch: garage table should include Crash wrecks (has %d)", n)
	}
}

// Runs saved before the two pools keep each car's lives in both.
func TestOldRunMigrates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	old := `{"run":{"active":true,"difficulty":"Easy","lives_start":3,"cars_lost":1,` +
		`"cars":{"HIGHASCAR1S1":{"name":"NIXON SPECIAL","lives":2},"HIGHEUCAR2S1":{"name":"EA","lives":0}}}}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	tr := NewTracker(&fakeMem{ram: make([]byte, 0x2000000)}, path, func() float64 { return 0 })
	r := tr.run()
	if r.Cars["HIGHASCAR1S1"].CrashLives != 2 || r.Cars["HIGHEUCAR2S1"].CrashLives != 0 || r.CrashLost != 1 || !r.Pools {
		t.Fatalf("migration: %+v %+v", r.Cars["HIGHASCAR1S1"], r)
	}
}
