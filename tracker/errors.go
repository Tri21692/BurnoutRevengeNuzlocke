package main

import "errors"

var (
	errNotConnected = errors.New("not connected to PCSX2")
	errNeedPine     = errors.New("PINE is needed for this write: enable it in PCSX2 (Settings > Advanced)")
)
