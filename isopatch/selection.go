package main

import (
	"errors"
	"path/filepath"
	"strings"
)

// Selection is what goes into a patched ISO. Every part can be chosen on its own, except the modes,
// which need the tracker and so the Nuzlocke rules (the dead-car block and the finished-event signal).
type Selection struct {
	Core    bool     `json:"core"`    // Block dead cars in garage: what nuzlocke.exe needs
	Pause   bool     `json:"pause"`   // Block pause-menu Retry and Quit
	Level   string   `json:"level"`   // Harder AI level, "" for the game's own AI
	Options []string `json:"options"` // option keys: widescreen, fps60, limited, revive, roulette, allcars
}

// needsCore: the modes are run by the tracker.
var needsCore = map[string]bool{"limited": true, "revive": true, "roulette": true}

var tags = map[string]string{"widescreen": "16-9", "fps60": "60 FPS", "limited": "Limited", "revive": "Revive",
	"roulette": "Roulette", "allcars": "All cars"}

func (s Selection) levelIndex() int {
	if s.Level == "" {
		return -1
	}
	return levelIndex(s.Level)
}

func optionIndex(key string) int {
	for i, o := range options {
		if o.key == key {
			return i
		}
	}
	return -1
}

func (s Selection) has(key string) bool {
	for _, k := range s.Options {
		if k == key {
			return true
		}
	}
	return false
}

func (s Selection) Validate() error {
	if s.Level != "" && s.levelIndex() < 0 {
		return errors.New("unknown AI level " + s.Level)
	}
	for _, k := range s.Options {
		if optionIndex(k) < 0 {
			return errors.New("unknown option " + k)
		}
		if needsCore[k] && !s.Core {
			return errors.New(options[optionIndex(k)].name + " needs the Nuzlocke rules (Block dead cars)")
		}
	}
	if !s.Core && !s.Pause && s.Level == "" && len(s.Options) == 0 {
		return errors.New("nothing chosen to patch")
	}
	return nil
}

// Words lists every word the selection writes, in a fixed order.
func (s Selection) Words() []word {
	var out []word
	if s.Core {
		out = append(out, coreWords...)
	}
	if s.Pause {
		out = append(out, pauseWords...)
	}
	if i := s.levelIndex(); i >= 0 {
		out = append(out, levels[i].words...)
	}
	for _, o := range options { // the options' own order, whatever order they were given in
		if s.has(o.key) {
			out = append(out, o.words...)
		}
	}
	return out
}

// Name describes the selection, for the patched ISO's file name: "Nuzlocke, Hard AI, 16-9".
func (s Selection) Name() string {
	var parts []string
	if s.Core {
		parts = append(parts, "Nuzlocke")
	}
	if s.Pause {
		parts = append(parts, "No retry")
	}
	if i := s.levelIndex(); i >= 0 {
		parts = append(parts, levels[i].name+" AI")
	}
	for _, o := range options {
		if s.has(o.key) {
			parts = append(parts, tags[o.key])
		}
	}
	return strings.Join(parts, ", ")
}

// OutputPath is where the patched copy goes: next to the original, with the selection in its name.
func (s Selection) OutputPath(isoPath string) string {
	ext := filepath.Ext(isoPath)
	return strings.TrimSuffix(isoPath, ext) + " (" + s.Name() + ")" + ext
}
