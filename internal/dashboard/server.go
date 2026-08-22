// Package dashboard provides the HTTP server for the tiny-claw
// visualization dashboard. It exposes a JSON API backed by the
// observability storage layer.
package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/aethelgards/tiny-claw/internal/observability"
)

// Server is the HTTP server for the dashboard.
type Server struct {
	storage *observability.Storage
	mux     *http.ServeMux
}

// NewServer creates a new dashboard server serving API routes backed by
// the given observability storage.
func NewServer(storage *observability.Storage) *Server {
	s := &Server{
		storage: storage,
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

// routes registers the API endpoints on the server's mux.
func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("GET /api/sessions/{id}/traces", s.handleGetTraces)
	s.mux.HandleFunc("GET /api/sessions/{id}/tools", s.handleGetTools)
	s.mux.HandleFunc("GET /api/stats/overview", s.handleStatsOverview)
	s.mux.HandleFunc("GET /api/stats/daily", s.handleStatsDaily)
	s.mux.HandleFunc("GET /api/stats/models", s.handleStatsModels)
	s.mux.HandleFunc("GET /api/stats/tools", s.handleStatsTools)
}

// Start starts the HTTP server listening on addr. It blocks until the
// listener fails or is closed.
func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

// Handler returns the http.Handler for testing and embedding.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// handleHealth responds to GET /api/health with a JSON status payload.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeJSON writes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
	}
}
