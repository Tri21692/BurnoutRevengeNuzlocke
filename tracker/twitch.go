package main

// Twitch chat votes: the tracker reads a channel's chat anonymously (no login, read only) and lets
// chat vote on the next Event Roulette event (1, 2 or 3) and the next Chaos modifier (A, B or C).

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	twitchAddr         = "irc.chat.twitch.tv:6697"
	defaultVoteSeconds = 45
)

// TwitchSettings are kept next to the run's state, so they carry over between runs.
type TwitchSettings struct {
	Channel string `json:"channel"`
	Seconds int    `json:"vote_seconds"`
}

var channelName = regexp.MustCompile(`^[a-z0-9_]{3,25}$`)

// Chat is the connection to a channel's chat. Votes come in on its own goroutine.
type Chat struct {
	mu         sync.Mutex
	channel    string
	status     string // "off", "connecting", "connected", or an error
	collecting bool
	ballots    map[string]string // user -> 1/2/3/a/b/c, while a vote is open
	stop       chan struct{}
	dial       func() (net.Conn, error)
}

func NewChat() *Chat {
	return &Chat{status: "off", dial: func() (net.Conn, error) {
		d := &net.Dialer{Timeout: 10 * time.Second}
		return tls.DialWithDialer(d, "tcp", twitchAddr, &tls.Config{ServerName: "irc.chat.twitch.tv"})
	}}
}

// SetChannel joins a channel's chat ("" leaves).
func (c *Chat) SetChannel(channel string) {
	channel = cleanChannel(channel)
	c.mu.Lock()
	defer c.mu.Unlock()
	if channel == c.channel && c.stop != nil {
		return
	}
	if c.stop != nil {
		close(c.stop)
		c.stop = nil
	}
	c.channel = channel
	if channel == "" {
		c.status = "off"
		return
	}
	stop := make(chan struct{})
	c.stop = stop
	c.status = "connecting"
	go c.loop(channel, stop)
}

func (c *Chat) setStatus(stop chan struct{}, s string) {
	c.mu.Lock()
	if c.stop == stop {
		c.status = s
	}
	c.mu.Unlock()
}

// Status: "off", "connecting", "connected" or an error message.
func (c *Chat) Status() (channel, status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.channel, c.status
}

func (c *Chat) Connected() bool {
	_, s := c.Status()
	return s == "connected"
}

// loop stays in the channel's chat until stopped, reconnecting when the connection drops.
func (c *Chat) loop(channel string, stop chan struct{}) {
	wait := 5 * time.Second
	for {
		err := c.session(channel, stop)
		select {
		case <-stop:
			return
		default:
		}
		msg := "reconnecting"
		if err != nil {
			msg = "can't reach Twitch chat, retrying (" + err.Error() + ")"
		}
		c.setStatus(stop, msg)
		select {
		case <-stop:
			return
		case <-time.After(wait):
		}
		if wait < time.Minute {
			wait *= 2
		}
	}
}

func (c *Chat) session(channel string, stop chan struct{}) error {
	conn, err := c.dial()
	if err != nil {
		return err
	}
	defer conn.Close()
	go func() { <-stop; conn.Close() }()
	fmt.Fprintf(conn, "NICK justinfan%d\r\nJOIN #%s\r\n", 10000+rand.Intn(80000), channel)
	rd := bufio.NewReader(conn)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(7 * time.Minute)) // Twitch pings every 5 minutes
		line, err := rd.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "PING"):
			fmt.Fprintf(conn, "PONG%s\r\n", strings.TrimPrefix(line, "PING"))
		case strings.Contains(line, " JOIN #"+channel):
			c.setStatus(stop, "connected")
		case strings.Contains(line, " RECONNECT"):
			return nil
		default:
			if user, text, ok := parsePrivmsg(line); ok {
				c.handle(user, text)
			}
		}
	}
}

// parsePrivmsg reads ":user!user@user.tmi.twitch.tv PRIVMSG #channel :text".
func parsePrivmsg(line string) (user, text string, ok bool) {
	if !strings.HasPrefix(line, ":") {
		return "", "", false
	}
	prefix, rest, found := strings.Cut(line[1:], " PRIVMSG ")
	if !found {
		return "", "", false
	}
	_, text, found = strings.Cut(rest, " :")
	if !found {
		return "", "", false
	}
	user, _, _ = strings.Cut(prefix, "!")
	return strings.ToLower(user), text, user != ""
}

// ballot reads a vote from a chat message: its first word, 1, 2, 3, A, B or C (a leading ! is fine).
func ballot(text string) string {
	f := strings.Fields(strings.ToLower(text))
	if len(f) == 0 {
		return ""
	}
	k := strings.TrimPrefix(f[0], "!")
	switch k {
	case "1", "2", "3", "a", "b", "c":
		return k
	}
	return ""
}

// handle counts a chat message as a vote while one is open. Each viewer has one vote per question:
// a later message changes it.
func (c *Chat) handle(user, text string) {
	k := ballot(text)
	if k == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.collecting {
		return
	}
	kind := "event"
	if k >= "a" {
		kind = "mod"
	}
	c.ballots[user+"|"+kind] = k
}

func (c *Chat) StartVote() {
	c.mu.Lock()
	c.collecting, c.ballots = true, map[string]string{}
	c.mu.Unlock()
}

// Tally counts the votes so far per option (1, 2, 3, a, b, c).
func (c *Chat) Tally() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]int{}
	for _, k := range c.ballots {
		out[k]++
	}
	return out
}

func (c *Chat) EndVote() map[string]int {
	out := c.Tally()
	c.mu.Lock()
	c.collecting, c.ballots = false, nil
	c.mu.Unlock()
	return out
}

// ---- the tracker's side ----

// chatVote is an open vote: the candidates for the next event and modifier. The first of each is the
// tracker's own pick, which stands on a tie or with no votes.
type chatVote struct {
	events     []string
	eventNames []string
	mods       []string
	ends       float64
}

var voteKeys = [2][3]string{{"1", "2", "3"}, {"a", "b", "c"}}

func (t *Tracker) settingsPath() string {
	return strings.TrimSuffix(t.statePath, ".json") + "_twitch.json"
}

func (t *Tracker) loadTwitch() {
	t.twitch = TwitchSettings{Seconds: defaultVoteSeconds}
	if data, err := os.ReadFile(t.settingsPath()); err == nil {
		_ = json.Unmarshal(data, &t.twitch)
	}
	if t.twitch.Seconds < 10 || t.twitch.Seconds > 300 {
		t.twitch.Seconds = defaultVoteSeconds
	}
	if t.chat != nil {
		t.chat.SetChannel(t.twitch.Channel)
	}
}

func cleanChannel(channel string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(channel), "#"))
}

// validChannel: a Twitch channel name, or "" for none.
func validChannel(channel string) bool {
	channel = cleanChannel(channel)
	return channel == "" || channelName.MatchString(channel)
}

// SetTwitch changes the channel and vote length, and saves them. A bad channel name is refused.
func (t *Tracker) SetTwitch(channel string, seconds int) bool {
	channel = cleanChannel(channel)
	if !validChannel(channel) {
		return false
	}
	if seconds < 10 || seconds > 300 {
		seconds = defaultVoteSeconds
	}
	t.twitch = TwitchSettings{Channel: channel, Seconds: seconds}
	if data, err := json.MarshalIndent(t.twitch, "", " "); err == nil {
		_ = os.WriteFile(t.settingsPath(), data, 0o644)
	}
	if t.chat != nil {
		t.chat.SetChannel(channel)
	}
	if channel == "" && t.vote != nil {
		t.closeVote()
	}
	return true
}

// openVote puts the next event and modifier to chat, when it's connected and those modes are on.
func (t *Tracker) openVote() {
	if t.chat == nil || !t.chat.Connected() {
		return
	}
	r := t.run()
	v := &chatVote{ends: t.now() + float64(t.twitch.Seconds)}
	if t.rouletteActive() && r.Roulette != "" {
		v.events = append([]string{r.Roulette}, t.rouletteCandidates(2, r.Roulette)...)
		region, base := t.currentText()
		for _, l := range v.events {
			v.eventNames = append(v.eventNames, t.nameOf(region, base, l))
		}
	}
	if t.chaosActive() && r.Chaos != "" {
		v.mods = append([]string{r.Chaos}, t.chaosCandidates(2, r.Chaos)...)
	}
	if len(v.events) < 2 && len(v.mods) < 2 {
		return
	}
	if len(v.events) < 2 {
		v.events = nil
	}
	if len(v.mods) < 2 {
		v.mods = nil
	}
	t.vote = v
	t.chat.StartVote()
	var ask []string
	if v.events != nil {
		ask = append(ask, "1, 2 or 3 for the next event")
	}
	if v.mods != nil {
		ask = append(ask, "A, B or C for the Chaos modifier")
	}
	t.later("Chat vote: type "+strings.Join(ask, ", ")+fmt.Sprintf(" (%d s)", t.twitch.Seconds), 15, "info")
}

// tickVote closes the vote when its time is up.
func (t *Tracker) tickVote() {
	if t.vote != nil && (t.now() >= t.vote.ends || !t.counting()) {
		t.closeVote()
	}
}

// winner is the option with the most votes; the tracker's own pick (the first) on a tie.
func winner(n int, keys [3]string, tally map[string]int) (int, int) {
	best := 0
	for i := 1; i < n; i++ {
		if tally[keys[i]] > tally[keys[best]] {
			best = i
		}
	}
	return best, tally[keys[best]]
}

func (t *Tracker) closeVote() {
	v := t.vote
	t.vote = nil
	if t.chat == nil {
		return
	}
	tally := t.chat.EndVote()
	r := t.run()
	if r == nil || !t.counting() {
		return
	}
	var said []string
	if v.events != nil {
		i, n := winner(len(v.events), voteKeys[0], tally)
		if v.events[i] != r.Roulette {
			r.Roulette, r.RouletteName = v.events[i], v.eventNames[i]
		}
		said = append(said, fmt.Sprintf("%s (Rank %d, %s)", r.RouletteName, eventRank(r.Roulette), plural2(n, "vote")))
	}
	if v.mods != nil {
		i, n := winner(len(v.mods), voteKeys[1], tally)
		if v.mods[i] != r.Chaos {
			t.setChaos(v.mods[i])
		}
		said = append(said, fmt.Sprintf("%s (%s)", chaosByID(r.Chaos).name, plural2(n, "vote")))
	}
	t.dirty = true
	t.later("Chat picked: "+strings.Join(said, " · "), 15, "info")
}

func plural2(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func (t *Tracker) voteSnapshot() map[string]any {
	v := t.vote
	if v == nil {
		return nil
	}
	tally := t.chat.Tally()
	var events, mods []map[string]any
	for i, l := range v.events {
		events = append(events, map[string]any{"key": voteKeys[0][i], "name": v.eventNames[i],
			"rank": eventRank(l), "votes": tally[voteKeys[0][i]]})
	}
	for i, id := range v.mods {
		m := chaosByID(id)
		mods = append(mods, map[string]any{"key": strings.ToUpper(voteKeys[1][i]), "name": m.name, "text": m.text,
			"good": m.good, "votes": tally[voteKeys[1][i]]})
	}
	return map[string]any{"events": events, "mods": mods, "left": max(0, int(v.ends-t.now()+0.99)),
		"seconds": t.twitch.Seconds}
}

func (t *Tracker) twitchSnapshot() map[string]any {
	out := map[string]any{"channel": t.twitch.Channel, "seconds": t.twitch.Seconds, "status": "off"}
	if t.chat != nil {
		_, out["status"] = t.chat.Status()
	}
	return out
}
