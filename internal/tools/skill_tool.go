package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

// SkillTool 把技能包装成 tool：模型通过 tool_call 按需加载技能正文，
// 避免 v1 把全部技能正文常驻注入 system prompt 导致的 token 爆炸。
type SkillTool struct {
	name        string
	description string
	body        string
}

func NewSkillTool(name, description, body string) *SkillTool {
	return &SkillTool{name: name, description: description, body: body}
}

func (t *SkillTool) Name() string { return t.name }

func (t *SkillTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        t.name,
		Description: t.description,
		// 无参数技能：仅 description 即触发条件，与现有 bash/edit_file 等 tool 的 InputSchema 写法一致。
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

// maxSkillBodyLen 防止单个坏技能正文占满上下文。
const maxSkillBodyLen = 32 * 1024

func (t *SkillTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	body := t.body
	truncated := false
	if len(body) > maxSkillBodyLen {
		body = body[:maxSkillBodyLen]
		truncated = true
	}
	var b strings.Builder
	fmt.Fprintf(&b, "以下是你加载的技能 <%s> 的完整执行指南，必须严格遵循：\n\n", t.name)
	b.WriteString(body)
	if truncated {
		b.WriteString("\n\n[技能正文超出 32KB 已截断]")
	}
	return b.String(), nil
}
