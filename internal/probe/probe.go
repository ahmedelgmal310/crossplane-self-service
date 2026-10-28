// Package probe implements env-probe, the stand-in application that every
// AppEnvironment runs in CI. It reports whether the environment's database,
// delivered by the platform as the app-db Secret, is reachable.
package probe

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Info describes the database session the probe obtained.
type Info struct {
	User          string `json:"user"`
	Database      string `json:"database"`
	ServerVersion string `json:"server_version"`
}

// DB is the subset of database behaviour the HTTP handlers need.
type DB interface {
	// Info runs a read-only query and reports who the probe is connected as.
	Info(ctx context.Context) (Info, error)
	// Write inserts one row into a probe table and returns the number of rows
	// in it, proving the delivered credentials can write.
	Write(ctx context.Context) (int64, error)
}

// Server serves the probe endpoints. A nil DB means the environment has no
// database, which is a valid configuration.
type Server struct {
	db       DB
	version  string
	hostname string
	timeout  time.Duration
}

// New returns a Server. db may be nil.
func New(db DB, version, hostname string) *Server {
	return &Server{db: db, version: version, hostname: hostname, timeout: 2 * time.Second}
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", s.readyz)
	mux.HandleFunc("GET /db", s.dbInfo)
	mux.HandleFunc("POST /db/write", s.dbWrite)
	mux.HandleFunc("GET /{$}", s.root)
	return mux
}

func (s *Server) root(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service":             "env-probe",
		"version":             s.version,
		"pod":                 s.hostname,
		"database_configured": s.db != nil,
	})
}

// readyz is the readiness probe. With a database configured the pod is only
// ready when a query succeeds, so a Ready AppEnvironment implies the app has
// reached its database with the delivered credentials.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ready": true, "database": "not configured"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	if _, err := s.db.Info(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ready": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ready": true, "database": "connected"})
}

type dbResponse struct {
	Configured bool    `json:"configured"`
	Connected  bool    `json:"connected"`
	Info       *Info   `json:"info,omitempty"`
	Rows       int64   `json:"rows,omitempty"`
	LatencyMS  float64 `json:"latency_ms,omitempty"`
	Error      string  `json:"error,omitempty"`
}

func (s *Server) dbInfo(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, dbResponse{})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	start := time.Now()
	info, err := s.db.Info(ctx)
	resp := dbResponse{Configured: true, LatencyMS: millis(time.Since(start))}
	if err != nil {
		resp.Error = err.Error()
		writeJSON(w, http.StatusServiceUnavailable, resp)
		return
	}
	resp.Connected = true
	resp.Info = &info
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) dbWrite(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusConflict, dbResponse{Error: "no database configured"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	start := time.Now()
	rows, err := s.db.Write(ctx)
	resp := dbResponse{Configured: true, LatencyMS: millis(time.Since(start))}
	if err != nil {
		resp.Error = err.Error()
		writeJSON(w, http.StatusServiceUnavailable, resp)
		return
	}
	resp.Connected = true
	resp.Rows = rows
	writeJSON(w, http.StatusOK, resp)
}

func millis(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
