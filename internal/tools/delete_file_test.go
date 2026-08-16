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

// executeDeleteFile runs the delete_file tool against the given workDir and
// path through its public API, mirroring how the registry invokes tools.
func executeDeleteFile(t *testing.T, workDir, path string) (string, error) {
	t.Helper()
	tool := NewDeleteFileTool(workDir)
	args, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return tool.Execute(context.Background(), args)
}

func TestDeleteFileTool_DeletesFile(t *testing.T) {
	workDir := t.TempDir()
	target := filepath.Join(workDir, "note.txt")
	writeFile(t, target, "hello world")

	got, err := executeDeleteFile(t, workDir, "note.txt")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(got, "note.txt") {
		t.Errorf("output should mention the deleted path, got %q", got)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("file should be gone after delete, stat err: %v", err)
	}
}

func TestDeleteFileTool_DeletesFileInNestedDirectory(t *testing.T) {
	workDir := t.TempDir()
	dir := filepath.Join(workDir, "a", "b")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(dir, "deep.txt")
	writeFile(t, target, "deep content")

	if _, err := executeDeleteFile(t, workDir, filepath.Join("a", "b", "deep.txt")); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("file should be gone, stat err: %v", err)
	}
}

func TestDeleteFileTool_MissingFileReturnsError(t *testing.T) {
	_, err := executeDeleteFile(t, t.TempDir(), "does-not-exist.txt")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestDeleteFileTool_InvalidArgsReturnsError(t *testing.T) {
	tool := NewDeleteFileTool(t.TempDir())
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path": 123}`))
	if err == nil {
		t.Fatal("expected error for non-string path arg, got nil")
	}
}

func TestDeleteFileTool_RejectsEmptyPath(t *testing.T) {
	_, err := executeDeleteFile(t, t.TempDir(), "")
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

func TestDeleteFileTool_RejectsAbsolutePath(t *testing.T) {
	workDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, outside, "secret")

	_, err := executeDeleteFile(t, workDir, outside)
	if err == nil {
		t.Fatal("expected error for absolute path, got nil")
	}
	if _, statErr := os.Stat(outside); statErr != nil {
		t.Fatalf("outside file must not be touched, stat err: %v", statErr)
	}
}

func TestDeleteFileTool_RejectsPathTraversal(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "inside.txt"), "inside")

	outside := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, outside, "secret")

	relOutside, err := filepath.Rel(workDir, outside)
	if err != nil {
		t.Fatalf("compute relative path: %v", err)
	}
	if !strings.Contains(relOutside, "..") {
		t.Fatalf("fixture invariant broken: %q contains no ..", relOutside)
	}

	_, err = executeDeleteFile(t, workDir, relOutside)
	if err == nil {
		t.Fatalf("expected error for traversal path %q, got nil", relOutside)
	}
	if _, statErr := os.Stat(outside); statErr != nil {
		t.Fatalf("outside file must not be touched, stat err: %v", statErr)
	}
}

func TestDeleteFileTool_RejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests skipped on windows")
	}
	workDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, outside, "secret")

	link := filepath.Join(workDir, "evil-link.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err := executeDeleteFile(t, workDir, "evil-link.txt")
	if err == nil {
		t.Fatal("expected error for symlink escaping workDir, got nil")
	}
	if _, statErr := os.Stat(outside); statErr != nil {
		t.Fatalf("outside target must not be touched, stat err: %v", statErr)
	}
}

func TestDeleteFileTool_DeletesSymlinkNotTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests skipped on windows")
	}
	workDir := t.TempDir()
	real := filepath.Join(workDir, "real.txt")
	writeFile(t, real, "real content")

	link := filepath.Join(workDir, "link.txt")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if _, err := executeDeleteFile(t, workDir, "link.txt"); err != nil {
		t.Fatalf("Execute returned error for in-workdir symlink: %v", err)
	}
	if _, err := os.Stat(link); !os.IsNotExist(err) {
		t.Errorf("symlink should be removed, stat err: %v", err)
	}
	if _, err := os.Stat(real); err != nil {
		t.Errorf("symlink target must survive deletion of the link, stat err: %v", err)
	}
}

func TestDeleteFileTool_RejectsDirectory(t *testing.T) {
	workDir := t.TempDir()
	dir := filepath.Join(workDir, "keep")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(dir, "inner.txt"), "inner")

	_, err := executeDeleteFile(t, workDir, "keep")
	if err == nil {
		t.Fatal("expected error for directory path, got nil")
	}
	if _, statErr := os.Stat(dir); statErr != nil {
		t.Fatalf("directory must not be deleted, stat err: %v", statErr)
	}
}

func TestDeleteFileTool_Cancellation(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "f.txt"), "content")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tool := NewDeleteFileTool(workDir)
	args, err := json.Marshal(map[string]string{"path": "f.txt"})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	_, err = tool.Execute(ctx, args)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
