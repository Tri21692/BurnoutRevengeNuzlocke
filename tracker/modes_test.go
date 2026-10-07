package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// modeRig is a tracker on a fake game with a garage, a crash junction car select and a replayable event.
type modeRig struct {
	t     *testing.T
	f     *fakeMem
	tr    *Tracker
	clock float64
	dir   string
}

const rigObj = 0x01D00000

func newModeRig(t *testing.T, difficulty string, garage, crash []string) *modeRig {
	f := &fakeMem{ram: make([]byte, 0x2000000)}
	f.w32(eventCount, 169)
	for i := 0; i < 169; i++ {
		f.ram[eventResults+i] = 0xFF
	}
	for a, v := range patchHooks {
		f.w32(a, v)
	}
	f.w32(crashTableRef, crashTableRefOn)
	f.w32(currentEvent, rigObj)
	m := &modeRig{t: t, f: f, clock: 100, dir: t.TempDir()}
	m.tr = NewTracker(f, filepath.Join(m.dir, "s.json"), func() float64 { return m.clock })
	m.tr.StartRun(difficulty)
	m.list(carouselList, garage)
	m.list(crashCarousel, crash)
	m.tick()
	return m
}

func (m *modeRig) list(at uint32, labels []string) {
	m.f.w32(at+0xBA4, uint32(len(labels)))
	for i, l := range labels {
		m.f.w64(at+uint32(8*i), encodeLabel(l))
	}
}

func (m *modeRig) tick() { m.clock += 3; m.tr.Poll() }

// finish plays an event with car: a win (Gold + Awesome) or a loss (Bronze).
func (m *modeRig) finish(car string, crash, win bool) {
	event := "K_01RDSF"
	if crash {
		event = "K_01DH1E"
	}
	m.f.w64(selectedCar, encodeLabel(car))
	m.f.w64(rigObj, encodeLabel(car))
	m.f.w64(rigObj+0x18, encodeLabel(event))
	medal, rating := uint32(1), uint32(0)
	if win {
		medal, rating = 3, 2
	}
	m.f.w32(lastMedal, medal)
	m.f.w32(lastRating, rating)
	m.f.w32(finishCounter, binary.LittleEndian.Uint32(m.f.ram[finishCounter:])+1)
	m.clock += 0.25
	m.tr.Poll()
	m.clock += 1.5
	m.tr.Poll()
	m.tick()
}

func (m *modeRig) table(at uint32) map[string]bool {
	n := binary.LittleEndian.Uint32(m.f.ram[at:])
	out := map[string]bool{}
	for i := uint32(0); i < n; i++ {
		out[decodeLabel(binary.LittleEndian.Uint64(m.f.ram[at+8+8*i:]))] = true
	}
	return out
}

var garage5 = []string{"HIGHUSCAR1A", "HIGHUSCAR1B", "HIGHUSCAR1C", "HIGHUSCAR2A", "HIGHUSCAR2B"}
var crash3 = []string{"MEDIUSCAR1A", "MEDIUSCAR1B", "MEDIUSCAR1C"}

func TestLimitedSelection(t *testing.T) {
	m := newModeRig(t, "Easy", garage5, nil)
	r := m.tr.run()
	if len(r.RacePick) != 0 {
		t.Fatal("no picks while the mode is off")
	}
	m.f.w32(limitedMarker, 1)
	m.list(crashCarousel, crash3)
	m.tick()
	if len(r.RacePick) != 2 || len(r.CrashPick) != 2 {
		t.Fatalf("pairs not picked: %v %v", r.RacePick, r.CrashPick)
	}
	garage := m.table(deadTable)
	if len(garage) != 3 || garage[r.RacePick[0]] || garage[r.RacePick[1]] {
		t.Fatalf("the 3 cars left out should be benched in the garage: %v, picks %v", garage, r.RacePick)
	}
	if c := m.table(crashTable); len(c) != 1 || c[r.CrashPick[0]] || c[r.CrashPick[1]] {
		t.Fatalf("the crash car left out should be benched: %v", c)
	}
	// every event: a new pair, never one of the cars from the last pair
	for i := 0; i < 6; i++ {
		prev := append([]string(nil), r.RacePick...)
		m.finish(prev[0], false, true)
		if len(r.RacePick) != 2 || contains(prev, r.RacePick[0]) || contains(prev, r.RacePick[1]) {
			t.Fatalf("event %d: pair %v repeats one of %v", i, r.RacePick, prev)
		}
	}
	// wrecked cars are never picked; with 3 usable cars one of the last pair has to come back
	r.Cars["HIGHUSCAR2A"].Lives = 0
	r.Cars["HIGHUSCAR2B"].Lives = 0
	r.CarsLost = 2
	m.tick() // wrecked picks are replaced
	for i := 0; i < 4; i++ {
		prev := append([]string(nil), r.RacePick...)
		m.finish(r.RacePick[0], false, true)
		if contains(r.RacePick, "HIGHUSCAR2A") || contains(r.RacePick, "HIGHUSCAR2B") || len(r.RacePick) != 2 {
			t.Fatalf("pair %v has a wrecked car", r.RacePick)
		}
		fresh := 0
		for _, l := range r.RacePick {
			if !contains(prev, l) {
				fresh++
			}
		}
		if fresh != 1 {
			t.Fatalf("with 3 usable cars the only car not picked last time must be picked: %v after %v", r.RacePick, prev)
		}
	}
	// a pick wrecked between events (here by hand) is replaced at once
	gone := r.RacePick[0]
	r.Cars[gone].Lives = 0
	m.tick()
	if contains(r.RacePick, gone) || len(r.RacePick) != 2 {
		t.Fatalf("wrecked pick %s not replaced: %v", gone, r.RacePick)
	}
	// Grace: nothing is benched
	m.tr.Grace()
	m.tick()
	if g := m.table(deadTable); len(g) != 0 {
		t.Fatalf("Grace should unbench everything: %v", g)
	}
}

func TestBlockedNames(t *testing.T) {
	both := &Car{Lives: 1, CrashLives: 1, InGarage: true, InCrash: true}
	cases := []struct {
		c                 *Car
		race, crash, inCJ bool
		want              string
	}{
		{both, true, true, false, "[BENCHED]"},
		{both, true, true, true, "[BENCHED]"},
		{both, true, false, false, "[BENCHED]"}, // benched for races: the garage can't pick it
		{both, true, false, true, ""},           // ...but the crash junction select can
		{both, false, true, false, ""},          // a race pick benched for crash junctions, in the garage
		{both, false, true, true, "[BENCHED]"},  // ...and in a crash junction's car select
		{&Car{Lives: 0, CrashLives: 1, InGarage: true, InCrash: true}, true, false, true, "[RACE X]"},
		{&Car{Lives: 0, CrashLives: 1, InGarage: true, InCrash: true}, true, true, true, "[BENCHED]"},
		{&Car{Lives: 0, CrashLives: 1, InGarage: true, InCrash: true}, true, true, false, "[RACE X]"},
		{&Car{Lives: 0, CrashLives: 0, InGarage: true, InCrash: true}, true, true, false, "[WRECKED]"},
		{&Car{Lives: 0, InGarage: true}, true, false, false, "[WRECKED]"},
		{&Car{Lives: 2, InGarage: true}, true, false, false, "[BENCHED]"},
		{&Car{CrashLives: 2, InCrash: true}, false, true, true, "[BENCHED]"}, // crash-only car
	}
	for i, c := range cases {
		if got := blockedName(c.c, 12, c.race, c.crash, c.inCJ); got != c.want {
			t.Errorf("case %d: %q, want %q", i, got, c.want)
		}
	}
	if got := blockedName(&Car{Lives: 2, InGarage: true}, 4, true, false, false); got != "[B]" {
		t.Errorf("short name: %q", got)
	}
}

// In a crash junction's car select every car but the two crash picks shows [BENCHED], even the race picks.
func TestBenchedNamesInCrashJunction(t *testing.T) {
	m := newModeRig(t, "Easy", nil, nil)
	pos := uint32(0x01234560)
	addrs := map[string]uint32{}
	add := func(label, text string) {
		m.f.w32(pos, textID(label))
		d := append(utf16le(text), 0, 0)
		copy(m.f.ram[pos+4:], d)
		addrs[label] = pos + 4
		pos = (pos + 4 + uint32(len(d)) + 3) &^ 3
	}
	for i, l := range garage5 {
		add(l, fmt.Sprintf("CAR NUMBER %d", i))
	}
	add("HIGHEUCAR2S1", "EA RACER GT")
	add("HIGHASCAR1S1", "NIXON")
	add("K_01DH1E", "CRASH - DOCK FIGHT")
	add("K_01DH3E", "CRASH - DECON")
	m.list(carouselList, garage5)
	m.list(crashCarousel, garage5) // every car can also crash
	m.f.w32(limitedMarker, 1)
	m.f.w64(rigObj+0x18, encodeLabel("K_01DH1E")) // a crash junction is chosen
	for i := 0; i < 5; i++ {
		m.tick()
	}
	r := m.tr.run()
	benched := 0
	for _, l := range garage5 {
		got := m.f.text(addrs[l], 30)
		if contains(r.CrashPick, l) == (got == "[BENCHED]") {
			t.Errorf("%s: %q (crash picks %v, race picks %v)", l, got, r.CrashPick, r.RacePick)
		}
		if got == "[BENCHED]" {
			benched++
		}
	}
	if benched != 3 {
		t.Errorf("%d benched, want 3", benched)
	}
	m.f.w64(rigObj+0x18, encodeLabel("K_01RDSF")) // a race is chosen: named for the garage
	m.tick()
	for _, l := range garage5 {
		if got := m.f.text(addrs[l], 30); contains(r.RacePick, l) == (got == "[BENCHED]") {
			t.Errorf("garage: %s: %q (race picks %v)", l, got, r.RacePick)
		}
	}
}

func TestReviveTokens(t *testing.T) {
	m := newModeRig(t, "Easy", []string{"HIGHUSCAR1A", "HIGHUSCAR1B"}, nil)
	r := m.tr.run()
	m.f.w32(reviveMarker, 1)
	m.tick()
	for i := 0; i < 3; i++ {
		m.finish("HIGHUSCAR1A", false, true)
	}
	if r.Tokens != 1 || r.TokensEarned != 1 {
		t.Fatalf("3 wins in a row on Easy should earn a token: %d", r.Tokens)
	}
	for i := 0; i < 3; i++ { // HIGHUSCAR1B: 3 Race lives gone
		m.finish("HIGHUSCAR1B", false, false)
	}
	if r.Cars["HIGHUSCAR1B"].Lives != 0 || r.CarsLost != 1 {
		t.Fatal("HIGHUSCAR1B should be wrecked")
	}
	if !m.tr.Revive("HIGHUSCAR1B", false) {
		t.Fatal("revive refused")
	}
	if c := r.Cars["HIGHUSCAR1B"]; c.Lives != 1 || r.CarsLost != 0 || r.Tokens != 0 || r.Revived != 1 {
		t.Fatalf("revive should give 1 life back and spend the token: %+v lost %d", c, r.CarsLost)
	}
	if m.tr.Revive("HIGHUSCAR1A", false) {
		t.Fatal("no token left, and the car isn't wrecked")
	}
	if len(r.Achievements) != 0 || !m.tr.achievementsLocked() {
		t.Fatalf("a run with Revive tokens earns no achievements: %+v", r.Achievements)
	}
	// a held token saves the run when the last car goes
	for i := 0; i < 3; i++ {
		m.finish("HIGHUSCAR1A", false, true)
	}
	r.Cars["HIGHUSCAR1A"].Lives = 1
	m.finish("HIGHUSCAR1B", false, false) // 1B wrecked again
	m.finish("HIGHUSCAR1A", false, false) // and the last car: the token steps in
	if r.Dead || r.Tokens != 0 || r.Cars["HIGHUSCAR1A"].Lives != 1 {
		t.Fatalf("the held token should have saved the run: dead %v tokens %d", r.Dead, r.Tokens)
	}
	m.finish("HIGHUSCAR1A", false, false)
	if !r.Dead {
		t.Fatal("no token left: the run should end")
	}
}

func TestTokensNeedTheMode(t *testing.T) {
	m := newModeRig(t, "Easy", []string{"HIGHUSCAR1A"}, nil)
	for i := 0; i < 6; i++ {
		m.finish("HIGHUSCAR1A", false, true)
	}
	if r := m.tr.run(); r.Tokens != 0 {
		t.Fatal("tokens without the mode")
	}
}

func TestAchievementsAndSummary(t *testing.T) {
	m := newModeRig(t, "Medium", []string{"HIGHUSCAR1A", "HIGHUSCAR1B"}, nil)
	r := m.tr.run()
	for i := 0; i < 5; i++ {
		m.finish("HIGHUSCAR1A", false, true)
	}
	if len(r.Achievements) != 1 || r.Achievements[0].ID != "streak5" {
		t.Fatalf("5 wins in a row should unlock Hot Streak: %+v", r.Achievements)
	}
	for i := 0; i < 2; i++ { // Medium: 2 Race lives each
		m.finish("HIGHUSCAR1A", false, false)
		m.finish("HIGHUSCAR1B", false, false)
	}
	if !r.Dead || r.EndedBy == "" || r.SummaryFile == "" {
		t.Fatalf("the run should be over with a summary: dead %v, ended %q, file %q", r.Dead, r.EndedBy, r.SummaryFile)
	}
	data, err := os.ReadFile(filepath.Join(m.dir, r.SummaryFile))
	if err != nil || len(data) < 100 {
		t.Fatalf("summary file: %v", err)
	}
	sum := m.tr.Snapshot()["run"].(map[string]any)["summary"].(map[string]any)
	cars := sum["cars"].([]map[string]any)
	if len(cars) != 2 || cars[0]["events"] != 7 || cars[0]["wins"] != 5 || cars[0]["gold"] != 5 {
		t.Fatalf("summary cars: %+v", cars)
	}
	t.Logf("summary:\n%s", data)
}

// A crash junction gets its location from the race events that share its location letter.
func TestJunctionLocation(t *testing.T) {
	m := newModeRig(t, "Easy", []string{"HIGHUSCAR1A"}, []string{"HIGHUSCAR1A"})
	pos := uint32(0x01234560)
	add := func(label, text string) {
		m.f.w32(pos, textID(label))
		d := append(utf16le(text), 0, 0)
		copy(m.f.ram[pos+4:], d)
		pos = (pos + 4 + uint32(len(d)) + 3) &^ 3
	}
	for i, ev := range [][2]string{{"K_01CDSR", "RACE - MOTOR CITY"}, {"K_01TDLR", "TRAFFIC ATTACK - MOTOR CITY"},
		{"K_01RLLR", "ROAD RAGE - ANGEL VALLEY"}, {"K_01DH1E", "CRASH - DOCK FIGHT"}, {"K_01LH6E", "CRASH - THE FALLS"}} {
		m.f.w64(eventIDs+uint32(8*i), encodeLabel(ev[0]))
		add(ev[0], ev[1])
	}
	add("HIGHUSCAR1A", "FACTORY R160 ST")
	add("HIGHEUCAR2S1", "EA RACER GT")
	add("HIGHASCAR1S1", "NIXON")
	add("K_01DH3E", "CRASH - DECON")
	for i := 0; i < 5; i++ {
		m.tick()
	}
	m.finish("HIGHUSCAR1A", true, true) // K_01DH1E
	if h := m.tr.run().History; len(h) != 1 || h[0].Where != "MOTOR CITY" || !h[0].Crash {
		t.Fatalf("Dock Fight should be a crash junction at Motor City: %+v", h)
	}
	m.f.w64(rigObj+0x18, encodeLabel("K_01LH6E"))
	m.f.w64(selectedCar, encodeLabel("HIGHUSCAR1A"))
	m.f.w32(finishCounter, binary.LittleEndian.Uint32(m.f.ram[finishCounter:])+1)
	m.clock += 0.25
	m.tr.Poll()
	m.clock += 1.5
	m.tr.Poll()
	if h := m.tr.run().History; len(h) != 2 || h[1].Where != "ANGEL VALLEY" || !h[1].Crash || h[1].Event != "CRASH - THE FALLS" {
		t.Fatalf("K_01LH6E should be a crash junction at Angel Valley: %+v", h[len(h)-1])
	}
}

// Any mode (or Unlock all cars) locks the achievements; a standard run earns them.
func TestAchievementsLockedByModes(t *testing.T) {
	for _, mode := range []uint32{limitedMarker, reviveMarker, rouletteMarker, allCarsPatch} {
		m := newModeRig(t, "Easy", []string{"HIGHUSCAR1A"}, nil)
		if mode == allCarsPatch {
			m.f.w32(allCarsPatch, allCarsPatchOn)
		} else {
			m.f.w32(mode, 1)
		}
		m.tick()
		for i := 0; i < 5; i++ {
			m.finish("HIGHUSCAR1A", false, true)
		}
		if r := m.tr.run(); len(r.Achievements) != 0 || !m.tr.achievementsLocked() {
			t.Errorf("mode %08X: achievements %+v", mode, r.Achievements)
		}
	}
}

// A Burning Lap or Preview loads only its own car into the garage: if you own it and it isn't one of
// the picks, it still isn't benched, so the event can be driven. The pair applies again afterwards.
func TestLimitedSelectionFixedCar(t *testing.T) {
	m := newModeRig(t, "Easy", garage5, nil)
	m.f.w32(limitedMarker, 1)
	m.tick()
	r := m.tr.run()
	var other string
	for _, l := range garage5 {
		if !contains(r.RacePick, l) {
			other = l
			break
		}
	}
	if !m.table(deadTable)[other] {
		t.Fatalf("%s should be benched in the normal garage", other)
	}
	m.list(carouselList, []string{other}) // the event's fixed car
	m.tick()
	if m.table(deadTable)[other] {
		t.Fatalf("%s is the event's only car and mustn't be benched", other)
	}
	m.list(carouselList, garage5) // back to the full garage
	m.tick()
	if !m.table(deadTable)[other] && !contains(r.RacePick, other) {
		t.Fatalf("%s should be benched again", other)
	}
}
