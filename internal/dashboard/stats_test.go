package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aethelgards/tiny-claw/internal/observability"
)

func TestHandleStatsOverview(t *testing.T) {
	storage := observability.NewStorage(t.TempDir())
	server := NewServer(storage)

	// Seed test data
	storage.SaveSession(observability.SessionData{
		ID:          "s1",
		Model:       "glm-4.6",
		Status:      observability.StatusCompleted,
		TotalCost:   0.01,
		TotalTokens: observability.TokenUsage{TotalTokens: 1000},
		DurationMS:  100,
	})
	storage.SaveSession(observability.SessionData{
		ID:          "s2",
		Model:       "glm-4.6",
		Status:      observability.StatusRunning,
		TotalCost:   0.02,
		TotalTokens: observability.TokenUsage{TotalTokens: 2000},
		DurationMS:  200,
	})

	req := httptest.NewRequest("GET", "/api/stats/overview", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp overviewResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp.TotalSessions != 2 {
		t.Errorf("expected 2 sessions, got %d", resp.TotalSessions)
	}
	if resp.TotalCost != 0.03 {
		t.Errorf("expected cost 0.03, got %f", resp.TotalCost)
	}
	if resp.TotalTokens != 3000 {
		t.Errorf("expected 3000 tokens, got %d", resp.TotalTokens)
	}
}

func TestHandleStatsDaily(t *testing.T) {
	storage := observability.NewStorage(t.TempDir())
	server := NewServer(storage)

	now := time.Now()
	storage.SaveSession(observability.SessionData{
		ID:          "s1",
		CreatedAt:   now,
		Status:      observability.StatusCompleted,
		TotalCost:   0.01,
		TotalTokens: observability.TokenUsage{TotalTokens: 1000},
		DurationMS:  100,
	})
	storage.SaveSession(observability.SessionData{
		ID:          "s2",
		CreatedAt:   now,
		Status:      observability.StatusFailed,
		TotalCost:   0.02,
		TotalTokens: observability.TokenUsage{TotalTokens: 2000},
		DurationMS:  300,
	})

	req := httptest.NewRequest("GET", "/api/stats/daily", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp dailyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(resp.Daily) != 1 {
		t.Fatalf("expected 1 day entry, got %d", len(resp.Daily))
	}
	if resp.Daily[0].Sessions != 2 {
		t.Errorf("expected 2 sessions, got %d", resp.Daily[0].Sessions)
	}
	if resp.Daily[0].Tokens != 3000 {
		t.Errorf("expected 3000 tokens, got %d", resp.Daily[0].Tokens)
	}
	if resp.Daily[0].AvgDurationMS != 200 {
		t.Errorf("expected avg duration 200ms, got %d", resp.Daily[0].AvgDurationMS)
	}
	if resp.Daily[0].SuccessRate != 0.5 {
		t.Errorf("expected success rate 0.5, got %f", resp.Daily[0].SuccessRate)
	}
}

func TestHandleStatsModels(t *testing.T) {
	storage := observability.NewStorage(t.TempDir())
	server := NewServer(storage)

	storage.SaveSession(observability.SessionData{
		ID:          "s1",
		Model:       "glm-4.6",
		TotalCost:   0.01,
		TotalTokens: observability.TokenUsage{TotalTokens: 1000},
	})
	storage.SaveSession(observability.SessionData{
		ID:          "s2",
		Model:       "deepseek-v4",
		TotalCost:   0.02,
		TotalTokens: observability.TokenUsage{TotalTokens: 2000},
	})

	req := httptest.NewRequest("GET", "/api/stats/models", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp modelsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(resp.Models) != 2 {
		t.Fatalf("expected 2 model entries, got %d", len(resp.Models))
	}
}

func TestHandleStatsTools(t *testing.T) {
	storage := observability.NewStorage(t.TempDir())
	server := NewServer(storage)

	storage.SaveSession(observability.SessionData{
		ID:     "s1",
		Status: observability.StatusCompleted,
	})
	storage.SaveToolCall(observability.ToolCallRecord{
		SessionID:  "s1",
		ToolName:   "read_file",
		DurationMS: 100,
	})
	storage.SaveToolCall(observability.ToolCallRecord{
		SessionID:  "s1",
		ToolName:   "read_file",
		DurationMS: 200,
	})
	storage.SaveToolCall(observability.ToolCallRecord{
		SessionID:  "s1",
		ToolName:   "read_file",
		DurationMS: 300,
		IsError:    true,
	})
	storage.SaveToolCall(observability.ToolCallRecord{
		SessionID:  "s1",
		ToolName:   "write_file",
		DurationMS: 50,
	})

	req := httptest.NewRequest("GET", "/api/stats/tools", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp toolsStatsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(resp.Tools) != 2 {
		t.Fatalf("expected 2 tool entries, got %d", len(resp.Tools))
	}
	for _, tool := range resp.Tools {
		switch tool.ToolName {
		case "read_file":
			if tool.Calls != 3 {
				t.Errorf("expected read_file calls 3, got %d", tool.Calls)
			}
			if tool.Errors != 1 {
				t.Errorf("expected read_file errors 1, got %d", tool.Errors)
			}
			if tool.AvgDurationMS != 200 {
				t.Errorf("expected read_file avg duration 200ms, got %d", tool.AvgDurationMS)
			}
		case "write_file":
			if tool.Calls != 1 {
				t.Errorf("expected write_file calls 1, got %d", tool.Calls)
			}
			if tool.Errors != 0 {
				t.Errorf("expected write_file errors 0, got %d", tool.Errors)
			}
		default:
			t.Errorf("unexpected tool %q", tool.ToolName)
		}
	}
}
