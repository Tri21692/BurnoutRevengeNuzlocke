package main

// The patcher's window: a page served on localhost and opened in the browser, with a button for every
// part of the mod. The patching itself runs here, in Go.

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed web/patcher.html
var webFS embed.FS

type patchJob struct {
	State string `json:"state"` // idle, working, done, error
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Out   string `json:"out"`
	Error string `json:"error"`
}

type gui struct {
	mu       sync.Mutex
	iso      string
	job      patchJob
	lastPing time.Time
	quit     chan struct{}
}

func runGUI(iso string) error {
	g := &gui{iso: iso, job: patchJob{State: "idle"}, quit: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, _ := webFS.ReadFile("web/patcher.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/api/state", g.state)
	mux.HandleFunc("/api/browse", g.browse)
	mux.HandleFunc("/api/iso", g.setISO)
	mux.HandleFunc("/api/patch", g.patch)
	mux.HandleFunc("/api/open", g.openFolder)
	mux.HandleFunc("/api/quit", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		g.stop()
	})

	ln, err := net.Listen("tcp", "127.0.0.1:8766")
	if err != nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0") // another copy is open: any free port
		if err != nil {
			return err
		}
	}
	url := "http://" + ln.Addr().String()
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()

	fmt.Println("Burnout Revenge Nuzlocke - ISO patcher v" + version)
	fmt.Println()
	fmt.Println("The patcher is open in your browser: " + url)
	fmt.Println("Close the patcher's page, or this window, when you're done.")
	openBrowser(url)

	// Stop once the page has been closed for a while (it checks in every 2 seconds), unless it's patching.
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-g.quit:
			_ = srv.Close()
			return nil
		case <-tick.C:
			g.mu.Lock()
			idle := !g.lastPing.IsZero() && time.Since(g.lastPing) > 30*time.Second && g.job.State != "working"
			g.mu.Unlock()
			if idle {
				_ = srv.Close()
				return nil
			}
		}
	}
}

func (g *gui) stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.job.State == "working" {
		return
	}
	select {
	case <-g.quit:
	default:
		close(g.quit)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func (g *gui) state(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	g.lastPing = time.Now()
	out := map[string]any{"iso": g.iso, "job": g.job, "version": version, "levels": levelNames(),
		"insane_warning": insaneWarning, "can_browse": canBrowse}
	if g.iso != "" {
		out["iso_name"] = filepath.Base(g.iso)
		if st, err := os.Stat(g.iso); err != nil || st.IsDir() {
			out["iso_error"] = "can't find that file"
		}
	}
	g.mu.Unlock()
	writeJSON(w, out)
}

func levelNames() []string {
	var out []string
	for _, l := range levels {
		out = append(out, l.name)
	}
	return out
}

func (g *gui) browse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	path, err := browseISO()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if path != "" {
		g.mu.Lock()
		g.iso = path
		g.mu.Unlock()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (g *gui) setISO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	g.mu.Lock()
	g.iso = strings.Trim(strings.TrimSpace(r.URL.Query().Get("path")), `"`)
	g.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// patch starts patching the chosen ISO with the posted selection; /api/state reports how it goes.
func (g *gui) patch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	var sel Selection
	if err := json.NewDecoder(r.Body).Decode(&sel); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.job.State == "working" {
		http.Error(w, "already patching", http.StatusConflict)
		return
	}
	if g.iso == "" {
		http.Error(w, "choose your Burnout Revenge ISO first", http.StatusBadRequest)
		return
	}
	if err := sel.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	iso, out := g.iso, sel.OutputPath(g.iso)
	g.job = patchJob{State: "working", Out: out}
	go func() {
		err := patchISO(iso, out, sel, func(done, total int64) {
			g.mu.Lock()
			g.job.Done, g.job.Total = done, total
			g.mu.Unlock()
		})
		g.mu.Lock()
		defer g.mu.Unlock()
		if err != nil {
			g.job.State, g.job.Error = "error", err.Error()
			return
		}
		g.job.State = "done"
		fmt.Println("Made:", out)
	}()
	w.WriteHeader(http.StatusNoContent)
}

func (g *gui) openFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	g.mu.Lock()
	out := g.job.Out
	done := g.job.State == "done"
	g.mu.Unlock()
	if !done {
		http.Error(w, "nothing made yet", http.StatusBadRequest)
		return
	}
	showInFolder(out)
	w.WriteHeader(http.StatusNoContent)
}

var errNoDialog = errors.New("no file dialog here: type or paste the ISO's path")
