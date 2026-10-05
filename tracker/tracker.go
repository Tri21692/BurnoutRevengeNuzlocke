package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Game addresses (SLUS-21242 v1.00)
const (
	serialWanted  = "SLUS-21242"
	eventCount    = 0x0056B1EC // 169
	eventIDs      = 0x0056B250 // 8-byte packed label per event
	eventResults  = 0x01F651DB // 1 byte per event, FF = not played, 04 = Gold + Perfect
	lastMedal     = 0x0056C970
	lastRating    = 0x0056C974
	resultPos     = 0x0052CA9C // FFFFFFFF while an event is running
	selectedCar   = 0x01EDACD0
	carouselList  = 0x01C94990 // garage carousel: labels, count at +0xBA4
	crashCarousel = 0x01C958A8 // crash junction car select, player 1 (player 2 is +0xC90)
	carouselSize  = 0xC90
	deadTable     = 0x000FF400 // cars wrecked in the Race pool: blocked in the garage
	crashTable    = 0x000FF700 // cars wrecked in the Crash pool: blocked in crash junctions
	deadMax       = 79
	currentEvent  = 0x01C10C18 // pointer to the loaded car (+0x00) and event (+0x18) labels
	deadFlag      = 0x000FE120 // 1 while the run is dead: the patch holds the game on the results screen
	textLo        = 0x00670000 // where the text table usually is; it can move, so it's searched for
	textWindow    = 0x80000    // bytes read around the place the text table was found

	// Written by the patch's finished-event signal (part 2 of the .pnach)
	finishHook    = 0x00115A60 // jal to the signal stub when the patch is active
	finishHookOn  = 0x0C03FC40
	finishCounter = 0x000FE100 // +1 every time the game stores an event result

	// Written by the Harder AI groups of the .pnach
	aiHook   = 0x00298FEC // jal to the catch-up wrapper when a Harder AI level is active
	aiHookOn = 0x0C03FE80
	aiMarker = 0x000FE110 // 1 Easy, 2 Medium, 3 Hard

	// The patched ISOs (isopatch/) carry the same code in unused space in the game's .data section
	// (00479D00-00479FFF) instead, so their hooks jump there, and the level is a word in that space.
	finishHookISO = 0x0C11E768
	aiHookISO     = 0x0C11E600 // jal 00479800 (V1.1.2 and later)
	aiHookISOOld  = 0x0C11E7A0 // jal 00479E80 (V1.1 and V1.1.1)
	aiLevelISO    = 0x00479FF0

	// The crash junction block reads its own table (crashTable) since V1.1; older patches read deadTable.
	crashTableRef    = 0x000FF180 // ori t4,t4,<table> in the .pnach's crash junction block
	crashTableRefISO = 0x00479E10 // the same instruction in a patched ISO
	crashTableRefOn  = 0x358CF700

	// The results screens' run's-dead lock (V1.1.2): the race results handler starts with a jump to it.
	deadLockHook    = 0x0019AD98
	deadLockHookOn  = 0x0803FC80 // j 000FF200 (.pnach)
	deadLockHookISO = 0x0811E700 // j 00479C00 (patched ISO)
)

var patchHooks = map[uint32]uint32{0x002ACE00: 0x0C03FC00, 0x002A69DC: 0x0C03FC00, finishHook: finishHookOn,
	0x0018F508: 0x0803FC50} // crash junction car select
var isoHooks = map[uint32]uint32{0x002ACE00: 0x0C11E740, 0x002A69DC: 0x0C11E740, finishHook: finishHookISO,
	0x0018F508: 0x0811E774}
var difficulties = map[string]int{"Easy": 3, "Medium": 2, "Hard": 1}

// requirements: the least an event needs on each difficulty (medal, rating shown on the results screen).
// Hard is Gold + Perfect, which the game only awards the first time.
var requirements = map[string]struct {
	medal, rating int
	text          string
}{
	"Easy":   {2, 2, "Silver + Great"},
	"Medium": {2, 3, "Silver + Awesome"},
	"Hard":   {3, 4, "Gold + Perfect"},
}

// passes reports whether a result meets the difficulty's requirement. shown is the rating as shown
// (Gold raises it one step); perfectNow is true when the event got its first Perfect just now.
func passes(difficulty string, medal uint32, shown int, perfectNow bool) bool {
	req, ok := requirements[difficulty]
	if !ok {
		req = requirements["Hard"]
	}
	if req.rating >= 4 {
		return medal == 3 && shown == 4 && perfectNow
	}
	return int(medal) >= req.medal && medal <= 3 && shown >= req.rating
}

var medals = map[uint32]string{3: "Gold", 2: "Silver", 1: "Bronze", 0: "No medal"}
var ratings = []string{"-", "Good", "Great", "Awesome", "Perfect"}
var aiLevels = []string{"Off", "Easy", "Medium", "Hard"}

// Mem is the connection to PCSX2 (real on Windows, faked in tests).
type Mem interface {
	Connected() bool
	Connect() bool
	Drop()
	Read(addr uint32, n int) ([]byte, error)
	Write(addr uint32, data []byte) error
	Serial() string
	HasPine() bool
}

type Car struct {
	Name       string `json:"name"`
	Lives      int    `json:"lives"`       // Race pool: every event type except Crash junctions
	CrashLives int    `json:"crash_lives"` // Crash pool: Crash junctions
	InGarage   bool   `json:"in_garage"`   // seen in the garage (or used in a Race event): can race
	InCrash    bool   `json:"in_crash"`    // seen in a crash junction's car select (or used there)
}

// CanRace and CanCrash: which pools a car has. A car not seen anywhere yet (from an older save) has both.
func (c *Car) CanRace() bool  { return c.InGarage || !c.InCrash }
func (c *Car) CanCrash() bool { return c.InCrash || !c.InGarage }

type HistoryEntry struct {
	Time    string `json:"time"`
	Event   string `json:"event"`
	Car     string `json:"car"`
	Result  string `json:"result"`
	Won     bool   `json:"won"`
	Counted bool   `json:"counted"`
	Crash   bool   `json:"crash,omitempty"` // a Crash junction (Crash pool)
	Label   string `json:"car_label,omitempty"`
	Medal   string `json:"medal,omitempty"` // gold, silver, bronze or none (empty in older saves)
	Where   string `json:"location,omitempty"`
}

// medalOf returns gold, silver, bronze or none for a history entry, from its result for older saves.
func medalOf(h HistoryEntry) string {
	if h.Medal != "" {
		return h.Medal
	}
	for _, m := range []string{"Gold", "Silver", "Bronze"} {
		if strings.HasPrefix(h.Result, m) {
			return strings.ToLower(m)
		}
	}
	return "none"
}

var medalKeys = map[uint32]string{3: "gold", 2: "silver", 1: "bronze"}

type Run struct {
	Active       bool            `json:"active"`
	Difficulty   string          `json:"difficulty"`
	LivesStart   int             `json:"lives_start"`
	Started      string          `json:"started"`
	PlaySeconds  float64         `json:"play_seconds"`
	Grace        bool            `json:"grace"`
	AILevel      string          `json:"ai_level,omitempty"` // last Harder AI level seen during the run
	Dead         bool            `json:"dead"`
	Cars         map[string]*Car `json:"cars"`
	EventsPlayed int             `json:"events_played"`
	EventsWon    int             `json:"events_won"`
	CarsLost     int             `json:"cars_lost"`       // wrecked in the Race pool
	CrashLost    int             `json:"crash_cars_lost"` // wrecked in the Crash pool
	Pools        bool            `json:"pools"`           // false for runs saved before the two pools
	Streak       int             `json:"streak"`
	BestStreak   int             `json:"best_streak"`
	History      []HistoryEntry  `json:"history"`
}

type State struct {
	Run *Run `json:"run"`
}

type memError struct{ err error }

type Tracker struct {
	mem               Mem
	statePath         string
	state             State
	now               func() float64
	message           string
	messageKind       string
	messageID         int
	messageUntil      float64
	eventActive       bool
	eventStartResults []byte
	pendingFinishAt   float64
	knownResults      []byte
	patchOK           int    // -1 unknown, 0 no, 1 yes
	aiLevel           string // Harder AI level in the game ("" until read)
	wrongGame         bool
	lastSlow          float64
	lastTick          float64
	originals         map[string]string
	OnRunDead         func()
	lastCounter       int64   // last finished-event counter seen, -1 = not yet
	resultsBefore     []byte  // saved results just before a change seen without the counter
	resultsChangedAt  float64 // when that happened
	textBase          uint32  // where the text table was found (0 = not found yet)
	lastLocate        float64
	nameCache         map[string]string // label -> display name, once found
	pendingEvent      string            // event label read when the event started or its result came in
	pendingCar        string            // car label read at the same time
	pendingTrack      string            // track label read at the same time, e.g. US_K1_V1
	patchOld          bool              // the crash junction block still uses the shared table
	dirty             bool              // a car's garage / crash junction flags changed
	carsBefore        map[string]bool   // cars the run had when the current event's result came in
	deadLockOn        bool              // the patch has the run's-dead results-screen lock
}

// Names that are always in the text table, used to find it.
var textAnchors = []string{"HIGHEUCAR2S1", "HIGHASCAR1S1", "K_01DH1E", "K_01DH3E"}

func NewTracker(mem Mem, statePath string, now func() float64) *Tracker {
	t := &Tracker{mem: mem, statePath: statePath, now: now, patchOK: -1,
		originals: map[string]string{}, messageKind: "info", lastCounter: -1,
		nameCache: map[string]string{}}
	t.lastTick = now()
	if data, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(data, &t.state)
	}
	if r := t.state.Run; r != nil && r.Cars == nil {
		r.Cars = map[string]*Car{}
	}
	if r := t.state.Run; r != nil && !r.Pools {
		// runs from before the two pools: each car keeps its lives in both
		for _, c := range r.Cars {
			c.CrashLives = c.Lives
		}
		r.CrashLost = r.CarsLost
		r.Pools = true
	}
	return t
}

func (t *Tracker) save() {
	data, err := json.MarshalIndent(t.state, "", " ")
	if err != nil {
		return
	}
	tmp := t.statePath + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, t.statePath)
	}
}

func (t *Tracker) say(text string, seconds float64, kind string) {
	t.message, t.messageKind = text, kind
	t.messageID++
	t.messageUntil = t.now() + seconds
}

func (t *Tracker) run() *Run { return t.state.Run }

func (t *Tracker) counting() bool {
	r := t.run()
	return r != nil && r.Active && !r.Grace && !r.Dead
}

// ---- memory helpers: reads panic with memError, recovered in Poll ----
func (t *Tracker) read(addr uint32, n int) []byte {
	b, err := t.mem.Read(addr, n)
	if err != nil {
		panic(memError{err})
	}
	return b
}
func (t *Tracker) u32(addr uint32) uint32 { return binary.LittleEndian.Uint32(t.read(addr, 4)) }
func (t *Tracker) u64(addr uint32) uint64 { return binary.LittleEndian.Uint64(t.read(addr, 8)) }

// textRegion returns a copy of the text table and its address, or nil if it hasn't been found.
func (t *Tracker) textRegion() ([]byte, uint32) {
	if t.textBase == 0 {
		return nil, 0
	}
	return t.read(t.textBase, textWindow), t.textBase
}

// hasAnchor: at least two of the known names are in this region, so it really is the text table.
func hasAnchor(region []byte, base uint32) bool {
	found := 0
	for _, a := range textAnchors {
		if _, text, _, ok := findText(region, base, a); ok && text != "" {
			found++
		}
	}
	return found >= 2
}

// locateText searches PS2 memory for the text table, using names that are always in it.
func (t *Tracker) locateText() bool {
	const chunk = 0x100000
	keys := make([][]byte, len(textAnchors))
	for i, a := range textAnchors {
		keys[i] = make([]byte, 4)
		binary.LittleEndian.PutUint32(keys[i], textID(a))
	}
	try := func(start uint32) bool {
		data := t.read(start, chunk+16)
		for _, k := range keys {
			for off := 0; ; {
				i := bytes.Index(data[off:], k)
				if i < 0 {
					break
				}
				i += off
				off = i + 1
				addr := start + uint32(i)
				if addr%4 != 0 || i+6 > len(data) {
					continue
				}
				c := binary.LittleEndian.Uint16(data[i+4:])
				if c < 0x20 || c > 0x7E { // the text right after the ID must look like text
					continue
				}
				base := uint32(0x00100000)
				if addr > base+textWindow/2 {
					base = (addr - textWindow/2) &^ 3
				}
				if base+textWindow > 0x2000000 {
					base = 0x2000000 - textWindow
				}
				if region := t.read(base, textWindow); hasAnchor(region, base) {
					t.textBase = base
					return true
				}
			}
		}
		return false
	}
	// the usual place first, then everything else
	if try(textLo) {
		return true
	}
	for s := uint32(0x00100000); s+chunk+16 <= 0x2000000; s += chunk {
		if try(s) {
			return true
		}
	}
	t.textBase = 0
	return false
}

// currentText returns the text table, finding it again if it moved (at most every 10 seconds).
func (t *Tracker) currentText() ([]byte, uint32) {
	region, base := t.textRegion()
	if region != nil && hasAnchor(region, base) {
		return region, base
	}
	if t.now()-t.lastLocate < 10 && t.lastLocate != 0 {
		return nil, 0
	}
	t.lastLocate = t.now()
	if !t.locateText() {
		return nil, 0
	}
	return t.textRegion()
}

// ---- run control ----
func (t *Tracker) StartRun(difficulty string) {
	lives, ok := difficulties[difficulty]
	if !ok {
		return
	}
	if t.run() != nil {
		t.restoreNames(nil, 0, true)
	}
	t.state.Run = &Run{Active: true, Difficulty: difficulty, LivesStart: lives, Pools: true,
		Started: time.Now().Format("2006-01-02 15:04:05"), Cars: map[string]*Car{}}
	t.originals = map[string]string{}
	t.save()
	t.syncDeadFlag()
	t.say(fmt.Sprintf("New %s run: %d Race and %d Crash %s per car.", difficulty, lives, lives, plural(lives, "life", "lives")), 15, "info")
}

// syncDeadFlag tells the patch whether the run is dead, so it holds the game on the results screen.
func (t *Tracker) syncDeadFlag() {
	if !t.mem.Connected() {
		return
	}
	defer func() { _ = recover() }() // also called from the web page's buttons, outside Poll
	r := t.run()
	want := uint32(0)
	if r != nil && r.Active && r.Dead && !r.Grace {
		want = 1
	}
	if t.u32(deadFlag) != want {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, want)
		_ = t.mem.Write(deadFlag, b)
	}
}

func (t *Tracker) Grace() {
	if r := t.run(); r != nil {
		r.Grace = true
		t.save()
		t.syncDeadFlag() // let the game leave the results screen
		t.restoreNames(nil, 0, true)
		t.say("Grace mode: nothing counts any more. All cars are usable again.", 15, "info")
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// ---- polling ----
func (t *Tracker) Poll() {
	now := t.now()
	dt := now - t.lastTick
	t.lastTick = now
	if !t.mem.Connected() {
		if !t.mem.Connect() {
			return
		}
		t.eventActive, t.knownResults, t.pendingFinishAt = false, nil, 0
		t.lastCounter, t.resultsBefore = -1, nil
	}
	defer func() {
		if rec := recover(); rec != nil {
			if _, ok := rec.(memError); ok {
				t.mem.Drop()
				return
			}
			panic(rec)
		}
	}()
	t.pollConnected(now, dt)
}

func (t *Tracker) pollConnected(now, dt float64) {
	serial := t.mem.Serial()
	if serial != "" && serial != serialWanted {
		t.wrongGame = true
		return
	}
	count := t.u32(eventCount)
	if count != 169 {
		t.wrongGame = false
		return
	}
	t.wrongGame = false
	if r := t.run(); r != nil && r.Active && !r.Dead {
		if dt > 5 {
			dt = 5
		}
		r.PlaySeconds += dt
	}
	results := t.read(eventResults, int(count))
	if t.knownResults == nil || len(t.knownResults) != len(results) {
		t.knownResults = results
	}

	hook := t.u32(finishHook) == finishHookOn || t.u32(finishHook) == finishHookISO
	if hook {
		// Every event type stores its result through one function, which the patch counts.
		c := int64(t.u32(finishCounter))
		switch {
		case t.lastCounter < 0 || c < t.lastCounter || c-t.lastCounter > 8:
			t.lastCounter = c // first look, or the game restarted: just resync
		case c != t.lastCounter:
			t.lastCounter = c
			if t.pendingFinishAt == 0 {
				t.captureEvent()
				t.eventStartResults = t.knownResults
				if t.resultsBefore != nil && now-t.resultsChangedAt < 3 {
					t.eventStartResults = t.resultsBefore // results moved a moment before the signal
				}
				t.pendingFinishAt = now + 1.0 // let the game finish writing the result
			}
		}
		t.eventActive = false
	} else {
		inEvent := t.u32(resultPos) == 0xFFFFFFFF
		if inEvent && !t.eventActive {
			t.eventActive = true
			t.captureEvent()
			t.eventStartResults = t.knownResults
		} else if !inEvent && t.eventActive {
			t.eventActive = false
			t.pendingFinishAt = now + 1.0
		}
	}
	if t.pendingFinishAt != 0 && now >= t.pendingFinishAt {
		t.pendingFinishAt = 0
		before := t.eventStartResults
		if before == nil {
			before = t.knownResults
		}
		t.finishEvent(before)
		t.knownResults = t.read(eventResults, int(count))
		t.resultsBefore = nil
	} else if !t.eventActive && t.pendingFinishAt == 0 && string(results) != string(t.knownResults) {
		changed := 0
		for i := range results {
			if results[i] != t.knownResults[i] {
				changed++
			}
		}
		if hook {
			// the signal decides; remember the old results in case it arrives right after
			t.resultsBefore, t.resultsChangedAt = t.knownResults, now
		} else if changed == 1 { // a result we didn't see start
			t.finishEvent(t.knownResults)
		}
		if changed > 1 {
			t.resultsBefore = nil // many changes = a profile was loaded
		}
		t.knownResults = results
	}
	if now-t.lastSlow >= 2.0 {
		t.lastSlow = now
		t.slowChecks()
	}
}

func (t *Tracker) finishEvent(before []byte) {
	r := t.run()
	count := int(t.u32(eventCount))
	after := t.read(eventResults, count)
	medal, rating := t.u32(lastMedal), t.u32(lastRating)
	car := decodeLabel(t.u64(selectedCar))
	firstDiff, perfectNow := -1, false
	for i := 0; i < len(before) && i < len(after); i++ {
		if before[i] != after[i] {
			if firstDiff < 0 {
				firstDiff = i
			}
			if after[i] == 0x04 && before[i] != 0x04 {
				perfectNow = true
			}
		}
	}
	region, base := t.currentText()
	eventLabel := t.pendingEvent // the game's current event: also known for replays
	if firstDiff >= 0 {
		eventLabel = decodeLabel(t.u64(eventIDs + uint32(firstDiff)*8))
	}
	crash := isCrashEvent(eventLabel)
	if crash && strings.Contains(t.pendingCar, "CAR") {
		car = t.pendingCar // the car loaded for the junction
	}
	location := t.trackName(region, base, t.pendingTrack)
	t.pendingEvent, t.pendingCar, t.pendingTrack = "", "", ""
	eventName := "Unknown event"
	if eventLabel != "" {
		eventName = t.nameOf(region, base, eventLabel)
	}
	if location == "" && !crash { // race events are named after their location: "Race - Motor City"
		if i := strings.LastIndex(eventName, " - "); i >= 0 {
			location = strings.TrimSpace(eventName[i+3:])
		}
	}
	where := eventName // the event and its location, for the messages
	if location != "" && !strings.Contains(strings.ToUpper(eventName), strings.ToUpper(location)) {
		where += " · " + location
	}
	carName := t.carName(region, base, car)
	shown, shownIdx := "?", -1
	if rating <= 3 {
		bump := uint32(0)
		if medal == 3 {
			bump = 1
		}
		shownIdx = min(4, int(rating+bump))
		shown = ratings[shownIdx]
	}
	difficulty := "Hard"
	if r != nil {
		difficulty = r.Difficulty
	}
	won := passes(difficulty, medal, shownIdx, perfectNow)
	medalName, ok := medals[medal]
	if !ok {
		medalName = "?"
	}
	resultText := medalName + " + " + shown

	if r == nil || !r.Active {
		return
	}
	r.History = append(r.History, HistoryEntry{Time: time.Now().Format("2006-01-02 15:04:05"),
		Event: eventName, Car: carName, Result: resultText, Won: won, Counted: t.counting(), Crash: crash,
		Label: car, Medal: medalKeyOf(medal), Where: location})
	if len(r.History) > 200 {
		r.History = r.History[len(r.History)-200:]
	}
	if !t.counting() {
		t.say(fmt.Sprintf("%s: %s (not counted)", where, resultText), 15, "info")
		t.save()
		return
	}
	t.registerCar(car, carName, crash)
	r.EventsPlayed++
	if won {
		r.EventsWon++
		r.Streak++
		if r.Streak > r.BestStreak {
			r.BestStreak = r.Streak
		}
		t.say(where+": "+resultText, 15, "win")
	} else {
		r.Streak = 0
		c := r.Cars[car]
		if c == nil { // not a car label (shouldn't happen): don't crash
			t.save()
			return
		}
		pool, lives, lost := "Race", &c.Lives, &r.CarsLost
		if crash {
			pool, lives, lost = "Crash", &c.CrashLives, &r.CrashLost
		}
		if *lives > 0 {
			*lives--
		}
		reason := resultText
		if difficulty == "Hard" && medal == 3 && rating == 3 && !perfectNow {
			reason = "already perfected"
		} else if req, ok := requirements[difficulty]; ok {
			reason += ", needs " + req.text
		}
		if *lives == 0 {
			*lost++
			t.say(fmt.Sprintf("%s is wrecked for %s events (%s: %s)", carName, pool, where, reason), 25, "loss")
		} else {
			t.say(fmt.Sprintf("%s lost a %s life, %d left (%s: %s)", carName, pool, *lives, where, reason), 20, "loss")
		}
	}
	// Only the cars the run had before this event (and the one just driven) count: a car this event
	// unlocks can't save the run.
	among := t.carsBefore
	if among != nil {
		among[car] = true
	}
	t.carsBefore = nil
	t.save()
	t.slowChecks()
	if t.allDeadAmong(among) {
		r.Dead = true
		t.save()
		t.syncDeadFlag()
		if t.OnRunDead != nil {
			t.OnRunDead()
		}
	}
}

// ---- cars ----
func (t *Tracker) nameOf(region []byte, base uint32, label string) string {
	if label == "" {
		return "?"
	}
	if region != nil {
		if _, text, _, ok := findText(region, base, label); ok && text != "" && !wreckedNames[text] {
			t.nameCache[label] = text
			return text
		}
	}
	if name, ok := t.nameCache[label]; ok {
		return name
	}
	return label
}

func (t *Tracker) carName(region []byte, base uint32, label string) string {
	if r := t.run(); r != nil {
		if c := r.Cars[label]; c != nil && c.Name != "" && c.Name != "?" && c.Name != label {
			return c.Name
		}
	}
	return t.nameOf(region, base, label)
}

// looksLikeLabel: an internal ID such as HIGHUSCAR1A or K_01DH3E (no spaces, has a digit).
func looksLikeLabel(s string) bool {
	if s == "" || len(s) > 12 || strings.ContainsAny(s, " -") {
		return false
	}
	digit := false
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			digit = true
		case c >= 'A' && c <= 'Z', c == '_':
		default:
			return false
		}
	}
	return digit
}

// repairNames replaces internal IDs saved earlier (cars, history) with real names once they're known.
func (t *Tracker) repairNames(region []byte, base uint32) {
	r := t.run()
	if r == nil || region == nil {
		return
	}
	changed := false
	for label, c := range r.Cars {
		if c.Name == "" || c.Name == "?" || c.Name == label {
			if name := t.nameOf(region, base, label); name != label {
				c.Name, changed = name, true
			}
		}
	}
	for i := range r.History {
		h := &r.History[i]
		for _, field := range []*string{&h.Car, &h.Event} {
			if looksLikeLabel(*field) {
				if name := t.nameOf(region, base, *field); name != *field {
					*field, changed = name, true
				}
			}
		}
	}
	if changed {
		t.save()
	}
}

// registerCar adds a car the first time it's seen and records where it can be used: crash is true for a
// crash junction (car select or event), false for the garage or a Race event.
func (t *Tracker) registerCar(label, name string, crash bool) {
	r := t.run()
	if r == nil || !strings.Contains(label, "CAR") {
		return
	}
	c := r.Cars[label]
	if c == nil {
		c = &Car{Name: name, Lives: r.LivesStart, CrashLives: r.LivesStart}
		r.Cars[label] = c
	} else if (c.Name == "" || c.Name == "?" || c.Name == label) && !wreckedNames[name] {
		c.Name = name
	}
	if crash && !c.InCrash {
		c.InCrash = true
		t.dirty = true
	} else if !crash && !c.InGarage {
		c.InGarage = true
		t.dirty = true
	}
}

// allDead: the run is over when every car is wrecked in the Race pool, or every car in the Crash pool.
func (t *Tracker) allDead() bool { return t.allDeadAmong(nil) }

// allDeadAmong is allDead counting only the cars in among (all cars when among is nil).
func (t *Tracker) allDeadAmong(among map[string]bool) bool {
	r := t.run()
	if r == nil || len(r.Cars) == 0 {
		return false
	}
	// only cars that have a pool count towards it: crash-only cars can't race, garage-only cars can't crash
	race, crash := true, true
	raceCars, crashCars := 0, 0
	for l, c := range r.Cars {
		if among != nil && !among[l] {
			continue
		}
		if c.CanRace() {
			raceCars++
			race = race && c.Lives == 0
		}
		if c.CanCrash() {
			crashCars++
			crash = crash && c.CrashLives == 0
		}
	}
	return (raceCars > 0 && race) || (crashCars > 0 && crash)
}

// deadLabels returns the cars wrecked in the Race pool and in the Crash pool.
func (t *Tracker) deadLabels() (race, crash []string) {
	r := t.run()
	if r == nil || !r.Active || r.Grace {
		return nil, nil
	}
	for l, c := range r.Cars {
		if c.CanRace() && c.Lives == 0 {
			race = append(race, l)
		}
		if c.CanCrash() && c.CrashLives == 0 {
			crash = append(crash, l)
		}
	}
	sort.Strings(race)
	sort.Strings(crash)
	return race, crash
}

// captureEvent remembers the game's current event and car, so replays (which don't change the saved
// results) still get a name and the right pool.
func (t *Tracker) captureEvent() {
	// cars a reward from this event could add don't count towards whether it killed the run
	t.carsBefore = map[string]bool{}
	if r := t.run(); r != nil {
		for l := range r.Cars {
			t.carsBefore[l] = true
		}
	}
	ptr := t.u32(currentEvent)
	if ptr < 0x00100000 || ptr >= 0x01FFFFE0 {
		return
	}
	if l := decodeLabel(t.u64(ptr + 0x18)); strings.HasPrefix(l, "K_") && looksLikeLabel(l) {
		t.pendingEvent = l
	}
	if l := decodeLabel(t.u64(ptr)); strings.Contains(l, "CAR") {
		t.pendingCar = l
	}
	// the track sits among the event object's other labels
	for off := uint32(0x08); off < 0x100; off += 8 {
		if l := decodeLabel(t.u64(ptr + off)); trackLabel.MatchString(l) {
			t.pendingTrack = l
			break
		}
	}
}

// trackLabel matches the game's track labels, US_K1_V1 to AS_S3_V2 (region, route, variant).
var trackLabel = regexp.MustCompile(`^(US|EU|AS)_[A-Z][0-9]_V[0-9]$`)

// trackName is a track's name from the game's text, or "" when the game has none for it.
func (t *Tracker) trackName(region []byte, base uint32, label string) string {
	if label == "" {
		return ""
	}
	for _, l := range []string{label, label[:5]} { // the variant, then the route
		if name := t.nameOf(region, base, l); name != l && name != "?" && !looksLikeLabel(name) {
			return name
		}
	}
	return ""
}

// isCrashEvent: Crash junction labels have DH after the number, e.g. K_01DH1E (Crash - Dock Fight).
func isCrashEvent(label string) bool {
	return len(label) >= 6 && strings.HasPrefix(label, "K_") && label[4:6] == "DH"
}

func deadTableBytes(labels []string) []byte {
	b := make([]byte, 8+8*len(labels))
	binary.LittleEndian.PutUint32(b, uint32(len(labels)))
	for i, l := range labels {
		binary.LittleEndian.PutUint64(b[8+8*i:], encodeLabel(l))
	}
	return b
}

// ---- every 2 seconds: register cars, enforce the dead list and names, check the patch ----
func (t *Tracker) slowChecks() {
	r := t.run()
	region, base := t.currentText()
	t.repairNames(region, base)
	if r != nil && r.Active {
		// cars in the garage and in either player's crash junction car select get their lives
		before := len(r.Cars)
		for li, list := range []uint32{carouselList, crashCarousel, crashCarousel + carouselSize} {
			n := t.u32(list + 0xBA4)
			if n == 0 || n > deadMax {
				continue
			}
			raw := t.read(list, int(n)*8)
			labels := make([]string, n)
			allCars := true
			for i := range labels {
				labels[i] = decodeLabel(binary.LittleEndian.Uint64(raw[i*8:]))
				if !strings.Contains(labels[i], "CAR") {
					allCars = false
				}
			}
			if allCars {
				for _, l := range labels {
					t.registerCar(l, t.carName(region, base, l), li > 0)
				}
			}
		}
		if len(r.Cars) != before || t.dirty {
			t.dirty = false
			t.save()
		}
	}

	raceDead, crashDead := t.deadLabels()
	t.patchOld = t.u32(crashTableRef) != crashTableRefOn && t.u32(crashTableRefISO) != crashTableRefOn
	t.deadLockOn = t.u32(deadLockHook) == deadLockHookOn || t.u32(deadLockHook) == deadLockHookISO
	t.syncDeadFlag()
	garage := raceDead
	if t.patchOld {
		garage = union(raceDead, crashDead) // old patch: one table for both, so block either kind
	}
	for _, tbl := range []struct {
		addr   uint32
		labels []string
	}{{deadTable, garage}, {crashTable, crashDead}} {
		want := deadTableBytes(tbl.labels)
		if string(t.read(tbl.addr, len(want))) != string(want) {
			if err := t.mem.Write(tbl.addr, want); err != nil {
				t.say("Couldn't update the dead-car list: "+err.Error(), 10, "info")
			}
		}
	}
	inRace, inCrash := map[string]bool{}, map[string]bool{}
	for _, l := range raceDead {
		inRace[l] = true
	}
	for _, l := range crashDead {
		inCrash[l] = true
	}
	dead := union(raceDead, crashDead)

	for _, label := range dead {
		if region == nil {
			break
		}
		addr, text, room, ok := findText(region, base, label)
		if !ok {
			continue
		}
		if !wreckedNames[text] {
			t.originals[label] = text
			if c := r.Cars[label]; c != nil && (c.Name == "" || c.Name == "?" || c.Name == label) {
				c.Name = text
			}
		}
		c := r.Cars[label]
		usableRace := c.CanRace() && !inRace[label]
		usableCrash := c.CanCrash() && !inCrash[label]
		newName := wreckedName(room, !usableRace && usableCrash, !usableCrash && usableRace, !usableRace && !usableCrash)
		if text != newName {
			data := utf16le(newName)
			data = append(data, make([]byte, room*2+2-len(data))...)
			_ = t.mem.Write(addr, data)
		}
	}
	if len(dead) == 0 && region != nil {
		t.restoreNames(region, base, false)
	}

	if t.hooksMatch(patchHooks) || t.hooksMatch(isoHooks) {
		t.patchOK = 1
	} else {
		t.patchOK = 0
	}
	t.readAILevel()
}

func (t *Tracker) hooksMatch(hooks map[uint32]uint32) bool {
	for a, v := range hooks {
		if t.u32(a) != v {
			return false
		}
	}
	return true
}

// readAILevel works out which Harder AI level is active from the hook and marker (.pnach) or the
// level word (patched ISO).
func (t *Tracker) readAILevel() {
	level := aiLevels[0]
	var m uint32
	switch t.u32(aiHook) {
	case aiHookOn:
		m = t.u32(aiMarker)
	case aiHookISO, aiHookISOOld:
		m = t.u32(aiLevelISO)
	}
	if m >= 1 && int(m) < len(aiLevels) {
		level = aiLevels[m]
	}
	t.aiLevel = level
	if r := t.run(); r != nil && r.Active && !r.Dead && r.AILevel != level {
		r.AILevel = level
		t.save()
	}
}

// restoreNames puts real names back on wrecked cars, only with a trusted original that fits.
func (t *Tracker) restoreNames(region []byte, base uint32, clearTable bool) {
	r := t.run()
	if !t.mem.Connected() || r == nil {
		return
	}
	defer func() { _ = recover() }()
	if region == nil {
		region, base = t.currentText()
	}
	for label, c := range r.Cars {
		if region == nil {
			break
		}
		original := t.originals[label]
		if original == "" {
			original = c.Name
		}
		if original == "" || wreckedNames[original] || original == label {
			continue
		}
		addr, text, _, ok := findText(region, base, label)
		if !ok || !wreckedNames[text] {
			continue
		}
		if len([]rune(original)) <= textCapacity(region, base, addr) {
			_ = t.mem.Write(addr, append(utf16le(original), 0, 0))
		}
	}
	if clearTable {
		_ = t.mem.Write(deadTable, make([]byte, 8))
		_ = t.mem.Write(crashTable, make([]byte, 8))
	}
}

// ---- snapshot for the web pages ----
func fmtTime(s float64) string {
	n := int(s)
	return fmt.Sprintf("%d:%02d:%02d", n/3600, n/60%60, n%60)
}

func (t *Tracker) Snapshot() map[string]any {
	r := t.run()
	snap := map[string]any{
		"connected":  t.mem.Connected(),
		"wrong_game": t.wrongGame,
		"patch_ok":   t.patchOK,
		"patch_old":  t.patchOK == 1 && (t.patchOld || !t.deadLockOn),
		"pine":       t.mem.HasPine(),
		"ai_level":   "",
		"run":        nil,
	}
	if t.mem.Connected() {
		snap["ai_level"] = t.aiLevel
	}
	if r == nil || !r.Active {
		return snap
	}
	raceCars, crashCars := 0, 0
	for _, c := range r.Cars {
		if c.CanRace() {
			raceCars++
		}
		if c.CanCrash() {
			crashCars++
		}
	}
	snap["run"] = map[string]any{
		"difficulty": r.Difficulty, "requirement": requirements[r.Difficulty].text, "lives_start": r.LivesStart, "grace": r.Grace, "dead": r.Dead,
		"won": r.EventsWon, "played": r.EventsPlayed, "cars_total": len(r.Cars), "cars_lost": r.CarsLost,
		"crash_cars_lost": r.CrashLost, "race_cars_total": raceCars, "crash_cars_total": crashCars,
		"time": fmtTime(r.PlaySeconds), "best_streak": r.BestStreak, "streak": r.Streak, "started": r.Started,
		"ai_level": r.AILevel,
	}
	if t.mem.Connected() {
		func() {
			defer func() { _ = recover() }()
			label := decodeLabel(t.u64(selectedCar))
			if c := r.Cars[label]; c != nil {
				snap["car"] = map[string]any{"name": c.Name, "lives": c.Lives, "crash_lives": c.CrashLives,
					"can_race": c.CanRace(), "can_crash": c.CanCrash(), "history": carHistory(r, label, c.Name)}
			}
		}()
	}
	if t.message != "" && t.now() < t.messageUntil {
		snap["message"] = map[string]any{"text": t.message, "kind": t.messageKind, "id": t.messageID}
	}
	var recent []HistoryEntry
	for i := len(r.History) - 1; i >= 0 && len(recent) < 8; i-- {
		if r.History[i].Counted {
			recent = append(recent, r.History[i])
		}
	}
	snap["recent"] = recent
	return snap
}

func union(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range append(append([]string(nil), a...), b...) {
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out
}

func medalKeyOf(medal uint32) string {
	if m, ok := medalKeys[medal]; ok {
		return m
	}
	return "none"
}

// carHistory: the events one car has played this run, oldest first (the last 40).
func carHistory(r *Run, label, name string) []map[string]any {
	var out []map[string]any
	for _, h := range r.History {
		if h.Label != label && (h.Label != "" || !strings.EqualFold(h.Car, name)) {
			continue
		}
		out = append(out, map[string]any{"event": h.Event, "result": h.Result, "won": h.Won,
			"medal": medalOf(h), "location": h.Where, "crash": h.Crash, "time": h.Time, "counted": h.Counted})
	}
	if len(out) > 40 {
		out = out[len(out)-40:]
	}
	return out
}
