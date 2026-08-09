package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

type EditFileTool struct {
	workDir string
}

func (e *EditFileTool) Name() string {
	return "edit_file"
}

func (e *EditFileTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        e.Name(),
		Description: "对现有文件进行局部的字符串替换。这比重写整个文件更安全、更快速。请提供足够的 old_text 上下文以确保匹配的唯一性。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "要修改的文件路径",
				},
				"old_text": map[string]any{
					"type":        "string",
					"description": "文件中原有的文本。必须包含足够的上下文（建议上下各多包含几行），以确保在文件中的唯一性。",
				},
				"new_text": map[string]any{
					"type":        "string",
					"description": "要替换成的新文本",
				},
			},
			"required": []string{
				"path",
				"old_text",
				"new_text",
			},
		},
	}
}

type editFileInput struct {
	Path    string `json:"path"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

func (e *EditFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var input editFileInput
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("edit file failed, input json convert to editFileInput struct failed, fail info: %s", err.Error())
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(input.OldText) == "" {
		return "", errors.New("old_text cannot be empty")
	}

	// Resolve the target once with the same workspace-containment rules as
	// write_file, so the read and the write hit the exact same file, then
	// take a per-path exclusive lock for the whole read-modify-write cycle:
	// two concurrent edits on the same file would otherwise both read the
	// original content and the second write would clobber the first.
	resolved, err := resolvePathForWrite(e.workDir, input.Path)
	if err != nil {
		return "", err
	}
	release := toolLocks.acquirePath(resolved, true)
	defer release()

	content, err := e.readFullContent(ctx, resolved, input.Path)
	if err != nil {
		return "", err
	}

	newContent, err := e.fuzzyReplace(content, input.OldText, input.NewText)
	if err != nil {
		return "", err
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := writeFileAtomic(ctx, resolved, []byte(newContent)); err != nil {
		return "", err
	}
	return fmt.Sprintf("成功修改文件: %s (%d 字节)", input.Path, len(newContent)), nil
}

// readFullContent reads the whole target file. Unlike read_file it returns
// the full content without truncation: a replacement must be computed against
// the entire file, not a head/tail excerpt.
func (e *EditFileTool) readFullContent(ctx context.Context, resolved, displayPath string) (string, error) {
	f, _, err := openValidatedFile(resolved, displayPath, defaultMaxFileSize)
	if err != nil {
		return "", err
	}
	// Closing the fd from a cancelled context unblocks a pending read.
	stop := context.AfterFunc(ctx, func() { _ = f.Close() })
	defer stop()
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("read file content failed, fail info: %s", err.Error())
	}
	return string(data), nil
}

func NewEditFileTool(workDir string) BaseTool {
	return &EditFileTool{
		workDir: workDir,
	}
}

// fuzzyReplace 实现了四级容错降级替换算法
func (e *EditFileTool) fuzzyReplace(originalContent, oldText, newText string) (string, error) {
	// L1: 精确匹配
	count := strings.Count(originalContent, oldText)
	if count == 1 {
		return strings.Replace(originalContent, oldText, newText, 1), nil
	}
	if count > 1 {
		return "", fmt.Errorf("old_text 匹配到了 %d 处，请提供更多的上下文代码以确保唯一性", count)
	}

	// L2: 换行符归一化 (统一将 \r\n 转换为 \n)
	normalizedContent := strings.ReplaceAll(originalContent, "\r\n", "\n")
	normalizedOld := strings.ReplaceAll(oldText, "\r\n", "\n")

	count = strings.Count(normalizedContent, normalizedOld)
	if count == 1 {
		return strings.Replace(normalizedContent, normalizedOld, newText, 1), nil
	}

	// L3: Trim Space 匹配 (忽略首尾的空行和空格)
	trimmedOld := strings.TrimSpace(normalizedOld)
	if trimmedOld != "" {
		count = strings.Count(normalizedContent, trimmedOld)
		if count == 1 {
			// 注意：这里替换时，我们只能替换被 Trim 后的部分，不能直接用 newText 破坏原本的缩进
			// 为了保持本专栏代码不过于冗长复杂，当触发 L3/L4 时，如果 newText 没有带有正确的缩进，
			// 可能会导致替换后代码格式不美观。但这总比直接报错让 Agent 死循环要好。
			return strings.Replace(normalizedContent, trimmedOld, newText, 1), nil
		}
	}

	// L4: 逐行去缩进匹配 (最强力的容错：消除大模型遗漏缩进的幻觉)
	return lineByLineReplace(normalizedContent, normalizedOld, newText)
}

// lineByLineReplace 将文本按行切割，去除首尾空白后进行滑动窗口匹配
func lineByLineReplace(content, oldText, newText string) (string, error) {
	contentLines := strings.Split(content, "\n")
	oldLines := strings.Split(strings.TrimSpace(oldText), "\n")

	if len(oldLines) == 0 || len(contentLines) < len(oldLines) {
		return "", fmt.Errorf("找不到该代码片段")
	}

	// 清理 oldLines 的每行首尾空白
	for i := range oldLines {
		oldLines[i] = strings.TrimSpace(oldLines[i])
	}

	matchCount := 0
	matchStartIndex := -1
	matchEndIndex := -1

	// 滑动窗口在原始文件中寻找匹配块
	for i := 0; i <= len(contentLines)-len(oldLines); i++ {
		isMatch := true
		for j := 0; j < len(oldLines); j++ {
			if strings.TrimSpace(contentLines[i+j]) != oldLines[j] {
				isMatch = false
				break
			}
		}

		if isMatch {
			matchCount++
			matchStartIndex = i
			matchEndIndex = i + len(oldLines)
		}
	}

	if matchCount == 0 {
		return "", fmt.Errorf("在文件中未找到 old_text，请大模型先调用 read_file 仔细确认文件内容和缩进")
	}
	if matchCount > 1 {
		return "", fmt.Errorf("模糊匹配到了 %d 处相似代码，请提供更多上下行代码以精确定位", matchCount)
	}

	// 执行替换：将匹配到的原始行范围替换为 newText 拆分后的行
	// (这里简单处理，将 newText 直接作为整体替换进去)
	var newContentLines []string
	newContentLines = append(newContentLines, contentLines[:matchStartIndex]...)
	newContentLines = append(newContentLines, newText) // 插入新内容
	newContentLines = append(newContentLines, contentLines[matchEndIndex:]...)

	return strings.Join(newContentLines, "\n"), nil
}
