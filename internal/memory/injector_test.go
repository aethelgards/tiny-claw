package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryInjector_Recent_MergesProjectAndGlobal(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	// Save to both scopes
	_, _ = store.Save(Memory{Type: TypeProject, Content: "项目记忆1", Source: "test"}, ScopeProject)
	_, _ = store.Save(Memory{Type: TypeProject, Content: "项目记忆2", Source: "test"}, ScopeProject)
	_, _ = store.Save(Memory{Type: TypePreference, Content: "全局偏好1", Source: "test"}, ScopeGlobal)

	injector := NewMemoryInjector(store, 400)

	ctx := context.Background()
	memories := injector.Recent(ctx)

	// Should have all 3 (project + global)
	if len(memories) != 3 {
		t.Errorf("expected 3 memories, got %d", len(memories))
	}

	// Project should come first (merged project > global)
	if memories[0].Content != "项目记忆2" || memories[1].Content != "项目记忆1" {
		t.Errorf("project memories should come first, got: %v", memories)
	}
}

func TestMemoryInjector_Recent_RespectsTokenBudget(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	// Save many memories
	for i := 1; i <= 20; i++ {
		content := "记忆 " + string(rune('A'+i%26))
		_, _ = store.Save(Memory{Type: TypeProject, Content: content, Source: "test"}, ScopeProject)
	}

	injector := NewMemoryInjector(store, 100) // Very small budget

	ctx := context.Background()
	memories := injector.Recent(ctx)

	// Should be limited by budget (rough estimate: 400 chars / ~2 = 200 tokens, but we set 100)
	// Each memory ~10 tokens (rough estTokens = 4 + len/2)
	// With 100 budget, should get roughly 10-15 memories
	if len(memories) > 15 {
		t.Errorf("should respect budget, got %d memories", len(memories))
	}
}

func TestMemoryInjector_Recent_EmptyStore(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	injector := NewMemoryInjector(store, 400)
	memories := injector.Recent(context.Background())

	if len(memories) != 0 {
		t.Errorf("empty store should return empty slice")
	}
}

func TestMemoryInjector_Recent_NoTouchOnInject(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	_, _ = store.Save(Memory{Type: TypeProject, Content: "注入不计数", Source: "test"}, ScopeProject)
	_, _ = store.Save(Memory{Type: TypeProject, Content: "另一条记忆", Source: "test"}, ScopeProject)

	injector := NewMemoryInjector(store, 400)

	// Call Recent multiple times
	_ = injector.Recent(context.Background())
	_ = injector.Recent(context.Background())
	_ = injector.Recent(context.Background())

	results := store.Recall("注入", ScopeProject, 10)
	if len(results) != 1 {
		t.Fatalf("recall failed")
	}

	// AccessCount should still be 0 (injection doesn't count)
	if results[0].AccessCount != 0 {
		t.Errorf("injection should not increase AccessCount, got %d", results[0].AccessCount)
	}
}

func TestMemoryInjector_Recent_ScoresByAccessAndRecency(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	now := time.Now()

	// Low access, recent
	_, _ = store.Save(Memory{
		Type:         TypeProject,
		Content:      "low access recent",
		AccessCount:  1,
		LastAccessAt: now.Add(-1 * time.Hour),
		CreatedAt:    now.Add(-2 * time.Hour),
	}, ScopeProject)

	// High access, older
	_, _ = store.Save(Memory{
		Type:         TypeProject,
		Content:      "high access older",
		AccessCount:  10,
		LastAccessAt: now.Add(-24 * time.Hour),
		CreatedAt:    now.Add(-48 * time.Hour),
	}, ScopeProject)

	injector := NewMemoryInjector(store, 400)
	memories := injector.Recent(context.Background())

	// High access should rank higher despite being older
	if len(memories) < 2 {
		t.Fatalf("need 2 memories")
	}
	if memories[0].Content != "high access older" {
		t.Errorf("high access should rank higher, got: %s", memories[0].Content)
	}
}