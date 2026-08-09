package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"
)

// executeReadFile runs the read_file tool against the given workDir and path
// through its public API, mirroring how the registry invokes tools.
func executeReadFile(t *testing.T, workDir, path string) (string, error) {
	t.Helper()
	return executeReadFileCtx(t, context.Background(), workDir, path)
}

func executeReadFileCtx(t *testing.T, ctx context.Context, workDir, path string) (string, error) {
	t.Helper()
	tool := NewReadFileTool(workDir)
	args, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return tool.Execute(ctx, args)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func TestReadFileTool_ReadsFileContent(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "note.txt"), "hello world")

	got, err := executeReadFile(t, workDir, "note.txt")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestReadFileTool_ReadsFileInNestedDirectory(t *testing.T) {
	workDir := t.TempDir()
	dir := filepath.Join(workDir, "a", "b")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(dir, "deep.txt"), "deep content")

	got, err := executeReadFile(t, workDir, filepath.Join("a", "b", "deep.txt"))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got != "deep content" {
		t.Errorf("got %q, want %q", got, "deep content")
	}
}

func TestReadFileTool_MissingFileReturnsError(t *testing.T) {
	_, err := executeReadFile(t, t.TempDir(), "does-not-exist.txt")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestReadFileTool_InvalidArgsReturnsError(t *testing.T) {
	tool := NewReadFileTool(t.TempDir())
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path": 123}`))
	if err == nil {
		t.Fatal("expected error for non-string path arg, got nil")
	}
}

func TestReadFileTool_RejectsEmptyPath(t *testing.T) {
	_, err := executeReadFile(t, t.TempDir(), "")
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

func TestReadFileTool_RejectsAbsolutePath(t *testing.T) {
	workDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, outside, "secret")

	_, err := executeReadFile(t, workDir, outside)
	if err == nil {
		t.Fatal("expected error for absolute path, got nil")
	}
}

func TestReadFileTool_RejectsPathTraversal(t *testing.T) {
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

	_, err = executeReadFile(t, workDir, relOutside)
	if err == nil {
		t.Fatalf("expected error for traversal path %q, got nil", relOutside)
	}
}

func TestReadFileTool_RejectsSymlinkEscape(t *testing.T) {
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

	_, err := executeReadFile(t, workDir, "evil-link.txt")
	if err == nil {
		t.Fatal("expected error for symlink escaping workDir, got nil")
	}
}

func TestReadFileTool_AllowsSymlinkWithinWorkDir(t *testing.T) {
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

	got, err := executeReadFile(t, workDir, "link.txt")
	if err != nil {
		t.Fatalf("Execute returned error for in-workdir symlink: %v", err)
	}
	if got != "real content" {
		t.Errorf("got %q, want %q", got, "real content")
	}
}

func TestReadFileTool_ShortContentNotTruncated(t *testing.T) {
	orig := maxLen
	t.Cleanup(func() { maxLen = orig })

	workDir := t.TempDir()
	content := strings.Repeat("a", maxLen)
	writeFile(t, filepath.Join(workDir, "full.txt"), content)

	got, err := executeReadFile(t, workDir, "full.txt")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got != content {
		t.Errorf("content at maxLen boundary changed: got %d bytes, want %d", len(got), len(content))
	}
}

func TestReadFileTool_LongContentShowsHeadAndTail(t *testing.T) {
	orig := maxLen
	maxLen = 256
	t.Cleanup(func() { maxLen = orig })

	workDir := t.TempDir()
	content := strings.Repeat("H", 1000) + "MIDDLE" + strings.Repeat("T", 1000)
	writeFile(t, filepath.Join(workDir, "big.txt"), content)

	got, err := executeReadFile(t, workDir, "big.txt")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.HasPrefix(got, "H") {
		t.Error("truncated output should start with the file head")
	}
	if !strings.HasSuffix(got, "T") {
		t.Error("truncated output should end with the file tail")
	}
	if strings.Contains(got, "MIDDLE") {
		t.Error("truncated output should elide the middle of the file")
	}
	if len(got) > maxLen {
		t.Errorf("truncated output is %d bytes, exceeds maxLen %d", len(got), maxLen)
	}
	if !utf8.ValidString(got) {
		t.Error("truncated output is not valid UTF-8")
	}
}

func TestReadFileTool_TruncationKeepsUTF8Valid(t *testing.T) {
	orig := maxLen
	maxLen = 64
	t.Cleanup(func() { maxLen = orig })

	workDir := t.TempDir()
	content := strings.Repeat("汉", 100) // 300 bytes, multi-byte runes
	writeFile(t, filepath.Join(workDir, "cjk.txt"), content)

	got, err := executeReadFile(t, workDir, "cjk.txt")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !utf8.ValidString(got) {
		t.Errorf("truncated output is not valid UTF-8: %q", got)
	}
	if len(got) > maxLen {
		t.Errorf("truncated output is %d bytes, exceeds maxLen %d", len(got), maxLen)
	}
}

func TestReadFileTool_TruncationMarkerReportsSize(t *testing.T) {
	orig := maxLen
	maxLen = 128
	t.Cleanup(func() { maxLen = orig })

	workDir := t.TempDir()
	content := strings.Repeat("x", 1000)
	writeFile(t, filepath.Join(workDir, "big.txt"), content)

	got, err := executeReadFile(t, workDir, "big.txt")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(got, "1000") {
		t.Errorf("truncation marker should report total file size, got: %q", got)
	}
}

func TestReadFileTool_ExceedsMaxFileSize(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "huge.txt"), strings.Repeat("a", 200))

	tool := NewReadFileTool(workDir, WithMaxFileSize(100))
	args, err := json.Marshal(map[string]string{"path": "huge.txt"})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	_, err = tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error when file exceeds maxFileSize, got nil")
	}
	if !strings.Contains(err.Error(), "200") || !strings.Contains(err.Error(), "100") {
		t.Errorf("error should report actual size and limit, got: %v", err)
	}
}

func TestReadFileTool_EmptyFileReturnsEmpty(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "empty.txt"), "")

	got, err := executeReadFile(t, workDir, "empty.txt")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestReadFileTool_StripsUTF8BOM(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "bom.txt"), "\xEF\xBB\xBFhello world")

	got, err := executeReadFile(t, workDir, "bom.txt")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got != "hello world" {
		t.Errorf("BOM should be stripped, got %q", got)
	}
}

func TestReadFileTool_BinaryFileReturnsSummary(t *testing.T) {
	workDir := t.TempDir()
	bin := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01, 0x02, 0x03}
	if err := os.WriteFile(filepath.Join(workDir, "img.bin"), bin, 0o644); err != nil {
		t.Fatalf("write binary fixture: %v", err)
	}

	got, err := executeReadFile(t, workDir, "img.bin")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(got, "二进制") {
		t.Errorf("output should identify binary file, got %q", got)
	}
	if strings.ContainsRune(got, '\x00') {
		t.Errorf("binary NUL bytes leaked into output: %q", got)
	}
	if !strings.Contains(got, "image") {
		t.Errorf("output should include detected MIME type (PNG), got %q", got)
	}
}

func TestReadFileTool_LineBoundaryTruncation(t *testing.T) {
	orig := maxLen
	maxLen = 128
	t.Cleanup(func() { maxLen = orig })

	workDir := t.TempDir()
	// Each line is exactly "L01\n" (4 bytes); head cut must land on a newline,
	// tail cut must start at a line beginning.
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("L01\n")
	}
	content := sb.String()
	writeFile(t, filepath.Join(workDir, "lines.txt"), content)

	got, err := executeReadFile(t, workDir, "lines.txt")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	marker := fmt.Sprintf("\n\n...[内容过长，已截断：共 %d 字节，以下展示文件开头与末尾]...\n\n", len(content))
	parts := strings.SplitN(got, marker, 2)
	if len(parts) != 2 {
		t.Fatalf("truncation marker not found in output: %q", got)
	}
	head, tail := parts[0], parts[1]
	if !strings.HasSuffix(head, "\n") {
		t.Errorf("head should end at a line boundary, got %q", head)
	}
	if !strings.HasPrefix(tail, "L01\n") {
		t.Errorf("tail should start at a line beginning, got %q", tail)
	}
}

// recordingReaderAt records every ReadAt call so tests can assert that only
// the head and tail regions are read (bounded memory).
type recordingReaderAt struct {
	data  []byte
	reads []struct {
		off int64
		n   int64
	}
}

func (r *recordingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	r.reads = append(r.reads, struct {
		off int64
		n   int64
	}{off: off, n: int64(len(p))})
	if off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n := copy(p, r.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func TestReadFileContent_BoundedReads(t *testing.T) {
	orig := maxLen
	maxLen = 256
	t.Cleanup(func() { maxLen = orig })

	rec := &recordingReaderAt{data: bytes.Repeat([]byte{'a'}, 10000)}
	got, err := readFileContent(context.Background(), rec, 10000)
	if err != nil {
		t.Fatalf("readFileContent returned error: %v", err)
	}
	if len(rec.reads) != 2 {
		t.Fatalf("expected exactly 2 reads (head+tail), got %d: %+v", len(rec.reads), rec.reads)
	}
	if rec.reads[0].off != 0 {
		t.Errorf("head read should start at offset 0, got %d", rec.reads[0].off)
	}
	if rec.reads[1].off != 10000-rec.reads[1].n {
		t.Errorf("tail read should end at file end, got off=%d n=%d", rec.reads[1].off, rec.reads[1].n)
	}
	totalRead := rec.reads[0].n + rec.reads[1].n
	if totalRead > int64(maxLen) {
		t.Errorf("read %d bytes total, exceeds maxLen %d", totalRead, maxLen)
	}
	if !utf8.ValidString(got) {
		t.Error("bounded read output is not valid UTF-8")
	}
}

func TestReadFileContent_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rec := &recordingReaderAt{data: bytes.Repeat([]byte{'a'}, 100)}
	_, err := readFileContent(ctx, rec, 100)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestReadFileTool_Cancellation(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "f.txt"), strings.Repeat("a", 100))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := executeReadFileCtx(t, ctx, workDir, "f.txt")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
