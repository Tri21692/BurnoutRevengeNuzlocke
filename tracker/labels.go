package main

import (
	"bytes"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// Criterion-style 64-bit IDs: up to 12 characters packed in base 40.
const idAlphabet = " -/0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_"

func decodeLabel(v uint64) string {
	chars := make([]byte, 12)
	for i := 11; i >= 0; i-- {
		chars[i] = idAlphabet[v%40]
		v /= 40
	}
	return strings.TrimSpace(string(chars))
}

func encodeLabel(s string) uint64 {
	s = strings.ToUpper(s)
	for len(s) < 12 {
		s += " "
	}
	var v uint64
	for _, c := range s[:12] {
		i := strings.IndexRune(idAlphabet, c)
		if i < 0 {
			i = 0
		}
		v = v*40 + uint64(i)
	}
	return v
}

var crcTable = func() (t [256]uint32) {
	for i := range t {
		c := uint32(i)
		for k := 0; k < 8; k++ {
			if c&1 != 0 {
				c = (c >> 1) ^ 0xEDB88320
			} else {
				c >>= 1
			}
		}
		t[i] = c
	}
	return
}()

// textID is the game's label -> text ID hash (function 002F7050): CRC32 table, start FFFFFFFF,
// arithmetic (sign-keeping) shift, no final inversion.
func textID(label string) uint32 {
	crc := int32(-1)
	for _, b := range []byte(label) {
		idx := (uint32(b) ^ uint32(crc)) & 0xFF
		crc = (crc >> 8) ^ int32(crcTable[idx])
	}
	return uint32(crc)
}

func utf16le(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, 2*len(u))
	for i, c := range u {
		binary.LittleEndian.PutUint16(out[2*i:], c)
	}
	return out
}

// findText looks up a label's text in a copy of the game's text table.
func findText(region []byte, base uint32, label string) (addr uint32, text string, room int, ok bool) {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, textID(label))
	off := 0
	for off < len(region) {
		i := bytes.Index(region[off:], key)
		if i < 0 {
			return
		}
		i += off
		if i%4 == 0 {
			start := i + 4
			end := start
			for end+1 < len(region) && !(region[end] == 0 && region[end+1] == 0) {
				end += 2
			}
			u := make([]uint16, (end-start)/2)
			for k := range u {
				u[k] = binary.LittleEndian.Uint16(region[start+2*k:])
			}
			return base + uint32(start), string(utf16.Decode(u)), len(u), true
		}
		off = i + 1
	}
	return
}

// textCapacity: characters that fit at addr, up to the next entry, minus one for the end marker.
func textCapacity(region []byte, base, addr uint32) int {
	pos := int(addr - base)
	end := pos
	for end+1 < len(region) && !(region[end] == 0 && region[end+1] == 0) {
		end += 2
	}
	end += 2
	for end%4 != 0 {
		end++
	}
	for end+4 <= len(region) && binary.LittleEndian.Uint32(region[end:]) == 0 {
		end += 4
	}
	return (end-pos)/2 - 1
}

// Names shown for wrecked cars: in both pools, only in the Race pool, or only in the Crash pool.
var wreckedSets = [][]string{
	{"[WRECKED]", "[WRECK]", "[X]", "X"},
	{"[RACE X]", "[R X]", "[R]", "R"},
	{"[CRASH X]", "[C X]", "[C]", "C"},
}

// benchedSet: Limited Selection's healthy cars left out of the pair.
var benchedSet = []string{"[BENCHED]", "[BENCH]", "[B]", "B"}

func pickName(set []string, room int) string {
	for _, o := range set {
		if len(o) <= room {
			return o
		}
	}
	return ""
}

var wreckedNames = func() map[string]bool {
	m := map[string]bool{}
	for _, set := range append(wreckedSets, benchedSet) {
		for _, n := range set {
			if len(n) > 1 {
				m[n] = true
			}
		}
	}
	m["X"] = true
	return m
}()

// wreckedName picks the name for a car that can't be used for Race events (raceOut), for Crash junctions
// (crashOut), or at all (both): the longest version that fits in room characters.
func wreckedName(room int, raceOut, crashOut, both bool) string {
	set := wreckedSets[0]
	if raceOut && !both {
		set = wreckedSets[1]
	} else if crashOut && !both {
		set = wreckedSets[2]
	}
	for _, o := range set {
		if len(o) <= room {
			return o
		}
	}
	return ""
}
