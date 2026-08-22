package dashboard

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/aethelgards/tiny-claw/internal/observability"
)

const (
	defaultListPage  = 1
	defaultListLimit = 20
	maxListLimit     = 100
)

// sessionListResponse is the JSON payload returned by GET /api/sessions.
type sessionListResponse struct {
	Sessions   []observability.SessionData `json:"sessions"`
	Page       int                         `json:"page"`
	Limit      int                         `json:"limit"`
	Total      int                         `json:"total"`
	TotalPages int                         `json:"total_pages"`
}

// spanNode is a single node of the trace tree: one span plus its child
// spans, nested according to ParentID.
type spanNode struct {
	observability.TraceEntry
	Children []*spanNode `json:"children"`
}

// tracesResponse is the JSON payload returned by
// GET /api/sessions/{id}/traces.
type tracesResponse struct {
	SessionID string      `json:"session_id"`
	Traces    []*spanNode `json:"traces"`
}

// toolsResponse is the JSON payload returned by
// GET /api/sessions/{id}/tools.
type toolsResponse struct {
	SessionID string                     `json:"session_id"`
	Tools     []observability.ToolCallRecord `json:"tools"`
}

// handleListSessions responds to GET /api/sessions with a paginated,
// filterable list of sessions ordered most recent first.
//
// Query parameters:
//   - page:   1-based page number (default 1)
//   - limit:  items per page, clamped to 100 (default 20)
//   - status: optional filter, one of running/completed/failed
//   - search: optional case-insensitive substring match on prompt content
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	page := defaultListPage
	if raw := r.URL.Query().Get("page"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			writeError(w, http.StatusBadRequest, "invalid page parameter")
			return
		}
		page = v
	}

	limit := defaultListLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			writeError(w, http.StatusBadRequest, "invalid limit parameter")
			return
		}
		limit = min(v, maxListLimit)
	}

	statusFilter := observability.SessionStatus(r.URL.Query().Get("status"))
	switch statusFilter {
	case "", observability.StatusRunning, observability.StatusCompleted, observability.StatusFailed:
	default:
		writeError(w, http.StatusBadRequest, "invalid status parameter")
		return
	}

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))

	all, err := s.storage.ListSessions(0, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	filtered := make([]observability.SessionData, 0, len(all))
	for _, sess := range all {
		if statusFilter != "" && sess.Status != statusFilter {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(sess.Prompt), search) {
			continue
		}
		filtered = append(filtered, sess)
	}

	total := len(filtered)
	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}
	start := min((page-1)*limit, total)
	end := min(start+limit, total)

	writeJSON(w, http.StatusOK, sessionListResponse{
		Sessions:   filtered[start:end],
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

// handleGetSession responds to GET /api/sessions/{id} with the full
// session record, or 404 if the session does not exist.
func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	session, err := s.storage.LoadSession(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// handleGetTraces responds to GET /api/sessions/{id}/traces with the
// session's spans assembled into a tree via their parent IDs. Spans whose
// parent is missing are promoted to roots. A session without traces yields
// an empty tree.
func (s *Server) handleGetTraces(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	entries, err := s.storage.LoadTraces(sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load traces")
		return
	}
	writeJSON(w, http.StatusOK, tracesResponse{
		SessionID: sessionID,
		Traces:    buildSpanTree(entries),
	})
}

// buildSpanTree nests flat trace entries into a forest of root spans keyed
// by ParentID, preserving insertion order at every level.
func buildSpanTree(entries []observability.TraceEntry) []*spanNode {
	nodes := make([]*spanNode, len(entries))
	bySpanID := make(map[string]*spanNode, len(entries))
	for i, entry := range entries {
		node := &spanNode{TraceEntry: entry, Children: []*spanNode{}}
		nodes[i] = node
		bySpanID[entry.SpanID] = node
	}

	roots := []*spanNode{}
	for _, node := range nodes {
		if node.ParentID == "" {
			roots = append(roots, node)
			continue
		}
		parent, ok := bySpanID[node.ParentID]
		if !ok {
			// Orphaned span: promote to root rather than dropping it.
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	return roots
}

// handleGetTools responds to GET /api/sessions/{id}/tools with the
// session's tool call records in insertion order.
func (s *Server) handleGetTools(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	records, err := s.storage.LoadToolCalls(sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load tool calls")
		return
	}
	writeJSON(w, http.StatusOK, toolsResponse{
		SessionID: sessionID,
		Tools:     records,
	})
}

// writeError writes a JSON error payload with the given status code.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
