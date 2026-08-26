package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompact_Basic(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	// Save memories with different access counts and ages
	now := time.Now()

	// Recent high-access (should keep)
	m1 := Memory{
		Type:          TypeProject,
		Content:       "keep high access",
		AccessCount:   10,
		LastAccessAt:  now.Add(-1 * time.Hour),
		CreatedAt:     now.Add(-24 * time.Hour),
	}
	_, _ = store.Save(m1, ScopeProject)

	// Old high-access (should keep - score > threshold)
	m2 := Memory{
		Type:          TypeProject,
		Content:       "keep old high access",
		AccessCount:   100,
		LastAccessAt:  now.Add(-24 * time.Hour),
		CreatedAt:     now.Add(-48 * time.Hour),
	}
	_, _ = store.Save(m2, ScopeProject)

	// Old zero-access (should evict - age > 30d, score = 0)
	m3 := Memory{
		Type:         TypeProject,
		Content:      "evict zero access",
		AccessCount:  0,
		CreatedAt:    now.Add(-31 * 24 * time.Hour),
		LastAccessAt: now.Add(-31 * 24 * time.Hour),
	}
	_, _ = store.Save(m3, ScopeProject)

	// Old low-access (should evict - score < 1, age > 30d)
	m4 := Memory{
		Type:         TypeProject,
		Content:      "evict low score",
		AccessCount:  1,
		LastAccessAt: now.Add(-31 * 24 * time.Hour),
		CreatedAt:    now.Add(-32 * 24 * time.Hour),
	}
	_, _ = store.Save(m4, ScopeProject)

	// Recent zero-access but young (should keep - age < 30d)
	m5 := Memory{
		Type:         TypeProject,
		Content:      "keep young zero access",
		AccessCount:  0,
		CreatedAt:    now.Add(-1 * 24 * time.Hour),
		LastAccessAt: now.Add(-1 * 24 * time.Hour),
	}
	_, _ = store.Save(m5, ScopeProject)

	evicted, err := store.Compact(1.0)
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}

	// Should evict m3 and m4
	if evicted != 2 {
		t.Errorf("expected 2 evicted, got %d", evicted)
	}

	// Check remaining
	results := store.Recall("", ScopeProject, 20)
	contents := make(map[string]bool)
	for _, r := range results {
		contents[r.Content] = true
	}

	expectedKeep := []string{"keep high access", "keep old high access", "keep young zero access"}
	for _, c := range expectedKeep {
		if !contents[c] {
			t.Errorf("expected to keep: %s", c)
		}
	}

	if contents["evict zero access"] || contents["evict low score"] {
		t.Errorf("evicted items should not be in results")
	}
}

func TestCompact_ThresholdTuning(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	now := time.Now()
	m := Memory{
		Type:         TypeProject,
		Content:      "threshold test",
		AccessCount:  10,
		LastAccessAt: now.Add(-31 * 24 * time.Hour), // > 30 days
		CreatedAt:    now.Add(-32 * 24 * time.Hour),
	}
	_, _ = store.Save(m, ScopeProject)

	// High threshold should evict
	evicted, _ := store.Compact(100.0)
	if evicted != 1 {
		t.Errorf("high threshold should evict, got %d", evicted)
	}

	// Re-add and test low threshold
	_, _ = store.Save(m, ScopeProject)
	evicted, _ = store.Compact(0.0)
	if evicted != 0 {
		t.Errorf("low threshold should not evict, got %d", evicted)
	}
}

func TestCompact_ArchiveCreated(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	now := time.Now()
	m := Memory{
		Type:         TypeProject,
		Content:      "archive test",
		AccessCount:  0,
		CreatedAt:    now.Add(-31 * 24 * time.Hour),
		LastAccessAt: now.Add(-31 * 24 * time.Hour),
	}
	_, _ = store.Save(m, ScopeProject)

	_, _ = store.Compact(1.0)

	// Check archive file exists
	archivePath := filepath.Join(projectDir, "archive", "project.jsonl")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("archive file not created: %v", err)
	}

	var archived Memory
	if err := json.Unmarshal(data[:len(data)-1], &archived); err != nil {
		t.Fatalf("archive content invalid: %v", err)
	}
	if archived.Content != "archive test" {
		t.Errorf("archive content mismatch: %s", archived.Content)
	}
}