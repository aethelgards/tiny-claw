package dashboard

import (
	"net/http"
	"sort"
	"time"

	"github.com/aethelgards/tiny-claw/internal/observability"
)

// overviewResponse is the JSON payload returned by GET /api/stats/overview.
type overviewResponse struct {
	TotalSessions int     `json:"total_sessions"`
	TotalCost     float64 `json:"total_cost"`
	TotalTokens   int64   `json:"total_tokens"`
	AvgDurationMS int64   `json:"avg_duration_ms"`
	SuccessRate   float64 `json:"success_rate"`
}

// dailyEntry represents aggregated stats for a single day.
type dailyEntry struct {
	Date     string  `json:"date"`
	Sessions int     `json:"sessions"`
	Cost     float64 `json:"cost"`
	Tokens   int64   `json:"tokens"`
}

// dailyResponse is the JSON payload returned by GET /api/stats/daily.
type dailyResponse struct {
	Daily []dailyEntry `json:"daily"`
}

// modelEntry represents aggregated stats for a single model.
type modelEntry struct {
	Model    string  `json:"model"`
	Sessions int     `json:"sessions"`
	Cost     float64 `json:"cost"`
	Tokens   int64   `json:"tokens"`
}

// modelsResponse is the JSON payload returned by GET /api/stats/models.
type modelsResponse struct {
	Models []modelEntry `json:"models"`
}

// toolEntry represents aggregated stats for a single tool.
type toolEntry struct {
	Tool         string `json:"tool"`
	Count        int    `json:"count"`
	AvgDurationMS int64  `json:"avg_duration_ms"`
}

// toolsStatsResponse is the JSON payload returned by GET /api/stats/tools.
type toolsStatsResponse struct {
	Tools []toolEntry `json:"tools"`
}

// handleStatsOverview responds to GET /api/stats/overview with aggregate stats.
func (s *Server) handleStatsOverview(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.storage.ListSessions(0, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load sessions")
		return
	}

	var totalCost float64
	var totalTokens int64
	var totalDuration int64
	var completed int

	for _, sess := range sessions {
		totalCost += sess.TotalCost
		totalTokens += sess.TotalTokens.TotalTokens
		totalDuration += sess.DurationMS
		if sess.Status == observability.StatusCompleted {
			completed++
		}
	}

	avgDuration := int64(0)
	successRate := float64(0)
	if len(sessions) > 0 {
		avgDuration = totalDuration / int64(len(sessions))
		successRate = float64(completed) / float64(len(sessions))
	}

	writeJSON(w, http.StatusOK, overviewResponse{
		TotalSessions: len(sessions),
		TotalCost:     totalCost,
		TotalTokens:   totalTokens,
		AvgDurationMS: avgDuration,
		SuccessRate:   successRate,
	})
}

// handleStatsDaily responds to GET /api/stats/daily with per-day aggregates.
func (s *Server) handleStatsDaily(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.storage.ListSessions(0, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load sessions")
		return
	}

	dailyMap := make(map[string]*dailyEntry)
	for _, sess := range sessions {
		date := sess.CreatedAt.Format(time.DateOnly)
		entry, ok := dailyMap[date]
		if !ok {
			entry = &dailyEntry{Date: date}
			dailyMap[date] = entry
		}
		entry.Sessions++
		entry.Cost += sess.TotalCost
		entry.Tokens += sess.TotalTokens.TotalTokens
	}

	daily := make([]dailyEntry, 0, len(dailyMap))
	for _, entry := range dailyMap {
		daily = append(daily, *entry)
	}
	sort.Slice(daily, func(i, j int) bool {
		return daily[i].Date > daily[j].Date
	})

	writeJSON(w, http.StatusOK, dailyResponse{Daily: daily})
}

// handleStatsModels responds to GET /api/stats/models with per-model aggregates.
func (s *Server) handleStatsModels(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.storage.ListSessions(0, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load sessions")
		return
	}

	modelMap := make(map[string]*modelEntry)
	for _, sess := range sessions {
		entry, ok := modelMap[sess.Model]
		if !ok {
			entry = &modelEntry{Model: sess.Model}
			modelMap[sess.Model] = entry
		}
		entry.Sessions++
		entry.Cost += sess.TotalCost
		entry.Tokens += sess.TotalTokens.TotalTokens
	}

	models := make([]modelEntry, 0, len(modelMap))
	for _, entry := range modelMap {
		models = append(models, *entry)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Sessions > models[j].Sessions
	})

	writeJSON(w, http.StatusOK, modelsResponse{Models: models})
}

// handleStatsTools responds to GET /api/stats/tools with per-tool aggregates.
func (s *Server) handleStatsTools(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.storage.ListSessions(0, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load sessions")
		return
	}

	toolMap := make(map[string]*toolEntry)
	for _, sess := range sessions {
		records, err := s.storage.LoadToolCalls(sess.ID)
		if err != nil {
			continue
		}
		for _, record := range records {
			entry, ok := toolMap[record.ToolName]
			if !ok {
				entry = &toolEntry{Tool: record.ToolName}
				toolMap[record.ToolName] = entry
			}
			entry.Count++
			entry.AvgDurationMS += record.DurationMS
		}
	}

	tools := make([]toolEntry, 0, len(toolMap))
	for _, entry := range toolMap {
		if entry.Count > 0 {
			entry.AvgDurationMS = entry.AvgDurationMS / int64(entry.Count)
		}
		tools = append(tools, *entry)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Count > tools[j].Count
	})

	writeJSON(w, http.StatusOK, toolsStatsResponse{Tools: tools})
}
