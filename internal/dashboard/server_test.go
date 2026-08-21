package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aethelgards/tiny-claw/internal/observability"
)

func TestServerHealthCheck(t *testing.T) {
	storage := observability.NewStorage(t.TempDir())
	server := NewServer(storage)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/health status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
	}
	if body.Status != "ok" {
		t.Errorf("response status field = %q, want %q", body.Status, "ok")
	}
}
