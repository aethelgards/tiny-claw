package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aethelgards/tiny-claw/internal/provider"
	"github.com/aethelgards/tiny-claw/internal/schema"
)

// MemoryExtractor 从对话消息中提取结构化记忆。
// 返回提取出的记忆列表；LLM 调用失败或响应不可解析时返回错误，由调用方决定处置方式。
type MemoryExtractor interface {
	Extract(ctx context.Context, messages []schema.Message) ([]Memory, error)
}

// LLMExtractor 基于 LLM 的记忆提取器
type LLMExtractor struct {
	provider provider.LLMProvider
	timeout  time.Duration
}

func NewLLMExtractor(p provider.LLMProvider) *LLMExtractor {
	return &LLMExtractor{
		provider: p,
		timeout:  10 * time.Second,
	}
}

func (e *LLMExtractor) WithTimeout(d time.Duration) *LLMExtractor {
	e.timeout = d
	return e
}

const extractorSystemPrompt = `你是记忆提取器。从下面的对话消息中提取值得长期记住的信息，只提取四类：
- preferences: 用户级偏好（编码风格、语言偏好、常用命令、沟通习惯），必须跨项目适用，不要混入项目特有信息
- project: 项目知识（架构决策、技术栈、目录约定、构建/测试命令）
- errors: 错误模式（遇到的错误及解决方案，避免重复犯错）
- tools: 工具策略（哪些工具/参数组合在特定任务上有效）
要求：
- 每条记忆一句话，具体、可复用、不含临时状态（如"正在修改X"）
- 用 JSON 数组输出：[{"type":"project","content":"..."}]
- 没有值得记住的信息时输出 []`

// formatMessages 将对话消息压平为提取器可读的文本形式。
func formatMessages(messages []schema.Message) string {
	var sb strings.Builder
	for _, m := range messages {
		switch {
		case m.Role == schema.RoleAssistant && len(m.ToolCalls) > 0:
			for _, tc := range m.ToolCalls {
				sb.WriteString("[工具调用 ")
				sb.WriteString(tc.Name)
				sb.WriteString("(")
				sb.WriteString(string(tc.Arguments))
				sb.WriteString(")] ")
			}
			if m.Content != "" {
				sb.WriteString("[助手] ")
				sb.WriteString(m.Content)
				sb.WriteString("\n")
			}
		case m.Role == schema.RoleUser && m.ToolCallID != "":
			sb.WriteString("[工具结果 ")
			sb.WriteString(m.ToolCallID)
			sb.WriteString("] ")
			sb.WriteString(m.Content)
			sb.WriteString("\n")
		case m.Role == schema.RoleUser:
			sb.WriteString("[用户] ")
			sb.WriteString(m.Content)
			sb.WriteString("\n")
		default:
			sb.WriteString("[助手] ")
			sb.WriteString(m.Content)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// Extract 调用 LLM 提取记忆，校验类型合法性后返回待保存的记忆列表。
func (e *LLMExtractor) Extract(ctx context.Context, messages []schema.Message) ([]Memory, error) {
	if len(messages) == 0 {
		return nil, nil
	}

	promptMsgs := []schema.Message{
		{Role: schema.RoleSystem, Content: extractorSystemPrompt},
		{Role: schema.RoleUser, Content: "待提取的对话:\n" + formatMessages(messages)},
	}

	exCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	resp, err := e.provider.Generate(exCtx, promptMsgs, nil)
	if err != nil {
		return nil, fmt.Errorf("memory extractor generate: %w", err)
	}

	var items []struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(resp.Content), &items); err != nil {
		return nil, fmt.Errorf("memory extractor parse response %q: %w", resp.Content, err)
	}

	memories := make([]Memory, 0, len(items))
	for _, item := range items {
		mType, ok := ValidType(item.Type)
		if !ok || item.Content == "" {
			continue
		}
		memories = append(memories, Memory{
			Type:    mType,
			Content: item.Content,
			Source:  "auto",
		})
	}
	return memories, nil
}

// SessionHook 是压缩钩子：session.compress 在锁外以独立 goroutine 调用，
// 提取被丢弃消息中的记忆并落库。作为 goroutine 入口在此拦截 panic，
// 避免击穿整个进程。
type SessionHook struct {
	store     *MemoryStore
	extractor MemoryExtractor
	embedder  provider.Embedder
}

func NewSessionHook(store *MemoryStore, extractor MemoryExtractor, opts ...SessionHookOption) *SessionHook {
	h := &SessionHook{
		store:     store,
		extractor: extractor,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

type SessionHookOption func(*SessionHook)

func WithSessionEmbedder(e provider.Embedder) SessionHookOption {
	return func(h *SessionHook) {
		h.embedder = e
	}
}

func (h *SessionHook) Extract(dropped []schema.Message) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("memory session hook panicked", "panic", r)
		}
	}()

	if h.extractor == nil || h.store == nil || len(dropped) == 0 {
		return
	}

	memories, err := h.extractor.Extract(context.Background(), dropped)
	if err != nil {
		slog.Warn("memory extraction failed", "error", err)
		return
	}

	if h.embedder != nil && len(memories) > 0 {
		texts := make([]string, len(memories))
		for i, m := range memories {
			texts[i] = m.Content
		}
		vecs, err := h.embedder.Embed(context.Background(), texts)
		if err != nil {
			slog.Warn("batch embedding failed, falling back to keyword-only", "error", err)
		} else {
			for i := range memories {
				if i < len(vecs) && len(vecs[i]) > 0 {
					memories[i].Embedding = vecs[i]
				}
			}
		}
	}

	for _, m := range memories {
		if _, err := h.store.Save(m, scopeOfType(m.Type)); err != nil {
			slog.Warn("memory save failed", "type", m.Type, "error", err)
		}
	}
}
