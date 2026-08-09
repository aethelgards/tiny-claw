package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/aethelgards/tiny-claw/internal/provider"
	"github.com/aethelgards/tiny-claw/internal/schema"
)

// summarizeSystemPrompt 摘要任务的 system 提示词。
const summarizeSystemPrompt = "你是对话历史摘要器。把提供的对话消息压缩成简洁摘要：保留用户目标、关键事实、已完成的工具操作与结果、未完成事项。用中文输出，300 字以内，直接给摘要正文，不要任何前缀或客套。"

// NewLLMSummarizer 构造基于 LLM 的会话摘要函数，供 WithSummarizer 注入。
// 将旧摘要与即将被丢弃的原始消息整理为摘要提示词，调用 provider.Generate
// 生成新摘要；失败时错误上抛，由 Session.compress 回退为纯截断。
func NewLLMSummarizer(p provider.LLMProvider) Summarizer {
	return func(ctx context.Context, existingSummary string, msgs []schema.Message) (string, error) {
		promptMsgs := []schema.Message{
			{Role: schema.RoleSystem, Content: summarizeSystemPrompt},
		}
		if existingSummary != "" {
			promptMsgs = append(promptMsgs, schema.Message{
				Role:    schema.RoleUser,
				Content: "已有摘要:\n" + existingSummary,
			})
		}
		promptMsgs = append(promptMsgs, schema.Message{
			Role:    schema.RoleUser,
			Content: "待压缩的新增对话:\n" + renderMsgs(msgs),
		})

		resp, err := p.Generate(ctx, promptMsgs, nil)
		if err != nil {
			return "", err
		}
		return resp.Content, nil
	}
}

// renderMsgs 把原始消息渲染为摘要提示词的紧凑文本（带角色标记）。
func renderMsgs(msgs []schema.Message) string {
	var sb strings.Builder
	for _, m := range msgs {
		switch {
		case m.Role == schema.RoleAssistant && len(m.ToolCalls) > 0:
			for _, tc := range m.ToolCalls {
				fmt.Fprintf(&sb, "[工具调用 %s(%s)] %s\n", tc.Name, tc.ID, string(tc.Arguments))
			}
			if m.Content != "" {
				fmt.Fprintf(&sb, "[助手] %s\n", m.Content)
			}
		case m.Role == schema.RoleUser && m.ToolCallID != "":
			fmt.Fprintf(&sb, "[工具结果 %s] %s\n", m.ToolCallID, m.Content)
		case m.Role == schema.RoleUser:
			fmt.Fprintf(&sb, "[用户] %s\n", m.Content)
		default:
			fmt.Fprintf(&sb, "[助手] %s\n", m.Content)
		}
	}
	return sb.String()
}
