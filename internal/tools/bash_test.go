package tools

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// executeBash runs the bash tool against the given workDir and command
// through its public API, mirroring how the registry invokes tools.
func executeBash(t *testing.T, workDir, command string) (string, error) {
	t.Helper()
	tool := NewBashTool(workDir)
	args, err := json.Marshal(map[string]string{"command": command})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return tool.Execute(context.Background(), args)
}

func TestBashTool_ReturnsCommandOutput(t *testing.T) {
	got, err := executeBash(t, t.TempDir(), "echo hello-world")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(got, "hello-world") {
		t.Errorf("output should contain command output, got %q", got)
	}
	if !strings.Contains(got, "退出码: 0") {
		t.Errorf("output should report exit code 0, got %q", got)
	}
}

func TestBashTool_RunsInWorkDir(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "marker.txt"), "present")

	got, err := executeBash(t, workDir, "ls marker.txt")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(got, "marker.txt") {
		t.Errorf("output should list the workdir file, got %q", got)
	}
}

func TestBashTool_NonZeroExitIsDataNotError(t *testing.T) {
	got, err := executeBash(t, t.TempDir(), "exit 3")
	if err != nil {
		t.Fatalf("Execute returned error for non-zero exit: %v", err)
	}
	if !strings.Contains(got, "退出码: 3") {
		t.Errorf("output should report exit code 3, got %q", got)
	}
}

func TestBashTool_CapturesStderr(t *testing.T) {
	got, err := executeBash(t, t.TempDir(), "echo to-stdout; echo to-stderr >&2")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(got, "to-stdout") || !strings.Contains(got, "to-stderr") {
		t.Errorf("output should contain both stdout and stderr, got %q", got)
	}
}

func TestBashTool_EmptyCommandReturnsError(t *testing.T) {
	_, err := executeBash(t, t.TempDir(), "   ")
	if err == nil {
		t.Fatal("expected error for empty command, got nil")
	}
}

func TestBashTool_InvalidArgsReturnsError(t *testing.T) {
	tool := NewBashTool(t.TempDir())
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"command": 123}`))
	if err == nil {
		t.Fatal("expected error for non-string command arg, got nil")
	}
}

func TestBashTool_TruncatesLongOutput(t *testing.T) {
	orig := maxOutputLen
	maxOutputLen = 64
	t.Cleanup(func() { maxOutputLen = orig })

	long := strings.Repeat("a", 500)
	got, err := executeBash(t, t.TempDir(), "echo "+long)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(got, "截断") {
		t.Errorf("output should contain truncation marker, got %q", got)
	}
	// Allow slack for the truncation marker itself.
	if len(got) > maxOutputLen+128 {
		t.Errorf("output is %d bytes, exceeds maxOutputLen %d", len(got), maxOutputLen)
	}
}

func TestBashTool_ArgTimeoutOverridesDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep syntax differs on windows")
	}
	workDir := t.TempDir()
	// Default timeout is 120s; a 100ms arg timeout must kill a sleep 5.
	args, err := json.Marshal(map[string]any{"command": "sleep 5", "timeout": 0.1})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	tool := NewBashTool(workDir)
	_, err = tool.Execute(context.Background(), args)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestBashTool_WithTimeoutOption(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep syntax differs on windows")
	}
	workDir := t.TempDir()
	tool := NewBashTool(workDir, WithTimeout(100*time.Millisecond))
	args, err := json.Marshal(map[string]string{"command": "sleep 5"})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	_, err = tool.Execute(context.Background(), args)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestBashTool_Cancellation(t *testing.T) {
	workDir := t.TempDir()
	tool := NewBashTool(workDir)
	args, err := json.Marshal(map[string]string{"command": "sleep 5"})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = tool.Execute(ctx, args)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
