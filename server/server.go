// Package server provides the local HTTP+SSE plumbing every applet's own
// server wraps: loopback binding, idle-timeout self-shutdown, and the job
// lifecycle endpoints (progress stream, cancel, reveal-in-Finder, open).
// An applet embeds *Server in its own Server type and adds whatever
// routes are specific to it (e.g. the one that actually kicks off work) —
// see the "New applet" guide in this repo's README for the exact shape.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/DavidMarsanic/brightencode-appkit/browser"
	"github.com/DavidMarsanic/brightencode-appkit/jobs"
)

// DefaultIdleTimeout is how long an applet waits with no request activity
// and no running job before it self-exits — every applet in the catalog
// uses this same value today.
const DefaultIdleTimeout = 30 * time.Minute

type Server struct {
	Jobs *jobs.Registry
	// Ctx is the applet's top-level context (canceled on Ctrl+C/SIGTERM).
	// App-specific handlers pass this to Jobs.Create.
	Ctx context.Context

	// SkipReveal/SkipOpen opt out of the default "POST /api/reveal" /
	// "POST /api/open" registration in Start, for the rare applet that
	// needs its own (e.g. one that validates the path against a
	// server-side allowlist before revealing/opening it) — set before
	// calling Start, then register the replacement from mount.
	SkipReveal bool
	SkipOpen   bool

	idleTimeout  time.Duration
	lastActivity atomic.Int64
}

// New creates a Server. idleTimeout <= 0 uses DefaultIdleTimeout.
func New(ctx context.Context, idleTimeout time.Duration) *Server {
	if idleTimeout <= 0 {
		idleTimeout = DefaultIdleTimeout
	}
	s := &Server{
		Ctx:         ctx,
		Jobs:        jobs.NewRegistry(),
		idleTimeout: idleTimeout,
	}
	s.touch()
	return s
}

// Start binds a loopback listener on port (0 for automatic), mounts the
// shared job/reveal/open routes plus static under mux, calls mount so the
// caller can register its own app-specific routes (e.g. the one that
// creates a job), serves static last at "GET /", and returns the local
// base URL ("http://127.0.0.1:PORT").
func (s *Server) Start(port int, static fs.FS, mount func(*http.ServeMux)) (string, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", fmt.Errorf("starting local server: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/jobs/{id}/events", s.handleJobEvents)
	mux.HandleFunc("POST /api/jobs/{id}/cancel", s.handleJobCancel)
	if !s.SkipReveal {
		mux.HandleFunc("POST /api/reveal", s.handleReveal)
	}
	if !s.SkipOpen {
		mux.HandleFunc("POST /api/open", s.handleOpen)
	}
	if mount != nil {
		mount(mux)
	}
	mux.Handle("GET /", http.FileServer(http.FS(static)))

	httpSrv := &http.Server{Handler: s.trackActivity(mux)}
	go func() {
		_ = httpSrv.Serve(ln)
	}()
	go s.watchIdle()

	return "http://" + ln.Addr().String(), nil
}

// OpenWindow is a convenience wrapper around
// browser.OpenIfNotHosted(appName, addr) for the common case of opening
// the just-started server's own root URL.
func OpenWindow(appName, addr string) error {
	return browser.OpenIfNotHosted(appName, addr+"/")
}

func (s *Server) trackActivity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.touch()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) touch() {
	s.lastActivity.Store(time.Now().Unix())
}

func (s *Server) watchIdle() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		idleFor := time.Now().Unix() - s.lastActivity.Load()
		if idleFor > int64(s.idleTimeout.Seconds()) && !s.Jobs.HasActive() {
			os.Exit(0)
		}
	}
}

// DecodeJSON decodes r's body into dst, writing a standard 400 response
// and returning false on failure. Shared so every applet's request
// parsing produces the same error shape.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body", "code": "bad-request"})
		return false
	}
	return true
}

// WriteJSON writes v as the JSON response body with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
