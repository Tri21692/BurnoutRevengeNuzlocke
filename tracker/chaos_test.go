package main

import (
	"encoding/binary"
	"testing"
)

// result plays a Race event with car and a given medal and rating.
func (m *modeRig) result(car string, medal, rating uint32) {
	m.f.w64(selectedCar, encodeLabel(car))
	m.f.w64(rigObj, encodeLabel(car))
	m.f.w64(rigObj+0x18, encodeLabel("K_01RDSF"))
	m.f.w32(lastMedal, medal)
	m.f.w32(lastRating, rating)
	m.f.w32(finishCounter, binary.LittleEndian.Uint32(m.f.ram[finishCounter:])+1)
	m.clock += 0.25
	m.tr.Poll()
	m.clock += 1.5
	m.tr.Poll()
	m.tick()
}

func TestChaosModifiers(t *testing.T) {
	cars := []string{"HIGHUSCAR1A", "HIGHUSCAR1B", "HIGHUSCAR1C"}
	m := newModeRig(t, "Medium", cars, nil)
	r := m.tr.run()
	if r.Chaos != "" {
		t.Fatal("no modifier while the mode is off")
	}
	m.f.w32(chaosMarker, 1)
	m.tick()
	if chaosByID(r.Chaos) == nil || !r.UsedChaos || !m.tr.achievementsLocked() {
		t.Fatalf("the first modifier should be rolled and achievements locked: %q", r.Chaos)
	}
	a, b, c := r.Cars[cars[0]], r.Cars[cars[1]], r.Cars[cars[2]]
	play := func(mod, car string, medal, rating uint32) {
		r.Chaos = mod
		if mod == "lone" {
			r.ChaosRace = car
		}
		m.result(car, medal, rating)
		if r.Chaos == mod && mod != "calm" {
			t.Fatalf("%s: a new modifier should be rolled after the event, not the same one", mod)
		}
	}
	play("safety", cars[0], 1, 0)
	if a.Lives != 2 {
		t.Fatalf("Safety Net: no life lost, got %d", a.Lives)
	}
	play("double", cars[0], 1, 0)
	if a.Lives != 0 || r.CarsLost != 1 {
		t.Fatalf("Double or Nothing: 2 lives lost, got %d (%d lost)", a.Lives, r.CarsLost)
	}
	play("calm", cars[1], 1, 0)
	if b.Lives != 1 {
		t.Fatalf("Calm: one life, got %d", b.Lives)
	}
	play("second", cars[1], 3, 2)
	if a.Lives != 1 || r.CarsLost != 0 || b.Lives != 1 {
		t.Fatalf("Second Wind: the wrecked car gets a life back: %d %d (%d lost)", a.Lives, b.Lives, r.CarsLost)
	}
	play("gold", cars[1], 2, 3) // Silver + Awesome passes Medium, but not Gold or Bust
	if b.Lives != 0 {
		t.Fatalf("Gold or Bust: a Silver is a loss, got %d", b.Lives)
	}
	play("easy", cars[2], 1, 2) // Bronze + Great passes Medium's Easy Street
	if c.Lives != 2 || !r.History[len(r.History)-1].Won {
		t.Fatalf("Easy Street: Bronze + Great should pass, got %d", c.Lives)
	}
	play("sudden", cars[2], 1, 0)
	if c.Lives != 0 && !r.Dead {
		t.Fatalf("Sudden Death: the car is wrecked, got %d", c.Lives)
	}
}

func TestChaosLoneWolf(t *testing.T) {
	cars := []string{"HIGHUSCAR1A", "HIGHUSCAR1B", "HIGHUSCAR1C"}
	m := newModeRig(t, "Easy", cars, nil)
	r := m.tr.run()
	m.f.w32(chaosMarker, 1)
	m.tick()
	r.Chaos, r.ChaosRace = "lone", cars[1]
	m.tick()
	blocked := m.table(deadTable)
	if blocked[cars[1]] || !blocked[cars[0]] || !blocked[cars[2]] {
		t.Fatalf("Lone Wolf: only its car can be driven: %v", blocked)
	}
	// its car wrecked (e.g. by a loan payer): another one is picked
	r.Cars[cars[1]].Lives = 0
	m.tick()
	if r.ChaosRace == cars[1] || r.ChaosRace == "" {
		t.Fatalf("Lone Wolf should move to a usable car: %q", r.ChaosRace)
	}
	// not on Hard: Gold or Bust
	h := newModeRig(t, "Hard", cars, nil)
	h.f.w32(chaosMarker, 1)
	for i := 0; i < 50; i++ {
		h.tr.rollChaos()
		if h.tr.run().Chaos == "gold" {
			t.Fatal("Gold or Bust should not be rolled on Hard")
		}
	}
}
