package lark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenIDToChatIDMappingSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mapping.json")
	m := NewOpenIDToChatIDMappingWithPath(path)

	m.Set("ou_1", "oc_100")
	m.Set("ou_2", "oc_200")
	if m.Set("", "oc_x"); len(m.mappings) != 2 {
		t.Fatalf("Set 应忽略空 open_id，映射数 = %d, want 2", len(m.mappings))
	}

	// 从磁盘新建实例，验证 Load 恢复
	reloaded := NewOpenIDToChatIDMappingWithPath(path)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if got, ok := reloaded.Get("ou_1"); !ok || got != "oc_100" {
		t.Errorf("Get(ou_1) = %q, %v; want oc_100, true", got, ok)
	}
	if got, ok := reloaded.Get("ou_2"); !ok || got != "oc_200" {
		t.Errorf("Get(ou_2) = %q, %v; want oc_200, true", got, ok)
	}
}

func TestOpenIDToChatIDMappingLoadMissingFile(t *testing.T) {
	m := NewOpenIDToChatIDMappingWithPath(filepath.Join(t.TempDir(), "nope.json"))
	if err := m.Load(); err != nil {
		t.Fatalf("Load 缺失文件应静默返回 nil, got %v", err)
	}
	if len(m.mappings) != 0 {
		t.Errorf("Load 缺失文件后映射应为空, got %d", len(m.mappings))
	}
}

func TestOpenIDToChatIDMappingSetAutoSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mapping.json")
	m := NewOpenIDToChatIDMappingWithPath(path)
	m.Set("ou_9", "oc_9")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Set 应自动落盘文件, err=%v", err)
	}
}

func TestOpenIDToChatIDMappingNoPathNoSave(t *testing.T) {
	m := NewOpenIDToChatIDMapping()
	m.Set("ou_1", "oc_1")
	if err := m.Load(); err != nil {
		t.Fatalf("无 path 时 Load 应返回 nil, got %v", err)
	}
	if got, ok := m.Get("ou_1"); !ok || got != "oc_1" {
		t.Errorf("无 path 时应保持内存映射, Get(ou_1) = %q, %v", got, ok)
	}
}
