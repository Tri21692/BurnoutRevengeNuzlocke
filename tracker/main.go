package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

func listen(addr string) (net.Listener, error) { return net.Listen("tcp", addr) }

func main() {
	exe, _ := os.Executable()
	statePath := filepath.Join(filepath.Dir(exe), "nuzlocke_state.json")
	start := time.Now()
	now := func() float64 { return time.Since(start).Seconds() }

	game := NewGame()
	tr := NewTracker(game, statePath, now)
	url := "http://localhost:" + port

	if _, err := startServer(tr, game); err != nil {
		fmt.Println("Couldn't start: port " + port + " is already in use.")
		fmt.Println("Is the tracker already running? Its control page is " + url)
		openBrowser(url)
		fmt.Println("\nPress Enter to close.")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		return
	}

	fmt.Println("Burnout Revenge Nuzlocke tracker v" + version)
	fmt.Println("  Control page:  " + url)
	fmt.Println("  OBS overlay:   " + url + "/overlay   (Browser Source, about 800 x 400)")
	fmt.Println("  Saved run:     " + statePath)
	fmt.Println("\nKeep this window open while you play. Close it to stop the tracker.")
	openBrowser(url)

	ticker := time.NewTicker(250 * time.Millisecond)
	for range ticker.C {
		mu.Lock()
		tr.Poll()
		snap := tr.Snapshot()
		mu.Unlock()
		setSnapshot(snap)
	}
}
