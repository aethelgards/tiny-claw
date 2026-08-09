package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

// defaultMaxContentSize guards against writing pathologically large payloads.
const defaultMaxContentSize = 64 << 20 // 64 MiB

type WriteFileTool struct {
	workDir        string
	maxContentSize int64
}

// WriteFileOption configures a WriteFileTool at construction time.
type WriteFileOption func(*WriteFileTool)

// WithMaxContentSize overrides the maximum content size write_file accepts.
func WithMaxContentSize(size int64) WriteFileOption {
	return func(t *WriteFileTool) { t.maxContentSize = size }
}

func (w *WriteFileTool) Name() string {
	return "write_file"
}

func (w *WriteFileTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        w.Name(),
		Description: "将内容写入工作区内指定路径的文件，文件不存在时创建，父目录不存在时自动创建。请提供相对工作区的路径，路径不能包含 .. 或符号链接逃逸工作区。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "要写入的文件路径（相对工作区），如 cmd/claw/main.go",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "要写入的文件内容",
				},
			},
			"required": []string{
				"path",
				"content",
			},
		},
	}
}

type writeFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (w *WriteFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var writeFile writeFileArgs
	if err := json.Unmarshal(args, &writeFile); err != nil {
		return "", fmt.Errorf("write file failed, input json convert to writeFileArgs struct failed, fail info: %s", err.Error())
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if int64(len(writeFile.Content)) > w.maxContentSize {
		return "", fmt.Errorf("content size %d bytes exceeds limit %d bytes", len(writeFile.Content), w.maxContentSize)
	}

	// Resolve first so the lock is keyed by the concrete target path, then
	// take a per-path exclusive lock: writes to disjoint files proceed
	// concurrently, while writes to the same file are serialized.
	resolved, err := w.resolveWritePath(writeFile.Path)
	if err != nil {
		return "", err
	}
	release := toolLocks.acquirePath(resolved, true)
	defer release()

	if err := ctx.Err(); err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return "", fmt.Errorf("create parent directories failed, fail info: %s", err.Error())
	}
	if err := writeFileAtomic(ctx, resolved, []byte(writeFile.Content)); err != nil {
		return "", err
	}
	return fmt.Sprintf("成功写入文件: %s (%d 字节)", writeFile.Path, len(writeFile.Content)), nil
}

func (w *WriteFileTool) resolveWritePath(input string) (string, error) {
	return resolvePathForWrite(w.workDir, input)
}

// resolvePathForWrite validates that the target stays inside the workDir
// and returns the absolute path to write to. Unlike reading, the file itself
// may not exist yet, so only the parent directory is symlink-resolved; a
// symlink sitting at the final path (if any) must also resolve inside the
// workspace, or writing through it would escape.
func resolvePathForWrite(workDir, input string) (string, error) {
	target, resolvedWorkDir, err := resolveWorkspaceTarget(workDir, input)
	if err != nil {
		return "", err
	}

	// Symlink containment for the parent chain. The target's parent may not
	// exist yet (write_file creates directories), so resolve the deepest
	// existing ancestor and re-join the not-yet-created remainder below it.
	parent := filepath.Dir(target)
	resolvedParent := parent
	if rp, err := filepath.EvalSymlinks(parent); err == nil {
		resolvedParent = rp
	} else {
		ancestor := parent
		var rest []string
		for {
			rest = append([]string{filepath.Base(ancestor)}, rest...)
			ancestor = filepath.Dir(ancestor)
			if rp, err := filepath.EvalSymlinks(ancestor); err == nil {
				resolvedParent = filepath.Join(append([]string{rp}, rest...)...)
				break
			}
		}
	}
	if err := withinWorkspace(resolvedWorkDir, resolvedParent); err != nil {
		return "", fmt.Errorf("path %q resolves outside workspace through symlink: %w", input, err)
	}

	// A symlink at the final path redirects the write, so it must stay inside
	// the workspace too.
	if fi, err := os.Lstat(target); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		resolvedTarget, err := filepath.EvalSymlinks(target)
		if err != nil {
			return "", fmt.Errorf("resolve file failed: %w", err)
		}
		if err := withinWorkspace(resolvedWorkDir, resolvedTarget); err != nil {
			return "", fmt.Errorf("path %q resolves outside workspace through symlink: %w", input, err)
		}
	}

	return filepath.Join(resolvedParent, filepath.Base(target)), nil
}

func NewWriteFileTool(workDir string, opts ...WriteFileOption) BaseTool {
	t := &WriteFileTool{
		workDir:        workDir,
		maxContentSize: defaultMaxContentSize,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}
