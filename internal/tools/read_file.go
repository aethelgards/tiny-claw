package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"unicode/utf8"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

// maxLen bounds the number of bytes a single read_file result may contain.
// Kept as a var so tests can shrink it without writing multi-KB fixtures.
var maxLen = 8192

type ReadFileTool struct {
	workDir     string
	maxFileSize int64
}

// ReadFileOption configures a ReadFileTool at construction time.
type ReadFileOption func(*ReadFileTool)

// WithMaxFileSize overrides the maximum file size read_file will open.
func WithMaxFileSize(size int64) ReadFileOption {
	return func(t *ReadFileTool) { t.maxFileSize = size }
}

func (r *ReadFileTool) Name() string {
	return "read_file"
}

func (r *ReadFileTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        r.Name(),
		Description: "读取指定路径的文件内容。请提供相对工作区的路径，路径不能包含 .. 或符号链接逃逸工作区。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "要读取的文件路径（相对工作区），如 cmd/claw/main.go",
				},
			},
			"required": []string{
				"path",
			},
		},
	}
}

type readFileArgs struct {
	Path string `json:"path"`
}

func (r *ReadFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var readFile readFileArgs
	if err := json.Unmarshal(args, &readFile); err != nil {
		return "", fmt.Errorf("read file failed, input json convert to readfileArgs struct failed, fail info: %s", err.Error())
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Resolve first so the lock is keyed by the concrete target path, then
	// take a shared per-path lock: concurrent reads of the same file are
	// allowed, writers of that file are excluded, and reads of disjoint
	// files proceed fully in parallel.
	resolved, err := r.resolvePath(readFile.Path)
	if err != nil {
		return "", err
	}
	release := toolLocks.acquirePath(resolved, false)
	defer release()
	return r.execute(ctx, readFile.Path, resolved)
}

func (r *ReadFileTool) execute(ctx context.Context, displayPath, resolved string) (string, error) {
	f, size, err := openValidatedFile(resolved, displayPath, r.maxFileSize)
	if err != nil {
		return "", err
	}
	// Closing the fd from a cancelled context unblocks a pending read.
	stop := context.AfterFunc(ctx, func() { _ = f.Close() })
	defer stop()
	defer f.Close()
	return readFileContent(ctx, f, size)
}

// resolvePath validates that input stays inside the tool's workDir and returns
// the absolute, symlink-resolved path to read. It rejects empty or absolute
// inputs, lexical traversal (..), and symlinks that escape the workspace.
func (r *ReadFileTool) resolvePath(input string) (string, error) {
	target, resolvedWorkDir, err := resolveWorkspaceTarget(r.workDir, input)
	if err != nil {
		return "", err
	}

	// Symlink containment: the fully resolved target must also stay inside
	// the workspace.
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", fmt.Errorf("resolve file failed: %w", err)
	}
	if err := withinWorkspace(resolvedWorkDir, resolved); err != nil {
		return "", fmt.Errorf("path %q resolves outside workspace through symlink: %w", input, err)
	}
	return resolved, nil
}

// readFileContent reads at most maxLen bytes from ra in total: the whole file
// when it fits, otherwise only the head and tail regions around an elision
// marker. Binary files return a metadata summary instead of raw bytes.
// Memory usage is O(maxLen), independent of the file size.
func readFileContent(ctx context.Context, ra io.ReaderAt, size int64) (string, error) {
	if size <= 0 {
		return "", nil
	}

	if size <= int64(maxLen) {
		buf := make([]byte, size)
		if err := readAt(ctx, ra, buf, 0); err != nil {
			return "", err
		}
		buf = stripBOM(buf)
		if isBinary(buf) {
			return binarySummary(size, buf), nil
		}
		return string(buf), nil
	}

	marker := fmt.Sprintf("\n\n...[内容过长，已截断：共 %d 字节，以下展示文件开头与末尾]...\n\n", size)
	budget := maxLen - len(marker)
	if budget <= 0 {
		// The marker alone does not fit; fall back to a raw, rune-safe head.
		buf := make([]byte, maxLen)
		if err := readAt(ctx, ra, buf, 0); err != nil {
			return "", err
		}
		head := stripBOM(buf[:runeSafeCut(buf, len(buf))])
		if isBinary(head) {
			return binarySummary(size, head), nil
		}
		return string(head), nil
	}

	headBytes := budget * 2 / 3
	head := make([]byte, headBytes)
	if err := readAt(ctx, ra, head, 0); err != nil {
		return "", err
	}
	headEnd := cutToLineEnd(head, len(head))
	head = stripBOM(head[:headEnd])
	if isBinary(head) {
		return binarySummary(size, head), nil
	}

	tailBytes := budget - headBytes
	tail := make([]byte, tailBytes)
	if err := readAt(ctx, ra, tail, size-int64(tailBytes)); err != nil {
		return "", err
	}
	tailStart := cutToLineStart(tail, 0)

	return string(head) + marker + string(tail[tailStart:]), nil
}

// readAt reads len(p) bytes at offset off, honoring ctx cancellation.
func readAt(ctx context.Context, ra io.ReaderAt, p []byte, off int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := ra.ReadAt(p, off)
	if err != nil && err != io.EOF {
		return fmt.Errorf("read file content failed, fail info: %s", err.Error())
	}
	if n < len(p) {
		return io.ErrUnexpectedEOF
	}
	return ctx.Err()
}

// cutToLineEnd returns the largest cut <= max that ends at a line boundary,
// falling back to a rune boundary when the slice has no newline.
func cutToLineEnd(content []byte, max int) int {
	cut := runeSafeCut(content, max)
	if i := bytes.LastIndexByte(content[:cut], '\n'); i >= 0 {
		return i + 1
	}
	return cut
}

// cutToLineStart returns the smallest index >= min that starts at a line
// boundary, falling back to a rune boundary when the slice has no newline.
func cutToLineStart(content []byte, min int) int {
	i := min
	for i < len(content) && !utf8.RuneStart(content[i]) {
		i++
	}
	if j := bytes.IndexByte(content[i:], '\n'); j >= 0 {
		return i + j + 1
	}
	return i
}

// runeSafeCut returns the largest cut <= max such that content[:cut] is valid
// UTF-8, i.e. the cut never splits a multi-byte sequence. It also handles
// buffers that themselves end mid-rune (e.g. a partial read).
func runeSafeCut(content []byte, max int) int {
	if max > len(content) {
		max = len(content)
	}
	for max > 0 && !utf8.Valid(content[:max]) {
		max--
	}
	return max
}

// stripBOM removes a leading UTF-8 byte order mark.
func stripBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

// isBinary reports whether sample looks like binary data: it contains NUL
// bytes or is not valid UTF-8.
func isBinary(sample []byte) bool {
	for _, b := range sample {
		if b == 0 {
			return true
		}
	}
	return !utf8.Valid(sample)
}

// binarySummary returns a compact metadata description of a binary file so
// its raw bytes never enter the model context.
func binarySummary(size int64, sample []byte) string {
	contentType := http.DetectContentType(sample)
	const previewLen = 64
	if len(sample) > previewLen {
		sample = sample[:previewLen]
	}
	return fmt.Sprintf("[二进制文件] 大小: %d 字节, 类型: %s, 前 %d 字节: %x", size, contentType, len(sample), sample)
}

func NewReadFileTool(workDir string, opts ...ReadFileOption) BaseTool {
	t := &ReadFileTool{
		workDir:     workDir,
		maxFileSize: defaultMaxFileSize,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}
