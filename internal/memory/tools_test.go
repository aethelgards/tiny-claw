package memory

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestSaveMemoryTool(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)
	tool := NewSaveMemoryTool(store)

	// Test explicit save with type
	args := json.RawMessage(`{"content":"显式保存测试","type":"project","scope":"project"}`)
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result == "" {
		t.Errorf("should return success message")
	}

	// Test auto type inference (contains project keywords)
	args2 := json.RawMessage(`{"content":"本项目用 Go 1.26 + gin","scope":"project"}`)
	result2, err := tool.Execute(context.Background(), args2)
	if err != nil {
		t.Fatalf("Execute 2: %v", err)
	}
	if result2 == "" {
		t.Errorf("should return success message")
	}

	// Test preference inference
	args3 := json.RawMessage(`{"content":"用户偏好中文回复","scope":"global"}`)
	result3, err := tool.Execute(context.Background(), args3)
	if err != nil {
		t.Fatalf("Execute 3: %v", err)
	}
	if result3 == "" {
		t.Errorf("should return success message")
	}

	// Test error inference
	args4 := json.RawMessage(`{"content":"报错：connection refused 解决方法","scope":"project"}`)
	result4, err := tool.Execute(context.Background(), args4)
	if err != nil {
		t.Fatalf("Execute 4: %v", err)
	}
	if result4 == "" {
		t.Errorf("should return success message")
	}

	// Test default type inference (no keywords) -> project
	args5 := json.RawMessage(`{"content":"这是一个普通记忆"}`)
	result5, err := tool.Execute(context.Background(), args5)
	if err != nil {
		t.Fatalf("Execute 5: %v", err)
	}
	if result5 == "" {
		t.Errorf("should return success message")
	}

	// Verify saved
	results := store.Recall("", ScopeProject, 20)
	if len(results) < 4 {
		t.Errorf("should have at least 4 project memories, got %d", len(results))
	}
}

func TestRecallMemoryTool(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)
	saveTool := NewSaveMemoryTool(store)
	recallTool := NewRecallMemoryTool(store)

	// Save some memories
	_, _ = saveTool.Execute(context.Background(), json.RawMessage(`{"content":"项目记忆A","type":"project","scope":"project"}`))
	_, _ = saveTool.Execute(context.Background(), json.RawMessage(`{"content":"项目记忆B","type":"project","scope":"project"}`))
	_, _ = saveTool.Execute(context.Background(), json.RawMessage(`{"content":"全局偏好","type":"preferences","scope":"global"}`))

	// Test recall with query
	args := json.RawMessage(`{"query":"记忆A","scope":"project","limit":5}`)
	result, err := recallTool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result == "" {
		t.Errorf("should return results")
	}

	// Test recall with empty query (recent)
	args2 := json.RawMessage(`{"query":"","scope":"project","limit":10}`)
	result2, err := recallTool.Execute(context.Background(), args2)
	if err != nil {
		t.Fatalf("Execute 2: %v", err)
	}
	if result2 == "" {
		t.Errorf("should return recent")
	}

	// Test recall with auto scope (project then global)
	args3 := json.RawMessage(`{"query":"偏好"}`)
	result3, err := recallTool.Execute(context.Background(), args3)
	if err != nil {
		t.Fatalf("Execute 3: %v", err)
	}
	if result3 == "" {
		t.Errorf("should find global preference")
	}
}

func TestForgetMemoryTool(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)
	saveTool := NewSaveMemoryTool(store)
	forgetTool := NewForgetMemoryTool(store)

	// Save a memory
	args := json.RawMessage(`{"content":"待删除记忆","type":"project","scope":"project"}`)
	_, err := saveTool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Forget by ID (using a non-existent ID - just test tool can be called without panic)
	_, _ = forgetTool.Execute(context.Background(), json.RawMessage(`{"id":"test-id"}`))
	// Should not panic
}