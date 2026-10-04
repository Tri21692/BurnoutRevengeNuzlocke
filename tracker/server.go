package main

import (
	"embed"
	"encoding/json"
	"net/http"
	"sync"
)

//go:embed web/control.html web/overlay.html
var webFS embed.FS

const port = "8765"

var (
	mu       sync.Mutex
	snapMu   sync.Mutex
	snapJSON = []byte(`{"run":null}`)
)

func setSnapshot(s map[string]any) {
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	snapMu.Lock()
	snapJSON = b
	snapMu.Unlock()
}

func page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, _ := webFS.ReadFile("web/" + name)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
	}
}

func startServer(tr *Tracker, game *Game) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		page("control.html")(w, r)
	})
	mux.HandleFunc("/overlay", page("overlay.html"))
	mux.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		snapMu.Lock()
		b := snapJSON
		snapMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(b)
	})
	action := func(f func()) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "use POST", http.StatusMethodNotAllowed)
				return
			}
			mu.Lock()
			f()
			setSnapshot(tr.Snapshot())
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		}
	}
	mux.HandleFunc("/api/newrun", func(w http.ResponseWriter, r *http.Request) {
		action(func() { tr.StartRun(r.URL.Query().Get("difficulty")) })(w, r)
	})
	mux.HandleFunc("/api/grace", action(tr.Grace))
	mux.HandleFunc("/api/shutdown", action(game.CloseWindows))

	srv := &http.Server{Addr: "127.0.0.1:" + port, Handler: mux}
	ln, err := listen(srv.Addr)
	if err != nil {
		return nil, err
	}
	go func() { _ = srv.Serve(ln) }()
	return srv, nil
}
