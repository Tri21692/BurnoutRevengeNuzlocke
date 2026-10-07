package main

import (
	"encoding/binary"
	"testing"
)

// playEvent finishes a given event with car: a win (Gold + Awesome) or a loss (Bronze).
func (m *modeRig) playEvent(car, event string, win bool) {
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

func TestEventRoulette(t *testing.T) {
	m := newModeRig(t, "Easy", []string{"HIGHUSCAR1A", "HIGHUSCAR1B"}, nil)
	events := []string{"K_01CDSR", "K_01TFLR", "K_01DH1E", "K_02CRLF", "K_02RH5E", "K_03THLF"}
	for i := 0; i < 169; i++ { // the rest of the list stays empty
		m.f.w64(eventIDs+uint32(8*i), 0)
	}
	for i, l := range events {
		m.f.w64(eventIDs+uint32(8*i), encodeLabel(l))
		m.f.ram[eventResults+i] = 0xFF
	}
	r := m.tr.run()
	m.tick()
	if r.Roulette != "" {
		t.Fatal("no roulette while the mode is off")
	}
	m.f.w32(rouletteMarker, 1)
	m.tick()
	if r.Roulette == "" || r.Rerolls != 0 {
		t.Fatalf("first roll should be an event, with no reroll yet: %q, %d", r.Roulette, r.Rerolls)
	}
	if got := binary.LittleEndian.Uint64(m.f.ram[rouletteTarget:]); got != encodeLabel(r.Roulette) {
		t.Fatalf("the patch should be told the pick: %x", got)
	}
	// playing something else is a loss, even with a Gold, and the roulette event stays
	want := r.Roulette
	other := "K_01CDSR"
	if want == other {
		other = "K_01TFLR"
	}
	m.playEvent("HIGHUSCAR1A", other, true)
	if h := r.History[len(r.History)-1]; h.Won || r.Cars["HIGHUSCAR1A"].Lives != 2 || r.Roulette != want {
		t.Fatalf("off-roulette event should cost a life and keep the roulette: %+v, %q", h, r.Roulette)
	}
	// winning every open event: each roll is one not won yet, never the one just played
	seen := map[string]bool{}
	for i := 0; i < 6; i++ {
		ev := r.Roulette
		if seen[ev] {
			t.Fatalf("rolled %s again before every open event was won", ev)
		}
		seen[ev] = true
		car := "HIGHUSCAR1A"
		if isCrashEvent(ev) {
			m.list(crashCarousel, []string{"HIGHUSCAR1B"})
			car = "HIGHUSCAR1B"
		}
		m.playEvent(car, ev, true)
		if r.Roulette == ev {
			t.Fatalf("rolled %s twice in a row", ev)
		}
	}
	if len(seen) != 6 {
		t.Fatalf("every event should come up once: %v", seen)
	}
	// rerolls: one for every 5 wins (the off-roulette event was a loss)
	if r.RouletteWins != 6 || r.Rerolls != 1 {
		t.Fatalf("roulette wins %d, rerolls %d; want 6 and 1", r.RouletteWins, r.Rerolls)
	}
	m.tick()
	if got := decodeLabel(binary.LittleEndian.Uint64(m.f.ram[rouletteTarget:])); got != r.Roulette {
		t.Fatalf("lock follows the new pick: %s, roulette %s", got, r.Roulette)
	}
	before := r.Roulette
	if !m.tr.Reroll() || r.Rerolls != 0 || r.Roulette == before {
		t.Fatalf("reroll should spend one and change the event: %q -> %q", before, r.Roulette)
	}
	snap := m.tr.Snapshot()["run"].(map[string]any)["roulette"].(map[string]any)
	if snap["rerolls"] != 0 || snap["next_reroll"] != 4 || snap["rank"] == 0 {
		t.Fatalf("snapshot: %+v", snap)
	}
}

func TestEventRank(t *testing.T) {
	for l, want := range map[string]int{"K_01CDSR": 1, "K_10ULTI": 10, "K_03THLF": 3, "X": 0, "K_AB": 0} {
		if got := eventRank(l); got != want {
			t.Errorf("%s: %d, want %d", l, got, want)
		}
	}
}

// Without a live roulette (Grace, mode off) every event is open.
func TestRouletteLockClears(t *testing.T) {
	m := newModeRig(t, "Easy", []string{"HIGHUSCAR1A"}, nil)
	m.f.w64(eventIDs, encodeLabel("K_01CDSR"))
	m.f.w32(rouletteMarker, 1)
	m.tick()
	if binary.LittleEndian.Uint64(m.f.ram[rouletteTarget:]) == 0 {
		t.Fatal("the pick should be set")
	}
	m.tr.Grace()
	m.tick()
	if binary.LittleEndian.Uint64(m.f.ram[rouletteTarget:]) != 0 {
		t.Fatal("Grace should open every event")
	}
}

// Event Roulette picks a Burning Lap whose fixed car you own is wrecked: it's a freebie, the roulette
// moves on without a life lost.
func TestRouletteFreebie(t *testing.T) {
	m := newModeRig(t, "Easy", []string{"HIGHUSCAR1A", "HIGHUSCAR1B"}, nil)
	for i := 0; i < 169; i++ {
		m.f.w64(eventIDs+uint32(8*i), 0)
	}
	m.f.w64(eventIDs, encodeLabel("K_01BFLR"))
	m.f.w64(eventIDs+8, encodeLabel("K_01CDSR"))
	r := m.tr.run()
	r.Cars["HIGHUSCAR1B"].Lives = 0
	r.CarsLost = 1
	m.f.w32(rouletteMarker, 1)
	m.tick()
	if r.Roulette != "K_01BFLR" {
		r.Roulette = "K_01BFLR" // the Burning Lap is picked
	}
	lives := r.Cars["HIGHUSCAR1A"].Lives
	m.list(carouselList, []string{"HIGHUSCAR1B"}) // its fixed car, wrecked
	m.tick()
	if r.Roulette != "K_01CDSR" || len(r.Freebies) != 1 || r.Cars["HIGHUSCAR1A"].Lives != lives {
		t.Fatalf("should be a freebie: roulette %q, freebies %v", r.Roulette, r.Freebies)
	}
	m.list(carouselList, []string{"HIGHUSCAR1A", "HIGHUSCAR1B"})
	m.tick()
	if r.Roulette != "K_01CDSR" || len(r.Freebies) != 1 {
		t.Fatalf("a normal garage changes nothing: %q %v", r.Roulette, r.Freebies)
	}
}
