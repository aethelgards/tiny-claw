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

// executeEditFile runs the edit_file tool against the given workDir and
// inputs through its public API, mirroring how the registry invokes tools.
func executeEditFile(t *testing.T, workDir, path, oldText, newText string) (string, error) {
	t.Helper()
	return executeEditFileCtx(t, context.Background(), workDir, path, oldText, newText)
}

func executeEditFileCtx(t *testing.T, ctx context.Context, workDir, path, oldText, newText string) (string, error) {
	t.Helper()
	tool := NewEditFileTool(workDir)
	args, err := json.Marshal(map[string]string{"path": path, "old_text": oldText, "new_text": newText})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return tool.Execute(ctx, args)
}

func TestEditFileTool_ReplacesExactMatch(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "note.txt"), "hello world")

	got, err := executeEditFile(t, workDir, "note.txt", "hello", "goodbye")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(got, "note.txt") {
		t.Errorf("result should mention the edited path, got %q", got)
	}

	data, err := os.ReadFile(filepath.Join(workDir, "note.txt"))
	if err != nil {
		t.Fatalf("read back file: %v", err)
	}
	if string(data) != "goodbye world" {
		t.Errorf("file content = %q, want %q", string(data), "goodbye world")
	}
}

func TestEditFileTool_ReplacesMultiLineMatch(t *testing.T) {
	workDir := t.TempDir()
	content := "func foo() {\n\tbar()\n}\n"
	writeFile(t, filepath.Join(workDir, "f.go"), content)

	oldText := "func foo() {\n\tbar()\n}"
	newText := "func foo() {\n\tbar()\n\tbaz()\n}"
	if _, err := executeEditFile(t, workDir, "f.go", oldText, newText); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(workDir, "f.go"))
	if err != nil {
		t.Fatalf("read back file: %v", err)
	}
	if string(data) != "func foo() {\n\tbar()\n\tbaz()\n}\n" {
		t.Errorf("file content = %q", string(data))
	}
}

func TestEditFileTool_RequiresUniqueMatch(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "dup.txt"), "aaa\naaa\n")

	_, err := executeEditFile(t, workDir, "dup.txt", "aaa", "bbb")
	if err == nil {
		t.Fatal("expected error for non-unique old_text, got nil")
	}
	if !strings.Contains(err.Error(), "唯一性") {
		t.Errorf("error should ask for more context, got: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(workDir, "dup.txt"))
	if err != nil {
		t.Fatalf("read back file: %v", err)
	}
	if string(data) != "aaa\naaa\n" {
		t.Errorf("file should be unchanged on non-unique match, got %q", string(data))
	}
}

func TestEditFileTool_FuzzyMatchIgnoresIndentation(t *testing.T) {
	workDir := t.TempDir()
	content := "func foo() {\n\tbar()\n\tbaz()\n}\n"
	writeFile(t, filepath.Join(workDir, "f.go"), content)

	// old_text drops the leading tab on each line; L4 must still locate it.
	oldText := "bar()\nbaz()"
	newText := "qux()\nquux()"
	if _, err := executeEditFile(t, workDir, "f.go", oldText, newText); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(workDir, "f.go"))
	if err != nil {
		t.Fatalf("read back file: %v", err)
	}
	// L4 replaces the whole matched line range with newText, dropping the
	// region's indentation — the documented trade-off for tolerating missing
	// indentation in old_text.
	if string(data) != "func foo() {\nqux()\nquux()\n}\n" {
		t.Errorf("file content = %q", string(data))
	}
}

func TestEditFileTool_NotFoundReturnsError(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "note.txt"), "hello world")

	_, err := executeEditFile(t, workDir, "note.txt", "nope", "x")
	if err == nil {
		t.Fatal("expected error for unmatched old_text, got nil")
	}

	data, err := os.ReadFile(filepath.Join(workDir, "note.txt"))
	if err != nil {
		t.Fatalf("read back file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("file should be unchanged on no match, got %q", string(data))
	}
}

func TestEditFileTool_EmptyOldTextReturnsError(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "note.txt"), "hello world")

	_, err := executeEditFile(t, workDir, "note.txt", "   ", "x")
	if err == nil {
		t.Fatal("expected error for empty old_text, got nil")
	}
}

func TestEditFileTool_InvalidArgsReturnsError(t *testing.T) {
	tool := NewEditFileTool(t.TempDir())
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path": 123, "old_text": "a", "new_text": "b"}`))
	if err == nil {
		t.Fatal("expected error for non-string path arg, got nil")
	}
}

func TestEditFileTool_RejectsPathTraversal(t *testing.T) {
	workDir := t.TempDir()

	outside := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, outside, "secret")
	relOutside, err := filepath.Rel(workDir, outside)
	if err != nil {
		t.Fatalf("compute relative path: %v", err)
	}
	if !strings.Contains(relOutside, "..") {
		t.Fatalf("fixture invariant broken: %q contains no ..", relOutside)
	}

	_, err = executeEditFile(t, workDir, relOutside, "secret", "evil")
	if err == nil {
		t.Fatalf("expected error for traversal path %q, got nil", relOutside)
	}

	data, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("read outside file: %v", err)
	}
	if string(data) != "secret" {
		t.Errorf("outside file was modified through traversal: %q", string(data))
	}
}

func TestEditFileTool_RejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests skipped on windows")
	}
	workDir := t.TempDir()
	outsideDir := t.TempDir()
	writeFile(t, filepath.Join(outsideDir, "secret.txt"), "secret")

	link := filepath.Join(workDir, "evil-link")
	if err := os.Symlink(outsideDir, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err := executeEditFile(t, workDir, filepath.Join("evil-link", "secret.txt"), "secret", "evil")
	if err == nil {
		t.Fatal("expected error for symlink escaping workDir, got nil")
	}

	data, err := os.ReadFile(filepath.Join(outsideDir, "secret.txt"))
	if err != nil {
		t.Fatalf("read outside file: %v", err)
	}
	if string(data) != "secret" {
		t.Errorf("outside file was modified through symlink: %q", string(data))
	}
}

func TestEditFileTool_Cancellation(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "note.txt"), "hello world")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := executeEditFileCtx(t, ctx, workDir, "note.txt", "hello", "goodbye")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	data, err := os.ReadFile(filepath.Join(workDir, "note.txt"))
	if err != nil {
		t.Fatalf("read back file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("cancelled edit should not modify the file, got %q", string(data))
	}
}
