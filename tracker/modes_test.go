package main

import (
	"encoding/binary"
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
		c           *Car
		race, crash bool
		want        string
	}{
		{both, true, true, "[BENCHED]"},
		{both, true, false, ""}, // benched for races, usable in crash junctions: real name
		{&Car{Lives: 0, CrashLives: 1, InGarage: true, InCrash: true}, true, false, "[RACE X]"},
		{&Car{Lives: 0, CrashLives: 1, InGarage: true, InCrash: true}, true, true, "[BENCHED]"},
		{&Car{Lives: 0, CrashLives: 0, InGarage: true, InCrash: true}, true, true, "[WRECKED]"},
		{&Car{Lives: 0, InGarage: true}, true, false, "[WRECKED]"},
		{&Car{Lives: 2, InGarage: true}, true, false, "[BENCHED]"},
	}
	for i, c := range cases {
		if got := blockedName(c.c, 12, c.race, c.crash); got != c.want {
			t.Errorf("case %d: %q, want %q", i, got, c.want)
		}
	}
	if got := blockedName(&Car{Lives: 2, InGarage: true}, 4, true, false); got != "[B]" {
		t.Errorf("short name: %q", got)
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
	found := false
	for _, a := range r.Achievements {
		found = found || a.ID == "comeback"
	}
	if !found {
		t.Fatal("reviving should unlock Back From the Dead")
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
