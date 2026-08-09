package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// executeWriteFile runs the write_file tool against the given workDir, path
// and content through its public API, mirroring how the registry invokes
// tools.
func executeWriteFile(t *testing.T, workDir, path, content string) (string, error) {
	t.Helper()
	return executeWriteFileCtx(t, context.Background(), workDir, path, content)
}

func executeWriteFileCtx(t *testing.T, ctx context.Context, workDir, path, content string) (string, error) {
	t.Helper()
	tool := NewWriteFileTool(workDir)
	args, err := json.Marshal(map[string]string{"path": path, "content": content})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return tool.Execute(ctx, args)
}

func TestWriteFileTool_WritesFileContent(t *testing.T) {
	workDir := t.TempDir()

	got, err := executeWriteFile(t, workDir, "note.txt", "hello world")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(got, "note.txt") {
		t.Errorf("result should mention the written path, got %q", got)
	}

	data, err := os.ReadFile(filepath.Join(workDir, "note.txt"))
	if err != nil {
		t.Fatalf("read back file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("file content = %q, want %q", string(data), "hello world")
	}
}

func TestWriteFileTool_CreatesParentDirectories(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join("a", "b", "deep.txt")

	if _, err := executeWriteFile(t, workDir, path, "deep content"); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(workDir, path))
	if err != nil {
		t.Fatalf("read back nested file: %v", err)
	}
	if string(data) != "deep content" {
		t.Errorf("file content = %q, want %q", string(data), "deep content")
	}
}

func TestWriteFileTool_OverwritesExistingFile(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "note.txt"), "old content")

	if _, err := executeWriteFile(t, workDir, "note.txt", "new content"); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(workDir, "note.txt"))
	if err != nil {
		t.Fatalf("read back file: %v", err)
	}
	if string(data) != "new content" {
		t.Errorf("file content = %q, want %q", string(data), "new content")
	}
}

func TestWriteFileTool_EmptyContentCreatesEmptyFile(t *testing.T) {
	workDir := t.TempDir()

	if _, err := executeWriteFile(t, workDir, "empty.txt", ""); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	fi, err := os.Stat(filepath.Join(workDir, "empty.txt"))
	if err != nil {
		t.Fatalf("stat empty file: %v", err)
	}
	if fi.Size() != 0 {
		t.Errorf("file size = %d, want 0", fi.Size())
	}
}

func TestWriteFileTool_InvalidArgsReturnsError(t *testing.T) {
	tool := NewWriteFileTool(t.TempDir())
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path": 123, "content": "x"}`))
	if err == nil {
		t.Fatal("expected error for non-string path arg, got nil")
	}
}

func TestWriteFileTool_RejectsEmptyPath(t *testing.T) {
	_, err := executeWriteFile(t, t.TempDir(), "", "x")
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

func TestWriteFileTool_RejectsAbsolutePath(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret.txt")
	_, err := executeWriteFile(t, t.TempDir(), outside, "x")
	if err == nil {
		t.Fatal("expected error for absolute path, got nil")
	}
}

func TestWriteFileTool_RejectsPathTraversal(t *testing.T) {
	workDir := t.TempDir()

	outside := filepath.Join(t.TempDir(), "secret.txt")
	relOutside, err := filepath.Rel(workDir, outside)
	if err != nil {
		t.Fatalf("compute relative path: %v", err)
	}
	if !strings.Contains(relOutside, "..") {
		t.Fatalf("fixture invariant broken: %q contains no ..", relOutside)
	}

	_, err = executeWriteFile(t, workDir, relOutside, "evil")
	if err == nil {
		t.Fatalf("expected error for traversal path %q, got nil", relOutside)
	}
}

func TestWriteFileTool_RejectsSymlinkDirEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests skipped on windows")
	}
	workDir := t.TempDir()
	outsideDir := t.TempDir()

	link := filepath.Join(workDir, "evil-link")
	if err := os.Symlink(outsideDir, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err := executeWriteFile(t, workDir, filepath.Join("evil-link", "secret.txt"), "evil")
	if err == nil {
		t.Fatal("expected error for directory symlink escaping workDir, got nil")
	}
}

func TestWriteFileTool_RejectsSymlinkFileEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests skipped on windows")
	}
	workDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, outside, "secret")

	link := filepath.Join(workDir, "evil.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err := executeWriteFile(t, workDir, "evil.txt", "overwritten")
	if err == nil {
		t.Fatal("expected error for file symlink escaping workDir, got nil")
	}

	data, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("read outside file: %v", err)
	}
	if string(data) != "secret" {
		t.Errorf("outside file was modified through symlink: %q", string(data))
	}
}

func TestWriteFileTool_AllowsSymlinkWithinWorkDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests skipped on windows")
	}
	workDir := t.TempDir()
	realDir := filepath.Join(workDir, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	link := filepath.Join(workDir, "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if _, err := executeWriteFile(t, workDir, filepath.Join("link", "in.txt"), "ok"); err != nil {
		t.Fatalf("Execute returned error for in-workdir symlink: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(realDir, "in.txt"))
	if err != nil {
		t.Fatalf("read back file through real dir: %v", err)
	}
	if string(data) != "ok" {
		t.Errorf("file content = %q, want %q", string(data), "ok")
	}
}

func TestWriteFileTool_ExceedsMaxContentSize(t *testing.T) {
	workDir := t.TempDir()
	tool := NewWriteFileTool(workDir, WithMaxContentSize(10))
	args, err := json.Marshal(map[string]string{"path": "big.txt", "content": strings.Repeat("a", 20)})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	_, err = tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error when content exceeds maxContentSize, got nil")
	}
	if !strings.Contains(err.Error(), "20") || !strings.Contains(err.Error(), "10") {
		t.Errorf("error should report actual size and limit, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(workDir, "big.txt")); !os.IsNotExist(statErr) {
		t.Error("file should not exist when content exceeds the limit")
	}
}

func TestWriteFileTool_Cancellation(t *testing.T) {
	workDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := executeWriteFileCtx(t, ctx, workDir, "f.txt", "x")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(workDir, "f.txt")); !os.IsNotExist(statErr) {
		t.Error("cancelled write should not create the file")
	}
}
