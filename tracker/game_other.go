//go:build !windows

package main

// Stub so the logic can be built and tested on non-Windows systems.
type Game struct{}

func NewGame() *Game                                    { return &Game{} }
func (g *Game) Connected() bool                         { return false }
func (g *Game) Connect() bool                           { return false }
func (g *Game) Drop()                                   {}
func (g *Game) Read(addr uint32, n int) ([]byte, error) { return nil, errNotConnected }
func (g *Game) Write(addr uint32, data []byte) error    { return errNotConnected }
func (g *Game) Serial() string                          { return "" }
func (g *Game) HasPine() bool                           { return false }
func (g *Game) CloseWindows()                           {}
func openBrowser(url string)                            {}
