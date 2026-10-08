package main

// Chaos modifiers: with the mode on, every event gets a modifier rolled by the tracker that changes
// its rules: what it takes to pass, what a loss costs, what a win gives, or which car you may drive.

import (
	"fmt"
	"sort"
)

const (
	chaosMarker    = 0x000FE13C // [Nuzlocke\Mode\Chaos modifiers]
	chaosMarkerISO = 0x00479FEC
)

type chaosMod struct {
	id, name, text string
	good           bool // in your favor (green), or against you (red)
	weight         int
	hard           bool // rolled on Hard too
}

var chaosMods = []chaosMod{
	{"calm", "Calm", "no modifier this time", true, 3, true},
	{"safety", "Safety Net", "a loss costs no life", true, 2, true},
	{"second", "Second Wind", "a win gives a life back to your car with the fewest (wrecked ones first)", true, 2, true},
	{"easy", "Easy Street", "the requirement drops a step", true, 2, true},
	{"double", "Double or Nothing", "a loss costs 2 lives, a win gives a life back to your car with the fewest", false, 2, true},
	{"sudden", "Sudden Death", "a loss wrecks the car", false, 1, true},
	{"gold", "Gold or Bust", "only a Gold counts", false, 2, false}, // Hard needs a Gold anyway
	{"lone", "Lone Wolf", "one car picked at random is the only one you can drive", false, 2, true},
}

func chaosByID(id string) *chaosMod {
	for i := range chaosMods {
		if chaosMods[i].id == id {
			return &chaosMods[i]
		}
	}
	return nil
}

func (t *Tracker) chaosActive() bool {
	r := t.run()
	return t.chaosOn && r != nil && r.Active && !r.Dead && !r.Grace
}

// chaosNow is the modifier for the event being played ("" with the mode off).
func (t *Tracker) chaosNow() string {
	if !t.chaosActive() {
		return ""
	}
	return t.run().Chaos
}

// rollChaos picks the next event's modifier, never the same one twice in a row (Calm aside).
func (t *Tracker) rollChaos() {
	r := t.run()
	var pool []chaosMod
	total := 0
	for _, m := range chaosMods {
		if r.Difficulty == "Hard" && !m.hard || m.id == r.Chaos && m.id != "calm" {
			continue
		}
		pool = append(pool, m)
		total += m.weight
	}
	rng := t.pickRand()
	n := rng.Intn(total)
	pick := pool[0]
	for _, m := range pool {
		if n < m.weight {
			pick = m
			break
		}
		n -= m.weight
	}
	t.setChaos(pick.id)
	r.ChaosRolls++
	t.later("Chaos: "+t.chaosText(), 15, "info")
}

// setChaos makes id the next event's modifier, with Lone Wolf's cars.
func (t *Tracker) setChaos(id string) {
	r := t.run()
	r.Chaos, r.ChaosRace, r.ChaosCrash = id, "", ""
	if id == "lone" {
		r.ChaosRace, r.ChaosCrash = t.loneCar(false), t.loneCar(true)
	}
	t.dirty = true
}

// chaosCandidates: up to n other modifiers than not, for a chat vote, weighted like a roll.
func (t *Tracker) chaosCandidates(n int, not string) []string {
	r := t.run()
	var pool []chaosMod
	for _, m := range chaosMods {
		if r.Difficulty == "Hard" && !m.hard || m.id == not {
			continue
		}
		pool = append(pool, m)
	}
	rng := t.pickRand()
	var out []string
	for len(out) < n && len(pool) > 0 {
		total := 0
		for _, m := range pool {
			total += m.weight
		}
		k := rng.Intn(total)
		for i, m := range pool {
			if k < m.weight {
				out = append(out, m.id)
				pool = append(pool[:i], pool[i+1:]...)
				break
			}
			k -= m.weight
		}
	}
	return out
}

// loneCar picks Lone Wolf's car for a pool: from Limited Selection's pair when it's on.
func (t *Tracker) loneCar(crash bool) string {
	r := t.run()
	cands := t.usable(crash)
	if t.limitedActive() {
		pick := r.RacePick
		if crash {
			pick = r.CrashPick
		}
		var in []string
		for _, l := range pick {
			if contains(cands, l) {
				in = append(in, l)
			}
		}
		if len(in) > 0 {
			cands = in
		}
	}
	if len(cands) == 0 {
		return ""
	}
	sort.Strings(cands)
	return cands[t.pickRand().Intn(len(cands))]
}

// chaosText describes the current modifier, with Lone Wolf's cars.
func (t *Tracker) chaosText() string {
	r := t.run()
	m := chaosByID(r.Chaos)
	if m == nil {
		return ""
	}
	text := m.name + ": " + m.text
	if m.id == "easy" {
		text += fmt.Sprintf(" (%s)", easyStreetText(r.Difficulty))
	}
	if m.id == "lone" {
		text += fmt.Sprintf(" (Race: %s, Crash: %s)", t.carLabelName(r.ChaosRace), t.carLabelName(r.ChaosCrash))
	}
	return text
}

func (t *Tracker) carLabelName(l string) string {
	if c := t.run().Cars[l]; c != nil {
		return c.Name
	}
	return "-"
}

func easyStreetText(difficulty string) string {
	switch difficulty {
	case "Easy":
		return "Bronze + Good"
	case "Medium":
		return "Bronze + Great"
	}
	return "any Gold"
}

// checkChaos rolls the first modifier, and gives Lone Wolf a new car when its car can't be used any more.
func (t *Tracker) checkChaos() {
	if !t.chaosActive() {
		return
	}
	r := t.run()
	if r.Chaos == "" || chaosByID(r.Chaos) == nil {
		t.rollChaos()
		return
	}
	if r.Chaos != "lone" {
		return
	}
	for _, crash := range []bool{false, true} {
		car := &r.ChaosRace
		if crash {
			car = &r.ChaosCrash
		}
		if usable := t.usable(crash); !contains(usable, *car) && len(usable) > 0 {
			*car = t.loneCar(crash)
			t.dirty = true
		}
	}
}

// chaosBenched lists the healthy cars Lone Wolf leaves out.
func (t *Tracker) chaosBenched() (race, crash []string) {
	r := t.run()
	if t.chaosNow() != "lone" {
		return nil, nil
	}
	for _, l := range t.usable(false) {
		if l != r.ChaosRace && l != t.fixedCar {
			race = append(race, l)
		}
	}
	for _, l := range t.usable(true) {
		if l != r.ChaosCrash {
			crash = append(crash, l)
		}
	}
	return race, crash
}

// chaosPasses applies Easy Street and Gold or Bust to an event's result.
func chaosPasses(mod, difficulty string, medal uint32, shown int, perfectNow bool) bool {
	req, ok := requirements[difficulty]
	if !ok {
		req = requirements["Hard"]
	}
	switch mod {
	case "easy":
		if req.rating >= 4 {
			return medal == 3
		}
		return medal >= 1 && medal <= 3 && shown >= req.rating-1
	case "gold":
		return medal == 3 && passes(difficulty, medal, shown, perfectNow)
	}
	return passes(difficulty, medal, shown, perfectNow)
}

// chaosCost is how many lives a loss costs, out of the lives the car has.
func chaosCost(mod string, lives int) int {
	switch mod {
	case "safety":
		return 0
	case "double":
		return min(2, lives)
	case "sudden":
		return lives
	}
	return min(1, lives)
}

// chaosReward: Second Wind and Double or Nothing give a life back after a win, to the car of your own
// with the fewest lives in the event's pool (a wrecked one first), up to the starting lives.
func (t *Tracker) chaosReward(mod string, crash bool) {
	if mod != "second" && mod != "double" {
		return
	}
	r := t.run()
	best, bestLives := "", 0
	labels := make([]string, 0, len(r.Cars))
	for l := range r.Cars {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	for _, l := range labels {
		c := r.Cars[l]
		if !c.Owned || crash && !c.CanCrash() || !crash && !c.CanRace() {
			continue
		}
		lives := c.Lives
		if crash {
			lives = c.CrashLives
		}
		if lives < r.LivesStart && (best == "" || lives < bestLives) {
			best, bestLives = l, lives
		}
	}
	if best == "" {
		t.later(chaosByID(mod).name+": every car already has all its lives", 12, "info")
		return
	}
	c := r.Cars[best]
	lives, lost, pool := &c.Lives, &r.CarsLost, "Race"
	if crash {
		lives, lost, pool = &c.CrashLives, &r.CrashLost, "Crash"
	}
	if *lives == 0 && *lost > 0 {
		*lost--
	}
	*lives++
	t.later(fmt.Sprintf("%s: %s gets a %s life back (%d)", chaosByID(mod).name, c.Name, pool, *lives), 15, "win")
}

func (t *Tracker) chaosSnapshot() map[string]any {
	r := t.run()
	m := chaosByID(t.chaosNow())
	if m == nil {
		return nil
	}
	out := map[string]any{"id": m.id, "name": m.name, "text": m.text, "good": m.good}
	if m.id == "easy" {
		out["text"] = m.text + " (" + easyStreetText(r.Difficulty) + ")"
	}
	if m.id == "lone" {
		out["race_car"], out["crash_car"] = t.carLabelName(r.ChaosRace), t.carLabelName(r.ChaosCrash)
	}
	return out
}
