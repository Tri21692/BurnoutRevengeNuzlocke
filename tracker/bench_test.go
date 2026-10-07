package main

import "testing"

// Every healthy car outside the pair is renamed [BENCHED] in the garage; the two picks keep their names.
func TestBenchedNames(t *testing.T) {
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
	names := []string{"FACTORY R160 ST", "NIXON SPECIAL", "EA RACER GT", "REVENGE RACER", "CUSTOM COUPE ULTIMATE"}
	for i, l := range garage5 {
		add(l, names[i])
	}
	add("HIGHEUCAR2S1", "EA RACER GT")
	add("HIGHASCAR1S1", "NIXON")
	add("K_01DH1E", "CRASH - DOCK FIGHT")
	add("K_01DH3E", "CRASH - DECON")
	m.list(carouselList, garage5)
	m.f.w32(limitedMarker, 1)
	for i := 0; i < 5; i++ {
		m.tick()
	}
	r := m.tr.run()
	for i, l := range garage5 {
		got, want := m.f.text(addrs[l], 30), "[BENCHED]"
		if contains(r.RacePick, l) {
			want = names[i]
		}
		if got != want {
			t.Errorf("%s: %q, want %q (picks %v)", l, got, want, r.RacePick)
		}
	}
}
