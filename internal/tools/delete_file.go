package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

type DeleteFileTool struct {
	workDir string
}

func (d *DeleteFileTool) Name() string {
	return "delete_file"
}

func (d *DeleteFileTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        d.Name(),
		Description: "删除工作区内指定路径的文件。请提供相对工作区的路径，路径不能包含 .. 或符号链接逃逸工作区。仅删除文件，目录不会被删除。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "要删除的文件路径（相对工作区），如 cmd/claw/main.go",
				},
			},
			"required": []string{
				"path",
			},
		},
	}
}

type deleteFileArgs struct {
	Path string `json:"path"`
}

func (d *DeleteFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var input deleteFileArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("delete file failed, input json convert to deleteFileArgs struct failed, fail info: %s", err.Error())
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// Resolve with the same workspace-containment rules as write_file and
	// edit_file. A symlink at the target must stay inside the workspace; when
	// it does, deletion removes the link itself, never its target.
	resolved, err := resolvePathForWrite(d.workDir, input.Path)
	if err != nil {
		return "", err
	}
	// Take a per-path exclusive lock so a concurrent read/write/edit of the
	// same file cannot observe a half-deleted state, while deletions of
	// disjoint paths proceed in parallel.
	release := toolLocks.acquirePath(resolved, true)
	defer release()

	if err := ctx.Err(); err != nil {
		return "", err
	}

	// Lstat (not Stat) so a symlink is recognized as a link: only regular
	// files and links are removed, directories are rejected.
	info, err := os.Lstat(resolved)
	if err != nil {
		return "", fmt.Errorf("delete file failed, fail info: %s", err.Error())
	}
	if info.IsDir() {
		return "", fmt.Errorf("path %q is a directory, delete_file only removes files", input.Path)
	}

	if err := os.Remove(resolved); err != nil {
		return "", fmt.Errorf("delete file failed, fail info: %s", err.Error())
	}
	return fmt.Sprintf("成功删除文件: %s", input.Path), nil
}

func NewDeleteFileTool(workDir string) BaseTool {
	return &DeleteFileTool{workDir: workDir}
}
