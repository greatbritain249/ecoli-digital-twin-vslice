package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/example/ecoli-digital-twin-vslice/internal/sim"
)

type Server struct {
	mu         sync.RWMutex
	root       string
	sessions   map[string]*sim.Session
	replayFile string
}

func New(root string) *Server {
	replay := os.Getenv("ECOLI_REPLAY_FILE")
	if replay == "" {
		replay = filepath.Join(root, "data", "replay-example.json")
	}
	return &Server{root: root, sessions: make(map[string]*sim.Session), replayFile: replay}
}
func Listen(addr, root string) error {
	s := New(root)
	defer s.Close()
	httpServer := &http.Server{Addr: addr, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 75 * time.Second}
	return httpServer.ListenAndServe()
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/healthz", s.health)
	mux.HandleFunc("/api/catalog", s.staticJSON("data/catalog.json"))
	mux.HandleFunc("/api/scene", s.staticJSON("data/scene.json"))
	mux.HandleFunc("/api/replay/info", s.replayInfo)
	mux.HandleFunc("/api/simulations", s.simulations)
	mux.HandleFunc("/api/simulations/", s.simulation)
	mux.Handle("/", http.FileServer(http.Dir(filepath.Join(s.root, "apps", "web"))))
	return securityHeaders(mux)
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		next.ServeHTTP(w, r)
	})
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	send(w, http.StatusOK, map[string]interface{}{"status": "ok", "service": "ecoli-api", "model": sim.ModelName})
}
func (s *Server) staticJSON(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			method(w)
			return
		}
		// Static paths are server-defined, never supplied by HTTP clients.
		b, err := os.ReadFile(filepath.Join(s.root, path))
		if err != nil {
			errorJSON(w, 500, "server data unavailable; run make generate")
			log.Printf("read %s: %v", path, err)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(b)
	}
}
func method(w http.ResponseWriter) { errorJSON(w, http.StatusMethodNotAllowed, "method not allowed") }
func errorJSON(w http.ResponseWriter, status int, msg string) {
	send(w, status, map[string]string{"error": msg})
}
func send(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func decode(w http.ResponseWriter, r *http.Request, d interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(d); err != nil {
		return err
	}
	var extra interface{}
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("extra or invalid JSON content")
	}
	return nil
}
func id() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func (s *Server) pruneLocked() {
	for key, item := range s.sessions {
		if item.Expired(60 * time.Minute) {
			item.Close()
			delete(s.sessions, key)
		}
	}
}

type replayData struct {
	SchemaVersion int            `json:"schema_version"`
	Source        string         `json:"source"`
	Frames        []sim.Snapshot `json:"frames"`
}

func (s *Server) readReplay() (replayData, error) {
	var data replayData
	b, err := os.ReadFile(s.replayFile)
	if err != nil {
		return data, err
	}
	if len(b) > 6*1024*1024 {
		return data, fmt.Errorf("replay file too large")
	}
	if err = json.Unmarshal(b, &data); err != nil {
		return data, err
	}
	if data.SchemaVersion != 1 {
		return data, fmt.Errorf("unsupported replay schema version")
	}
	if data.Source == "" {
		return data, fmt.Errorf("replay source provenance must not be empty")
	}
	return data, nil
}
func (s *Server) replayInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	data, err := s.readReplay()
	if err != nil {
		send(w, 200, map[string]interface{}{"available": false, "reason": err.Error()})
		return
	}
	send(w, 200, map[string]interface{}{"available": true, "source": data.Source, "frames": len(data.Frames)})
}
func (s *Server) simulations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Medium sim.Medium `json:"medium"`
		Mode   string     `json:"mode"`
	}
	if err := decode(w, r, &req); err != nil {
		errorJSON(w, 400, "invalid request JSON")
		return
	}
	if req.Medium == "" {
		req.Medium = sim.Rich
	}
	if req.Mode == "" {
		req.Mode = "live"
	}
	var session *sim.Session
	var err error
	switch req.Mode {
	case "live":
		session, err = sim.NewSession(req.Medium)
	case "replay":
		var trajectory replayData
		trajectory, err = s.readReplay()
		if err == nil {
			session, err = sim.NewReplaySession(trajectory.Frames, trajectory.Source)
		}
	default:
		errorJSON(w, 400, "mode must be live or replay")
		return
	}
	if err != nil {
		errorJSON(w, 400, err.Error())
		return
	}
	key, err := id()
	if err != nil {
		session.Close()
		errorJSON(w, 500, "unable to create session")
		return
	}
	s.mu.Lock()
	s.pruneLocked()
	if len(s.sessions) >= 200 {
		s.mu.Unlock()
		session.Close()
		errorJSON(w, 429, "session capacity exceeded")
		return
	}
	s.sessions[key] = session
	s.mu.Unlock()
	send(w, 201, map[string]interface{}{"id": key, "snapshot": session.Snapshot(), "mode": req.Mode})
}
func (s *Server) get(key string) (*sim.Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.sessions[key]
	return item, ok
}
func (s *Server) simulation(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/simulations/")
	parts := strings.Split(rest, "/")
	if len(parts) < 1 || len(parts[0]) != 32 {
		errorJSON(w, 404, "simulation not found")
		return
	}
	key := parts[0]
	item, ok := s.get(key)
	if !ok {
		errorJSON(w, 404, "simulation not found")
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			send(w, 200, map[string]interface{}{"id": key, "snapshot": item.Snapshot()})
		case http.MethodDelete:
			s.mu.Lock()
			if x, found := s.sessions[key]; found {
				x.Close()
				delete(s.sessions, key)
			}
			s.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			method(w)
		}
		return
	}
	if len(parts) != 2 {
		errorJSON(w, 404, "unknown route")
		return
	}
	switch parts[1] {
	case "actions":
		if r.Method != http.MethodPost {
			method(w)
			return
		}
		var req struct {
			Action string     `json:"action"`
			Medium sim.Medium `json:"medium"`
		}
		if err := decode(w, r, &req); err != nil {
			errorJSON(w, 400, "invalid request JSON")
			return
		}
		snapshot, err := item.Action(req.Action, req.Medium)
		if err != nil {
			errorJSON(w, 400, err.Error())
			return
		}
		send(w, 200, snapshot)
	case "events":
		if r.Method != http.MethodGet {
			method(w)
			return
		}
		s.events(w, r, item)
	default:
		errorJSON(w, 404, "unknown route")
	}
}
func (s *Server) events(w http.ResponseWriter, r *http.Request, item *sim.Session) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		errorJSON(w, 500, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	updates, unsubscribe := item.Subscribe()
	defer unsubscribe()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-item.Done():
			return
		case state := <-updates:
			b, _ := json.Marshal(state)
			if _, err := fmt.Fprintf(w, "event: state\ndata: %s\n\n", b); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, session := range s.sessions {
		session.Close()
		delete(s.sessions, id)
	}
}
