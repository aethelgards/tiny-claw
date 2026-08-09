package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

// recordingProvider 记录收到的提示词并返回预设响应。
type recordingProvider struct {
	promptMsgs []schema.Message
	response   string
	err        error
}

func (r *recordingProvider) Generate(_ context.Context, msgs []schema.Message, _ []schema.ToolDefinition) (*schema.Message, error) {
	r.promptMsgs = msgs
	if r.err != nil {
		return nil, r.err
	}
	return &schema.Message{Role: schema.RoleAssistant, Content: r.response}, nil
}

func TestLLMSummarizerBuildsPrompt(t *testing.T) {
	rec := &recordingProvider{response: "压缩后的摘要"}
	sum := NewLLMSummarizer(rec)

	msgs := []schema.Message{
		{Role: schema.RoleUser, Content: "你好"},
		{Role: schema.RoleAssistant, Content: "世界"},
		{
			Role:    schema.RoleAssistant,
			Content: "调用工具",
			ToolCalls: []schema.ToolCall{{
				ID: "tc-1", Name: "read_file", Arguments: []byte(`{"path":"a.txt"}`),
			}},
		},
		{Role: schema.RoleUser, Content: "文件内容", ToolCallID: "tc-1"},
	}

	got, err := sum(context.Background(), "旧摘要", msgs)
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if got != "压缩后的摘要" {
		t.Fatalf("summary = %q, want provider response", got)
	}

	// 提示词结构：[system, user(已有摘要), user(新增对话)]
	if len(rec.promptMsgs) != 3 {
		t.Fatalf("want 3 prompt messages, got %d: %+v", len(rec.promptMsgs), rec.promptMsgs)
	}
	if rec.promptMsgs[0].Role != schema.RoleSystem {
		t.Fatalf("prompt[0] should be system, got %+v", rec.promptMsgs[0])
	}
	if !strings.Contains(rec.promptMsgs[1].Content, "旧摘要") {
		t.Fatalf("prompt[1] should carry existing summary, got %q", rec.promptMsgs[1].Content)
	}
	body := rec.promptMsgs[2].Content
	for _, want := range []string{"[用户]", "你好", "[工具调用", "read_file", "[工具结果", "tc-1", "[助手]", "世界"} {
		if !strings.Contains(body, want) {
			t.Fatalf("prompt[2] should contain %q, got:\n%s", want, body)
		}
	}
}

func TestLLMSummarizerNoExistingSummary(t *testing.T) {
	rec := &recordingProvider{response: "sum"}
	sum := NewLLMSummarizer(rec)

	if _, err := sum(context.Background(), "", nil); err != nil {
		t.Fatalf("summarize: %v", err)
	}
	// 无旧摘要时跳过"已有摘要"消息
	if len(rec.promptMsgs) != 2 {
		t.Fatalf("want 2 prompt messages without existing summary, got %d", len(rec.promptMsgs))
	}
}

func TestLLMSummarizerErrorPropagates(t *testing.T) {
	rec := &recordingProvider{err: errors.New("api boom")}
	sum := NewLLMSummarizer(rec)

	if _, err := sum(context.Background(), "", nil); err == nil {
		t.Fatal("summarizer should propagate provider error")
	}
}
