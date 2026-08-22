package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aethelgards/tiny-claw/internal/observability"
)

// listResponse mirrors the JSON payload returned by GET /api/sessions.
type listResponse struct {
	Sessions   []observability.SessionData `json:"sessions"`
	Page       int                         `json:"page"`
	Limit      int                         `json:"limit"`
	Total      int                         `json:"total"`
	TotalPages int                         `json:"total_pages"`
}

// testSpanNode mirrors one node of the trace tree returned by
// GET /api/sessions/{id}/traces.
type testSpanNode struct {
	observability.TraceEntry
	Children []*testSpanNode `json:"children"`
}

// testTracesResponse mirrors the JSON payload returned by
// GET /api/sessions/{id}/traces.
type testTracesResponse struct {
	SessionID string          `json:"session_id"`
	Traces    []*testSpanNode `json:"traces"`
}

// testToolsResponse mirrors the JSON payload returned by
// GET /api/sessions/{id}/tools.
type testToolsResponse struct {
	SessionID string                         `json:"session_id"`
	Tools     []observability.ToolCallRecord `json:"tools"`
}

func seedSession(t *testing.T, st *observability.Storage, id string, createdAt time.Time, status observability.SessionStatus, prompt string) {
	t.Helper()
	err := st.SaveSession(observability.SessionData{
		ID:        id,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		Prompt:    prompt,
		Model:     "test-model",
		Status:    status,
	})
	if err != nil {
		t.Fatalf("seed session %s: %v", id, err)
	}
}

func doGet(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestHandleListSessions(t *testing.T) {
	base := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

	newServerWithSessions := func(t *testing.T, n int) (*Server, *observability.Storage) {
		t.Helper()
		st := observability.NewStorage(t.TempDir())
		for i := range n {
			seedSession(t, st, fmt.Sprintf("s%02d", i), base.Add(time.Duration(i)*time.Minute),
				observability.StatusCompleted, fmt.Sprintf("prompt %02d", i))
		}
		return NewServer(st), st
	}

	t.Run("default pagination returns most recent first", func(t *testing.T) {
		server, _ := newServerWithSessions(t, 25)

		rec := doGet(t, server, "/api/sessions")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var body listResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
		}
		if body.Page != 1 || body.Limit != 20 {
			t.Errorf("page/limit = %d/%d, want 1/20", body.Page, body.Limit)
		}
		if body.Total != 25 || body.TotalPages != 2 {
			t.Errorf("total/total_pages = %d/%d, want 25/2", body.Total, body.TotalPages)
		}
		if len(body.Sessions) != 20 {
			t.Fatalf("len(sessions) = %d, want 20", len(body.Sessions))
		}
		if got := body.Sessions[0].ID; got != "s24" {
			t.Errorf("first session ID = %q, want %q (most recent first)", got, "s24")
		}
	})

	t.Run("custom page and limit", func(t *testing.T) {
		server, _ := newServerWithSessions(t, 25)

		rec := doGet(t, server, "/api/sessions?page=2&limit=10")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var body listResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
		}
		if body.Page != 2 || body.Limit != 10 {
			t.Errorf("page/limit = %d/%d, want 2/10", body.Page, body.Limit)
		}
		if body.Total != 25 || body.TotalPages != 3 {
			t.Errorf("total/total_pages = %d/%d, want 25/3", body.Total, body.TotalPages)
		}
		if len(body.Sessions) != 10 {
			t.Fatalf("len(sessions) = %d, want 10", len(body.Sessions))
		}
		if got := body.Sessions[0].ID; got != "s14" {
			t.Errorf("first session ID on page 2 = %q, want %q", got, "s14")
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		st := observability.NewStorage(t.TempDir())
		seedSession(t, st, "run-1", base.Add(1*time.Minute), observability.StatusRunning, "running task")
		seedSession(t, st, "done-1", base.Add(2*time.Minute), observability.StatusCompleted, "done task")
		seedSession(t, st, "fail-1", base.Add(3*time.Minute), observability.StatusFailed, "failed task")
		seedSession(t, st, "done-2", base.Add(4*time.Minute), observability.StatusCompleted, "another done task")
		server := NewServer(st)

		rec := doGet(t, server, "/api/sessions?status=completed")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var body listResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
		}
		if body.Total != 2 {
			t.Fatalf("total = %d, want 2", body.Total)
		}
		wantIDs := map[string]bool{"done-1": true, "done-2": true}
		for _, sess := range body.Sessions {
			if !wantIDs[sess.ID] {
				t.Errorf("unexpected session %q in filtered result", sess.ID)
			}
			delete(wantIDs, sess.ID)
		}
		if len(wantIDs) != 0 {
			t.Errorf("missing sessions from filtered result: %v", wantIDs)
		}
	})

	t.Run("search matches prompt case-insensitively", func(t *testing.T) {
		st := observability.NewStorage(t.TempDir())
		seedSession(t, st, "fix-login", base.Add(1*time.Minute), observability.StatusCompleted, "Fix the login bug")
		seedSession(t, st, "dark-mode", base.Add(2*time.Minute), observability.StatusCompleted, "Add dark mode")
		seedSession(t, st, "fix-signup", base.Add(3*time.Minute), observability.StatusCompleted, "fix signup flow")
		server := NewServer(st)

		rec := doGet(t, server, "/api/sessions?search=FIX")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var body listResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
		}
		if body.Total != 2 {
			t.Fatalf("total = %d, want 2", body.Total)
		}
		got := map[string]bool{}
		for _, sess := range body.Sessions {
			got[sess.ID] = true
		}
		if !got["fix-login"] || !got["fix-signup"] || got["dark-mode"] {
			t.Errorf("search results = %v, want fix-login and fix-signup only", got)
		}
	})

	t.Run("limit above maximum is clamped to 100", func(t *testing.T) {
		server, _ := newServerWithSessions(t, 5)

		rec := doGet(t, server, "/api/sessions?limit=500")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var body listResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
		}
		if body.Limit != 100 {
			t.Errorf("limit = %d, want clamped to 100", body.Limit)
		}
	})

	t.Run("invalid query parameters return 400", func(t *testing.T) {
		server, _ := newServerWithSessions(t, 3)

		cases := []struct {
			name string
			url  string
		}{
			{"non-integer page", "/api/sessions?page=abc"},
			{"zero page", "/api/sessions?page=0"},
			{"negative page", "/api/sessions?page=-1"},
			{"non-integer limit", "/api/sessions?limit=xyz"},
			{"zero limit", "/api/sessions?limit=0"},
			{"negative limit", "/api/sessions?limit=-5"},
			{"unknown status", "/api/sessions?status=paused"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				rec := doGet(t, server, tc.url)
				if rec.Code != http.StatusBadRequest {
					t.Errorf("GET %s status = %d, want %d; body: %s",
						tc.url, rec.Code, http.StatusBadRequest, rec.Body.String())
				}
			})
		}
	})
}

func TestHandleGetSession(t *testing.T) {
	st := observability.NewStorage(t.TempDir())
	seedSession(t, st, "sess-42", time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC),
		observability.StatusRunning, "investigate flaky test")
	server := NewServer(st)

	t.Run("existing session returns details", func(t *testing.T) {
		rec := doGet(t, server, "/api/sessions/sess-42")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var body observability.SessionData
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
		}
		if body.ID != "sess-42" {
			t.Errorf("session ID = %q, want %q", body.ID, "sess-42")
		}
		if body.Prompt != "investigate flaky test" {
			t.Errorf("prompt = %q, want %q", body.Prompt, "investigate flaky test")
		}
	})

	t.Run("unknown session returns 404", func(t *testing.T) {
		rec := doGet(t, server, "/api/sessions/nope")
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
		}
	})
}

func TestHandleGetTraces(t *testing.T) {
	sessionStart := time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC)

	t.Run("builds span tree from flat entries", func(t *testing.T) {
		st := observability.NewStorage(t.TempDir())
		seedSession(t, st, "trace-sess", sessionStart, observability.StatusCompleted, "trace me")

		spans := []observability.TraceEntry{
			{SessionID: "trace-sess", SpanID: "root-1", Name: "agent turn", StartTime: sessionStart, EndTime: sessionStart.Add(time.Second), Status: observability.SpanOK},
			{SessionID: "trace-sess", SpanID: "child-1", ParentID: "root-1", Name: "tool call", StartTime: sessionStart, EndTime: sessionStart.Add(time.Second), Status: observability.SpanOK},
			{SessionID: "trace-sess", SpanID: "root-2", Name: "agent turn", StartTime: sessionStart.Add(2 * time.Second), EndTime: sessionStart.Add(3 * time.Second), Status: observability.SpanOK},
		}
		for _, sp := range spans {
			if err := st.SaveTrace(sp); err != nil {
				t.Fatalf("save trace %s: %v", sp.SpanID, err)
			}
		}
		server := NewServer(st)

		rec := doGet(t, server, "/api/sessions/trace-sess/traces")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var body testTracesResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
		}
		if body.SessionID != "trace-sess" {
			t.Errorf("session_id = %q, want %q", body.SessionID, "trace-sess")
		}
		if len(body.Traces) != 2 {
			t.Fatalf("len(root spans) = %d, want 2; body: %s", len(body.Traces), rec.Body.String())
		}
		if body.Traces[0].SpanID != "root-1" || body.Traces[1].SpanID != "root-2" {
			t.Errorf("root order = [%s, %s], want [root-1 root-2]",
				body.Traces[0].SpanID, body.Traces[1].SpanID)
		}
		children := body.Traces[0].Children
		if len(children) != 1 || children[0].SpanID != "child-1" {
			t.Errorf("root-1 children = %+v, want [child-1]", children)
		}
		if len(children) == 1 && len(children[0].Children) != 0 {
			t.Errorf("child-1 should have no children, got %+v", children[0].Children)
		}
	})

	t.Run("session without traces returns empty array", func(t *testing.T) {
		server := NewServer(observability.NewStorage(t.TempDir()))

		rec := doGet(t, server, "/api/sessions/empty-sess/traces")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var body testTracesResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
		}
		if body.Traces == nil || len(body.Traces) != 0 {
			t.Errorf("traces = %#v, want empty non-nil slice", body.Traces)
		}
	})
}

func TestHandleGetTools(t *testing.T) {
	start := time.Date(2026, 8, 22, 8, 30, 0, 0, time.UTC)

	t.Run("returns tool call records in insertion order", func(t *testing.T) {
		st := observability.NewStorage(t.TempDir())
		seedSession(t, st, "tool-sess", start, observability.StatusCompleted, "use tools")

		records := []observability.ToolCallRecord{
			{SessionID: "tool-sess", SpanID: "t-1", ToolName: "read_file", Arguments: `{"path":"a.go"}`, Result: "ok", StartTime: start},
			{SessionID: "tool-sess", SpanID: "t-2", ToolName: "bash", Arguments: `{"cmd":"go test"}`, Result: "pass", IsError: false, DurationMS: 1200, StartTime: start.Add(time.Second)},
		}
		for _, r := range records {
			if err := st.SaveToolCall(r); err != nil {
				t.Fatalf("save tool call %s: %v", r.SpanID, err)
			}
		}
		server := NewServer(st)

		rec := doGet(t, server, "/api/sessions/tool-sess/tools")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var body testToolsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
		}
		if body.SessionID != "tool-sess" {
			t.Errorf("session_id = %q, want %q", body.SessionID, "tool-sess")
		}
		if len(body.Tools) != 2 {
			t.Fatalf("len(tools) = %d, want 2; body: %s", len(body.Tools), rec.Body.String())
		}
		if body.Tools[0].ToolName != "read_file" || body.Tools[1].ToolName != "bash" {
			t.Errorf("tool order = [%s, %s], want [read_file bash]",
				body.Tools[0].ToolName, body.Tools[1].ToolName)
		}
		if body.Tools[1].DurationMS != 1200 {
			t.Errorf("bash duration_ms = %d, want 1200", body.Tools[1].DurationMS)
		}
	})

	t.Run("session without tool calls returns empty array", func(t *testing.T) {
		server := NewServer(observability.NewStorage(t.TempDir()))

		rec := doGet(t, server, "/api/sessions/no-tools/tools")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var body testToolsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
		}
		if body.Tools == nil || len(body.Tools) != 0 {
			t.Errorf("tools = %#v, want empty non-nil slice", body.Tools)
		}
	})
}
