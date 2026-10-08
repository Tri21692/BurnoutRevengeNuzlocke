package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestChatParsing(t *testing.T) {
	user, text, ok := parsePrivmsg(":Some_User!some_user@some_user.tmi.twitch.tv PRIVMSG #chan :!2 go go")
	if !ok || user != "some_user" || text != "!2 go go" {
		t.Fatalf("got %q %q %v", user, text, ok)
	}
	if _, _, ok := parsePrivmsg(":tmi.twitch.tv 001 justinfan1 :Welcome"); ok {
		t.Fatal("not a chat message")
	}
	for in, want := range map[string]string{"1": "1", "!3": "3", "B": "b", "c please": "c", "4": "", "hello": "", "": ""} {
		if got := ballot(in); got != want {
			t.Fatalf("ballot(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestChatSession runs the chat client against a fake Twitch server.
func TestChatSession(t *testing.T) {
	server, client := net.Pipe()
	c := NewChat()
	c.dial = func() (net.Conn, error) { return client, nil }
	c.SetChannel("#SomeStreamer")
	rd := bufio.NewReader(server)
	for i := 0; i < 2; i++ { // NICK and JOIN
		line, _ := rd.ReadString('\n')
		if i == 1 && line != "JOIN #somestreamer\r\n" {
			t.Fatalf("join: %q", line)
		}
	}
	fmt.Fprintf(server, ":justinfan1!justinfan1@justinfan1.tmi.twitch.tv JOIN #somestreamer\r\n")
	fmt.Fprintf(server, "PING :tmi.twitch.tv\r\n")
	if line, _ := rd.ReadString('\n'); line != "PONG :tmi.twitch.tv\r\n" {
		t.Fatalf("pong: %q", line)
	}
	if !c.Connected() {
		t.Fatal("should be connected after the JOIN")
	}
	fmt.Fprintf(server, ":a!a@a.tmi.twitch.tv PRIVMSG #somestreamer :2\r\n") // before the vote: ignored
	c.StartVote()
	for _, m := range []string{"a :2", "b :2", "a :3", "c :B", "c :b", "d :hi"} {
		u, text, _ := strings.Cut(m, " :")
		fmt.Fprintf(server, ":%s!%s@%s.tmi.twitch.tv PRIVMSG #somestreamer :%s\r\n", u, u, u, text)
	}
	fmt.Fprintf(server, "PING :x\r\n")
	rd.ReadString('\n') // everything before the PING has been handled
	got := c.EndVote()
	if got["2"] != 1 || got["3"] != 1 || got["b"] != 1 || len(got) != 3 {
		t.Fatalf("one vote per viewer per question, the last one counting: %v", got)
	}
	c.SetChannel("")
	if _, s := c.Status(); s != "off" {
		t.Fatalf("status after leaving: %q", s)
	}
}

func TestChatVote(t *testing.T) {
	m := newModeRig(t, "Easy", []string{"HIGHUSCAR1A", "HIGHUSCAR1B"}, nil)
	events := []string{"K_01CDSR", "K_01TFLR", "K_01DH1E", "K_02CRLF", "K_02RH5E", "K_03THLF"}
	for i := 0; i < 169; i++ {
		m.f.w64(eventIDs+uint32(8*i), 0)
	}
	for i, l := range events {
		m.f.w64(eventIDs+uint32(8*i), encodeLabel(l))
		m.f.ram[eventResults+i] = 0xFF
	}
	m.f.w32(rouletteMarker, 1)
	m.f.w32(chaosMarker, 1)
	c := NewChat()
	c.channel, c.status = "x", "connected"
	m.tr.chat = c
	m.tr.twitch.Seconds = 30
	m.tick()
	r := m.tr.run()
	m.playEvent("HIGHUSCAR1A", r.Roulette, true)
	v := m.tr.vote
	if v == nil || len(v.events) != 3 || len(v.mods) != 3 {
		t.Fatalf("a vote on 3 events and 3 modifiers should be open: %+v", v)
	}
	if v.events[0] != r.Roulette || v.mods[0] != r.Chaos {
		t.Fatal("the tracker's own picks come first")
	}
	if got := binary.LittleEndian.Uint64(m.f.ram[rouletteTarget:]); got != encodeLabel("VOTING") {
		t.Fatalf("every event should be locked during the vote: %x", got)
	}
	snap := m.tr.Snapshot()["run"].(map[string]any)
	if snap["vote"] == nil || snap["roulette"] != nil || snap["chaos"] != nil {
		t.Fatal("the snapshot should show the vote instead of the picks")
	}
	if m.tr.Reroll() {
		t.Fatal("no reroll during a vote")
	}
	c.handle("u1", "3")
	c.handle("u2", "3")
	c.handle("u3", "1")
	c.handle("u1", "c")
	m.clock += 31
	m.tick()
	if m.tr.vote != nil || r.Roulette != v.events[2] || r.Chaos != v.mods[2] {
		t.Fatalf("chat's picks should win: %q %q (%v)", r.Roulette, r.Chaos, v)
	}
	if got := binary.LittleEndian.Uint64(m.f.ram[rouletteTarget:]); got != encodeLabel(v.events[2]) {
		t.Fatalf("the picked event should be open: %x", got)
	}
	// no votes: the tracker's picks stand
	m.playEvent("HIGHUSCAR1A", r.Roulette, true)
	v = m.tr.vote
	m.clock += 31
	m.tick()
	if r.Roulette != v.events[0] || r.Chaos != v.mods[0] {
		t.Fatal("with no votes the tracker's picks stand")
	}
	// disconnected chat: no vote
	c.status = "reconnecting"
	m.playEvent("HIGHUSCAR1A", r.Roulette, true)
	if m.tr.vote != nil {
		t.Fatal("no vote without chat")
	}
}
