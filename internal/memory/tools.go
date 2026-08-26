package memory

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

// --- SaveMemoryTool ---

type SaveMemoryTool struct {
	store *MemoryStore
}

func NewSaveMemoryTool(store *MemoryStore) *SaveMemoryTool {
	return &SaveMemoryTool{store: store}
}

func (t *SaveMemoryTool) Name() string {
	return "save_memory"
}

func (t *SaveMemoryTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        t.Name(),
		Description: "显式保存一条长期记忆。type 缺省自动推断（兜底 project），scope 缺省按类型路由。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{
					"type":        "string",
					"description": "记忆正文（一句话/一段话）",
				},
				"type": map[string]any{
					"type":        "string",
					"description": "记忆类型：preferences/project/errors/tools，缺省自动推断",
					"enum":        []string{"preferences", "project", "errors", "tools"},
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "存储作用域：global/project，缺省按类型路由",
					"enum":        []string{"global", "project"},
				},
			},
			"required": []string{"content"},
		},
	}
}

type saveMemoryArgs struct {
	Content string `json:"content"`
	Type    string `json:"type,omitempty"`
	Scope   string `json:"scope,omitempty"`
}

func (t *SaveMemoryTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a saveMemoryArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Content == "" {
		return "", errors.New("content is required")
	}

	mType := MemoryType(a.Type)
	if mType == "" {
		mType = inferType(a.Content)
	}
	if _, ok := ValidType(string(mType)); !ok {
		mType = TypeProject // fallback
	}

	scope := Scope(a.Scope)
	if scope == "" {
		scope = scopeOfType(mType)
	}

	m := Memory{
		Type:    mType,
		Content: a.Content,
		Source:  "explicit",
	}
	id, err := t.store.Save(m, scope)
	if err != nil {
		return "", err
	}
	return "saved: " + id, nil
}

func inferType(content string) MemoryType {
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "报错") || strings.Contains(lower, "失败") || strings.Contains(lower, "解决"):
		return TypeError
	case strings.Contains(lower, "项目") || strings.Contains(lower, "架构") || strings.Contains(lower, "依赖"):
		return TypeProject
	case strings.Contains(lower, "习惯") || strings.Contains(lower, "偏好") || strings.Contains(lower, "喜欢"):
		return TypePreference
	default:
		return TypeProject
	}
}

// --- RecallMemoryTool ---

type RecallMemoryTool struct {
	store *MemoryStore
}

func NewRecallMemoryTool(store *MemoryStore) *RecallMemoryTool {
	return &RecallMemoryTool{store: store}
}

func (t *RecallMemoryTool) Name() string {
	return "recall_memory"
}

func (t *RecallMemoryTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        t.Name(),
		Description: "按关键词检索长期记忆。scope 缺省先查项目再查全局。命中条目会增加访问计数。若未命中，可换同义关键词重试，或传空 query 回退最近活跃记忆。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "关键词查询，空则返回最近活跃",
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "搜索作用域：global/project/空(先项目再全局)",
					"enum":        []string{"global", "project"},
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "返回条数上限",
				},
			},
			"required": []string{},
		},
	}
}

type recallMemoryArgs struct {
	Query string `json:"query,omitempty"`
	Scope string `json:"scope,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

func (t *RecallMemoryTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a recallMemoryArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Limit <= 0 {
		a.Limit = 10
	}

	scope := Scope(a.Scope)
	results := t.store.Recall(a.Query, scope, a.Limit)

	// Touch each hit for access counting
	for _, r := range results {
		t.store.Touch(r.ID, scope)
	}

	// Format output
	if len(results) == 0 {
		return "no memories found", nil
	}

	var out strings.Builder
	for i, r := range results {
		out.WriteString("[")
		out.WriteString(string(r.Type))
		out.WriteString("] ")
		out.WriteString(r.Content)
		if i < len(results)-1 {
			out.WriteString("\n")
		}
	}
	return out.String(), nil
}

// --- ForgetMemoryTool ---

type ForgetMemoryTool struct {
	store *MemoryStore
}

func NewForgetMemoryTool(store *MemoryStore) *ForgetMemoryTool {
	return &ForgetMemoryTool{store: store}
}

func (t *ForgetMemoryTool) Name() string {
	return "forget_memory"
}

func (t *ForgetMemoryTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        t.Name(),
		Description: "按 ID 删除一条记忆。scope 缺省先查项目再查全局。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "记忆 ID（从 recall_memory 获取）",
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "作用域：global/project/空(先项目再全局)",
					"enum":        []string{"global", "project"},
				},
			},
			"required": []string{"id"},
		},
	}
}

type forgetMemoryArgs struct {
	ID    string `json:"id"`
	Scope string `json:"scope,omitempty"`
}

func (t *ForgetMemoryTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a forgetMemoryArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.ID == "" {
		return "", errors.New("id is required")
	}

	scope := Scope(a.Scope)
	if err := t.store.Forget(a.ID, scope); err != nil {
		return "", err
	}
	return "forgotten: " + a.ID, nil
}