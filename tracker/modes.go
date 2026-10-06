package main

// V2 modes: Limited Selection and Revive tokens (switched on by the patch, like the Harder AI level),
// and the achievements and end-of-run summary every run gets.

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The patch tells the tracker which modes are on: the .pnach's mode groups write a marker below the
// game, a patched ISO has the same words in the mod's space in the game file.
const (
	limitedMarker    = 0x000FE130 // [Nuzlocke\Mode\Limited Selection]
	limitedMarkerISO = 0x00479FF4
	reviveMarker     = 0x000FE134 // [Nuzlocke\Mode\Revive tokens]
	reviveMarkerISO  = 0x00479FF8
)

// Revive tokens: wins in a row needed for one, and how many can be held at once.
var reviveRules = map[string]struct{ streak, hold int }{
	"Easy": {3, 3}, "Medium": {5, 2}, "Hard": {8, 1},
}

func (t *Tracker) readModes() {
	t.limitedOn = t.u32(limitedMarker) == 1 || t.u32(limitedMarkerISO) == 1
	t.reviveOn = t.u32(reviveMarker) == 1 || t.u32(reviveMarkerISO) == 1
	if r := t.run(); r != nil && r.Active && !r.Dead {
		if t.limitedOn && !r.UsedLimited || t.reviveOn && !r.UsedRevive {
			r.UsedLimited = r.UsedLimited || t.limitedOn
			r.UsedRevive = r.UsedRevive || t.reviveOn
			t.save()
		}
	}
}

// ---- Limited Selection ----

// limitedActive: the pairs only apply in a live, counting run.
func (t *Tracker) limitedActive() bool {
	r := t.run()
	return t.limitedOn && r != nil && r.Active && !r.Dead && !r.Grace
}

// usable lists the cars of your own that still have lives in a pool, sorted.
func (t *Tracker) usable(crash bool) []string {
	r := t.run()
	var out []string
	for l, c := range r.Cars {
		if !c.Owned {
			continue
		}
		if crash && c.CanCrash() && c.CrashLives > 0 || !crash && c.CanRace() && c.Lives > 0 {
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out
}

// poolCars counts the cars of your own that have a pool, wrecked or not.
func (t *Tracker) poolCars(crash bool) int {
	n := 0
	for _, c := range t.run().Cars {
		if c.Owned && (crash && c.CanCrash() || !crash && c.CanRace()) {
			n++
		}
	}
	return n
}

func (t *Tracker) pickRand() *rand.Rand {
	r := t.run()
	h := fnv.New64a()
	fmt.Fprintf(h, "%s|%d", r.Seed, r.PickRolls)
	r.PickRolls++
	return rand.New(rand.NewSource(int64(h.Sum64())))
}

// rollPair picks up to 2 usable cars for a pool, none from the previous pair if it can.
func (t *Tracker) rollPair(crash bool, prev []string) []string {
	cands := t.usable(crash)
	was := map[string]bool{}
	for _, l := range prev {
		was[l] = true
	}
	var fresh, again []string
	for _, l := range cands {
		if was[l] {
			again = append(again, l)
		} else {
			fresh = append(fresh, l)
		}
	}
	rng := t.pickRand()
	rng.Shuffle(len(fresh), func(i, j int) { fresh[i], fresh[j] = fresh[j], fresh[i] })
	rng.Shuffle(len(again), func(i, j int) { again[i], again[j] = again[j], again[i] })
	pair := append(fresh, again...) // the previous pair only if there's nothing else
	if len(pair) > 2 {
		pair = pair[:2]
	}
	sort.Strings(pair)
	return pair
}

// rollPicks picks new pairs for the next event.
func (t *Tracker) rollPicks() {
	r := t.run()
	r.RacePick = t.rollPair(false, r.RacePick)
	r.CrashPick = t.rollPair(true, r.CrashPick)
	t.dirty = true
}

// checkPicks keeps the pairs valid between events: a pick that can't be used any more is replaced,
// and a short pair is topped up when more cars become usable. It never rerolls a usable pick.
func (t *Tracker) checkPicks() {
	if !t.limitedActive() {
		return
	}
	r := t.run()
	for _, crash := range []bool{false, true} {
		pick := &r.RacePick
		if crash {
			pick = &r.CrashPick
		}
		ok := map[string]bool{}
		for _, l := range t.usable(crash) {
			ok[l] = true
		}
		var keep []string
		for _, l := range *pick {
			if ok[l] {
				keep = append(keep, l)
			}
		}
		if len(keep) == len(*pick) && len(keep) >= min(2, len(ok)) {
			continue
		}
		// fill the free places from the cars not already picked
		var rest []string
		for l := range ok {
			if !contains(keep, l) {
				rest = append(rest, l)
			}
		}
		sort.Strings(rest)
		rng := t.pickRand()
		rng.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
		for len(keep) < 2 && len(rest) > 0 {
			keep, rest = append(keep, rest[0]), rest[1:]
		}
		sort.Strings(keep)
		*pick = keep
		t.dirty = true
	}
}

// benched lists the healthy cars of your own left out of each pool's pair.
func (t *Tracker) benched() (race, crash []string) {
	if !t.limitedActive() {
		return nil, nil
	}
	r := t.run()
	for _, l := range t.usable(false) {
		if !contains(r.RacePick, l) {
			race = append(race, l)
		}
	}
	for _, l := range t.usable(true) {
		if !contains(r.CrashPick, l) {
			crash = append(crash, l)
		}
	}
	return race, crash
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ---- Revive tokens ----

// earnToken gives a token for every streak of wins the difficulty asks for, up to the most it allows.
func (t *Tracker) earnToken() {
	r := t.run()
	rule, ok := reviveRules[r.Difficulty]
	if !t.reviveOn || !ok || r.Streak == 0 || r.Streak%rule.streak != 0 {
		return
	}
	if r.Tokens >= rule.hold {
		t.later(fmt.Sprintf("%d wins in a row, but you already hold %s", r.Streak, plural(r.Tokens, "revive token", "revive tokens")), 15, "info")
		return
	}
	r.Tokens++
	r.TokensEarned++
	t.later(fmt.Sprintf("Revive token earned: %d wins in a row (%d held)", r.Streak, r.Tokens), 20, "win")
}

// Revive spends a token to give a wrecked car of your own one life back in a pool.
func (t *Tracker) Revive(label string, crash bool) bool {
	r := t.run()
	if r == nil || !r.Active || r.Dead || r.Grace || r.Tokens == 0 {
		return false
	}
	c := r.Cars[label]
	if c == nil || !c.Owned {
		return false
	}
	lives, lost, pool := &c.Lives, &r.CarsLost, "Race"
	if crash {
		lives, lost, pool = &c.CrashLives, &r.CrashLost, "Crash"
	}
	if *lives != 0 || crash && !c.CanCrash() || !crash && !c.CanRace() {
		return false
	}
	*lives = 1
	if *lost > 0 {
		*lost--
	}
	r.Tokens--
	r.Revived++
	t.say(fmt.Sprintf("%s is back for %s events with 1 life (revive token)", c.Name, pool), 20, "win")
	t.unlock("comeback")
	t.save()
	return true
}

// autoRevive keeps a run alive that would just have ended, with a held token, on the car that lost
// its last life in this event.
func (t *Tracker) autoRevive() bool {
	r := t.run()
	if !t.reviveOn || r.Tokens == 0 || t.lastLoss == "" {
		return false
	}
	if !t.Revive(t.lastLoss, t.lastLossCrash) {
		return false
	}
	t.later(fmt.Sprintf("Last chance: a revive token saved the run (%s)", r.Cars[t.lastLoss].Name), 25, "win")
	return true
}

// ---- achievements ----

type Achievement struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Text string `json:"text"`
	Time string `json:"time"`
}

var achievementList = []struct{ id, name, text string }{
	{"streak5", "Hot Streak", "5 wins in a row"},
	{"streak10", "Unstoppable", "10 wins in a row"},
	{"streak25", "Legend", "25 wins in a row"},
	{"perfect5", "Perfectionist", "5 Gold + Perfect results"},
	{"crash5", "Junction King", "5 Crash junctions won"},
	{"cars15", "Collector", "15 cars of your own"},
	{"events50", "Survivor", "50 events played in one run"},
	{"insane10", "Certified Insane", "10 wins against Insane AI"},
	{"lastcar", "Last One Standing", "a win with only one usable car left"},
	{"comeback", "Back From the Dead", "a car revived with a token"},
}

func (t *Tracker) unlock(id string) {
	r := t.run()
	for _, a := range r.Achievements {
		if a.ID == id {
			return
		}
	}
	for _, a := range achievementList {
		if a.id == id {
			r.Achievements = append(r.Achievements, Achievement{a.id, a.name, a.text, time.Now().Format("2006-01-02 15:04")})
			t.later(fmt.Sprintf("Achievement: %s (%s)", a.name, a.text), 15, "achievement")
			return
		}
	}
}

// eventAchievements checks the achievements after a counted event.
func (t *Tracker) eventAchievements(won, crash, perfect, lastCar bool) {
	r := t.run()
	if won && perfect {
		r.Perfects++
	}
	if won && crash {
		r.CrashWins++
	}
	if won && t.aiLevel == "Insane" {
		r.InsaneWins++
	}
	checks := []struct {
		id string
		ok bool
	}{
		{"streak5", r.Streak >= 5}, {"streak10", r.Streak >= 10}, {"streak25", r.Streak >= 25},
		{"perfect5", r.Perfects >= 5}, {"crash5", r.CrashWins >= 5}, {"cars15", t.ownedCars() >= 15},
		{"events50", r.EventsPlayed >= 50}, {"insane10", r.InsaneWins >= 10}, {"lastcar", won && lastCar},
	}
	for _, c := range checks {
		if c.ok {
			t.unlock(c.id)
		}
	}
}

// ---- end-of-run summary ----

type carSummary struct {
	Label, Name          string
	Events, Wins         int
	Lives, CrashLives    int
	CanRace, CanCrash    bool
	Gold, Silver, Bronze int
	Wrecked, Owned       bool
}

func (t *Tracker) carSummaries() []carSummary {
	r := t.run()
	byLabel := map[string]*carSummary{}
	for l, c := range r.Cars {
		byLabel[l] = &carSummary{Label: l, Name: c.Name, Lives: c.Lives, CrashLives: c.CrashLives,
			CanRace: c.CanRace(), CanCrash: c.CanCrash(), Owned: c.Owned,
			Wrecked: (!c.CanRace() || c.Lives == 0) && (!c.CanCrash() || c.CrashLives == 0)}
	}
	for _, h := range r.History {
		s := byLabel[h.Label]
		if s == nil || !h.Counted {
			continue
		}
		s.Events++
		if h.Won {
			s.Wins++
		}
		switch medalOf(h) {
		case "gold":
			s.Gold++
		case "silver":
			s.Silver++
		case "bronze":
			s.Bronze++
		}
	}
	var out []carSummary
	for _, s := range byLabel {
		if s.Events > 0 || s.Owned {
			out = append(out, *s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Events != out[j].Events {
			return out[i].Events > out[j].Events
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (t *Tracker) summarySnapshot() map[string]any {
	r := t.run()
	var cars []map[string]any
	for _, s := range t.carSummaries() {
		cars = append(cars, map[string]any{"name": s.Name, "events": s.Events, "wins": s.Wins,
			"gold": s.Gold, "silver": s.Silver, "bronze": s.Bronze, "wrecked": s.Wrecked, "owned": s.Owned,
			"lives": s.Lives, "crash_lives": s.CrashLives, "can_race": s.CanRace, "can_crash": s.CanCrash})
	}
	return map[string]any{"cars": cars, "ended_by": r.EndedBy, "achievements": r.Achievements,
		"perfects": r.Perfects, "revived": r.Revived, "tokens_earned": r.TokensEarned, "seed": r.Seed,
		"limited": r.UsedLimited, "revive": r.UsedRevive, "file": r.SummaryFile}
}

// writeSummary saves the end-of-run summary as a text file next to the run's state.
func (t *Tracker) writeSummary() {
	r := t.run()
	var b strings.Builder
	fmt.Fprintf(&b, "BURNOUT REVENGE NUZLOCKE - RUN SUMMARY\r\n\r\n")
	fmt.Fprintf(&b, "%s run, %d + %d lives per car, AI %s\r\n", r.Difficulty, r.LivesStart, r.LivesStart, orDash(r.AILevel))
	var modes []string
	if r.UsedLimited {
		modes = append(modes, "Limited Selection")
	}
	if r.UsedRevive {
		modes = append(modes, "Revive tokens")
	}
	if len(modes) > 0 {
		fmt.Fprintf(&b, "Modes: %s\r\n", strings.Join(modes, ", "))
	}
	fmt.Fprintf(&b, "Started %s, run time %s, seed %s\r\n", r.Started, fmtTime(r.PlaySeconds), r.Seed)
	if r.EndedBy != "" {
		fmt.Fprintf(&b, "Ended by: %s\r\n", r.EndedBy)
	}
	rate := "-"
	if r.EventsPlayed > 0 {
		rate = fmt.Sprintf("%d%%", 100*r.EventsWon/r.EventsPlayed)
	}
	fmt.Fprintf(&b, "\r\nEvents won %d of %d (%s), best streak %d, Gold + Perfects %d\r\n", r.EventsWon, r.EventsPlayed, rate, r.BestStreak, r.Perfects)
	fmt.Fprintf(&b, "Cars wrecked: %d Race, %d Crash; revived %d; revive tokens earned %d\r\n", r.CarsLost, r.CrashLost, r.Revived, r.TokensEarned)
	fmt.Fprintf(&b, "\r\nCARS\r\n")
	for _, s := range t.carSummaries() {
		state := "alive"
		if s.Wrecked {
			state = "WRECKED"
		}
		if !s.Owned {
			state += ", loaned"
		}
		fmt.Fprintf(&b, "  %-24s %3d events, %3d wins  (gold %d, silver %d, bronze %d)  %s\r\n", s.Name, s.Events, s.Wins, s.Gold, s.Silver, s.Bronze, state)
	}
	if len(r.Achievements) > 0 {
		fmt.Fprintf(&b, "\r\nACHIEVEMENTS\r\n")
		for _, a := range r.Achievements {
			fmt.Fprintf(&b, "  %s: %s (%s)\r\n", a.Name, a.Text, a.Time)
		}
	}
	name := "nuzlocke_summary_" + time.Now().Format("2006-01-02_1504") + ".txt"
	path := filepath.Join(filepath.Dir(t.statePath), name)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err == nil {
		r.SummaryFile = name
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
