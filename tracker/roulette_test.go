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
	if r.Roulette == "" || r.Rerolls != 1 {
		t.Fatalf("first roll should be an event, with one reroll: %q, %d", r.Roulette, r.Rerolls)
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
	// rerolls: one to start with, one more for a win in each other rank (3 ranks)
	if len(r.RanksWon) != 3 || r.Rerolls != 3 {
		t.Fatalf("ranks won %v, rerolls %d; want 3 ranks and 3", r.RanksWon, r.Rerolls)
	}
	before := r.Roulette
	if !m.tr.Reroll() || r.Rerolls != 2 || r.Roulette == before {
		t.Fatalf("reroll should spend one and change the event: %q -> %q", before, r.Roulette)
	}
	snap := m.tr.Snapshot()["run"].(map[string]any)["roulette"].(map[string]any)
	if snap["rerolls"] != 2 || snap["rank"] == 0 {
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
