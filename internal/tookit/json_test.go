package tookit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAny2Json(t *testing.T) {
	type item struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	got := Any2Json(item{Name: "x", N: 1})
	var back item
	if err := json.Unmarshal([]byte(got), &back); err != nil || back.Name != "x" || back.N != 1 {
		t.Fatalf("Any2Json roundtrip = %q -> %+v, %v", got, back, err)
	}
}

func TestAppendLine(t *testing.T) {
	dir := t.TempDir()
	// 子目录不存在 → 自动创建
	path := filepath.Join(dir, "sub", "nested", "log.jsonl")

	if err := AppendLine(path, "line1"); err != nil {
		t.Fatalf("AppendLine #1: %v", err)
	}
	if err := AppendLine(path, `{"role":"user"}`); err != nil {
		t.Fatalf("AppendLine #2: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "line1\n{\"role\":\"user\"}\n"
	if string(data) != want {
		t.Fatalf("file content = %q, want %q", string(data), want)
	}
}

// 父路径被普通文件占据 → MkdirAll 失败。
func TestAppendLineMkdirError(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendLine(filepath.Join(blocked, "sub", "f.txt"), "x"); err == nil {
		t.Fatal("AppendLine should fail when parent path is a file")
	}
}

// 目标路径是目录 → OpenFile(O_APPEND) 失败。
func TestAppendLineOpenError(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "d")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := AppendLine(sub, "x"); err == nil {
		t.Fatal("AppendLine should fail when target is a directory")
	}
}
