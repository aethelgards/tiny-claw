package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 回归：Forget 必须把删除持久化到对应类型的文件，
// 否则重启后记忆复活，并会在目录下生成垃圾文件 .jsonl。
func TestMemoryStore_Forget_PersistsToTypeFile(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)
	m := Memory{Type: TypeProject, Content: "将被删除的记忆", Source: "test"}
	id, _ := store.Save(m, ScopeProject)

	if err := store.Forget(id, ScopeProject); err != nil {
		t.Fatalf("Forget: %v", err)
	}

	projFile := filepath.Join(projectDir, "project.jsonl")
	data, err := os.ReadFile(projFile)
	if err != nil {
		t.Fatalf("project.jsonl should exist: %v", err)
	}
	if len(data) > len("{\"version\":1}\n") {
		t.Errorf("project.jsonl still contains deleted memory on disk:\n%s", string(data))
	}

	if _, err := os.Stat(filepath.Join(projectDir, ".jsonl")); err == nil {
		t.Errorf("garbage file '.jsonl' created in project dir")
	}

	// 重启（重新加载）后不应复活
	store2, _ := NewMemoryStore(globalDir, projectDir)
	if results := store2.Recall("删除", ScopeProject, 10); len(results) != 0 {
		t.Errorf("deleted memory resurrected after reload: %+v", results[0])
	}
}

// 回归：Touch 的计数更新必须持久化到对应类型的文件，重启后不丢失。
func TestMemoryStore_Touch_PersistsToTypeFile(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)
	m := Memory{Type: TypeProject, Content: "Touch持久化", Source: "test"}
	id, _ := store.Save(m, ScopeProject)

	store.Touch(id, ScopeProject)

	store2, _ := NewMemoryStore(globalDir, projectDir)
	results := store2.Recall("Touch持久化", ScopeProject, 10)
	if len(results) != 1 {
		t.Fatalf("memory lost after reload")
	}
	if results[0].AccessCount != 1 {
		t.Errorf("AccessCount should persist after reload, got %d", results[0].AccessCount)
	}

	if _, err := os.Stat(filepath.Join(projectDir, ".jsonl")); err == nil {
		t.Errorf("garbage file '.jsonl' created in project dir")
	}
}

// 回归：一次 Compact 归档多条记忆时，归档文件必须保留全部（追加写而非覆盖）。
func TestMemoryStore_Compact_ArchiveAppendsAll(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	old := time.Now().Add(-40 * 24 * time.Hour)
	m1 := Memory{Type: TypeProject, Content: "归档记忆一", Source: "test", CreatedAt: old}
	id1, _ := store.Save(m1, ScopeProject)
	m2 := Memory{Type: TypeProject, Content: "归档记忆二", Source: "test", CreatedAt: old}
	id2, _ := store.Save(m2, ScopeProject)

	store.mu.Lock()
	for _, mm := range store.memories[ScopeProject][TypeProject] {
		if mm.ID == id1 || mm.ID == id2 {
			mm.CreatedAt = old
		}
	}
	store.mu.Unlock()

	n, err := store.Compact(0)
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 evicted, got %d", n)
	}

	archPath := filepath.Join(projectDir, "archive", "project.jsonl")
	data, _ := os.ReadFile(archPath)
	lines := 0
	for _, line := range splitLines(string(data)) {
		if line != "" {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("archive should contain 2 lines (append), got %d:\n%s", lines, string(data))
	}
}
