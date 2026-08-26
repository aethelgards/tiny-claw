package engine

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aethelgards/tiny-claw/internal/config"
	ctxpkg "github.com/aethelgards/tiny-claw/internal/context"
	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/aethelgards/tiny-claw/internal/tools"
)

// ---- 测试替身 ----

// fakeProvider 按序弹出预设响应；耗尽后返回空 assistant 消息。
// gotHistory 记录每次 Generate 收到的完整上下文，用于断言多轮记忆。
type fakeProvider struct {
	responses  []*schema.Message
	gotHistory [][]schema.Message
	err        error // 非 nil 时 Generate 恒失败
}

func (f *fakeProvider) Generate(_ context.Context, msgs []schema.Message, _ []schema.ToolDefinition) (*schema.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.gotHistory = append(f.gotHistory, msgs)
	if len(f.responses) == 0 {
		return &schema.Message{Role: schema.RoleAssistant, Content: "done"}, nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

type fakeTool struct{}

func (fakeTool) Name() string { return "fake_tool" }

func (fakeTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{Name: "fake_tool", Description: "fake tool for tests", InputSchema: map[string]any{}}
}

func (fakeTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return "fake result", nil
}

// ---- 测试辅助 ----

func newTestAgent(t *testing.T, p *fakeProvider, reg tools.Registry, workDir string) *AgentEngine {
	t.Helper()
	composer := ctxpkg.NewPromptComposer(workDir, false)
	return NewAgentEngine(p, reg, config.Settings{WorkDir: workDir}, NewNopReporter(), composer, ctxpkg.NewRecoveryManager(), NewReminderInjector(3))
}

// ---- 无会话（回归） ----

func TestRunWithoutSessionStateless(t *testing.T) {
	workDir := t.TempDir()
	p := &fakeProvider{responses: []*schema.Message{{Role: schema.RoleAssistant, Content: "ok"}}}
	agent := newTestAgent(t, p, tools.NewToolRegistry(), workDir)

	if err := agent.Run(context.Background(), "hello"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".claw", "sessions")); !os.IsNotExist(err) {
		t.Fatalf("without WithSession no sessions dir expected, err=%v", err)
	}
}

// ---- 单轮落盘 ----

func TestRunPersistsTurnToSession(t *testing.T) {
	workDir := t.TempDir()
	p := &fakeProvider{responses: []*schema.Message{{Role: schema.RoleAssistant, Content: "hi there"}}}
	reg := tools.NewToolRegistry()
	sess := NewSessionMessage().GetOrCreate("chat-1", workDir)
	agent := newTestAgent(t, p, reg, workDir).WithSession(sess)

	if err := agent.Run(context.Background(), "first msg"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mem := sess.GetWorkingMemory(context.Background())
	if len(mem) != 2 {
		t.Fatalf("want 2 messages in session, got %d: %+v", len(mem), mem)
	}
	if mem[0].Role != schema.RoleUser || mem[0].Content != "first msg" {
		t.Fatalf("mem[0] = %+v, want user 'first msg'", mem[0])
	}
	if mem[1].Role != schema.RoleAssistant || mem[1].Content != "hi there" {
		t.Fatalf("mem[1] = %+v, want assistant 'hi there'", mem[1])
	}

	// 磁盘 round-trip：新管理器（模拟重启）恢复同一轮
	restored := NewSessionMessage().GetOrCreate("chat-1", workDir)
	if m := restored.GetWorkingMemory(context.Background()); len(m) != 2 {
		t.Fatalf("disk restore failed: %+v", m)
	}
}

// ---- 多轮记忆 ----

func TestRunSecondTurnSeesHistory(t *testing.T) {
	workDir := t.TempDir()
	p := &fakeProvider{responses: []*schema.Message{
		{Role: schema.RoleAssistant, Content: "answer one"},
		{Role: schema.RoleAssistant, Content: "answer two"},
	}}
	sess := NewSessionMessage().GetOrCreate("chat-1", workDir)
	agent := newTestAgent(t, p, tools.NewToolRegistry(), workDir).WithSession(sess)

	if err := agent.Run(context.Background(), "q1"); err != nil {
		t.Fatalf("Run 1: %v", err)
	}
	if err := agent.Run(context.Background(), "q2"); err != nil {
		t.Fatalf("Run 2: %v", err)
	}

	if len(p.gotHistory) != 2 {
		t.Fatalf("want 2 Generate calls, got %d", len(p.gotHistory))
	}
	hist := p.gotHistory[1]
	if len(hist) != 4 {
		t.Fatalf("2nd turn context want 4 messages, got %d: %+v", len(hist), hist)
	}
	if hist[1].Role != schema.RoleUser || hist[1].Content != "q1" {
		t.Fatalf("hist[1] = %+v, want user 'q1'", hist[1])
	}
	if hist[2].Role != schema.RoleAssistant || hist[2].Content != "answer one" {
		t.Fatalf("hist[2] = %+v, want assistant 'answer one'", hist[2])
	}
	if hist[3].Role != schema.RoleUser || hist[3].Content != "q2" {
		t.Fatalf("hist[3] = %+v, want user 'q2'", hist[3])
	}
}

// ---- 失败轮不落盘 ----

func TestRunFailedTurnNotPersisted(t *testing.T) {
	workDir := t.TempDir()
	p := &fakeProvider{err: errors.New("api boom")}
	sess := NewSessionMessage().GetOrCreate("chat-1", workDir)
	agent := newTestAgent(t, p, tools.NewToolRegistry(), workDir).WithSession(sess)

	if err := agent.Run(context.Background(), "will fail"); err == nil {
		t.Fatal("Run should fail when provider errors")
	}
	if mem := sess.GetWorkingMemory(context.Background()); len(mem) != 0 {
		t.Fatalf("failed turn must not be persisted, got %+v", mem)
	}
}

// ---- 工具循环整轮落盘 ----

func TestRunToolLoopTurnPersisted(t *testing.T) {
	workDir := t.TempDir()
	p := &fakeProvider{responses: []*schema.Message{
		{
			Role:    schema.RoleAssistant,
			Content: "calling tool",
			ToolCalls: []schema.ToolCall{{
				ID: "tc-1", Name: "fake_tool", Arguments: []byte("{}"),
			}},
		},
		{Role: schema.RoleAssistant, Content: "final answer"},
	}}
	reg := tools.NewToolRegistry()
	if err := reg.Registry(fakeTool{}); err != nil {
		t.Fatalf("register fake tool: %v", err)
	}
	sess := NewSessionMessage().GetOrCreate("chat-1", workDir)
	agent := newTestAgent(t, p, reg, workDir).WithSession(sess)

	if err := agent.Run(context.Background(), "use tool"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mem := sess.GetWorkingMemory(context.Background())
	if len(mem) != 4 {
		t.Fatalf("want 4 messages in session, got %d: %+v", len(mem), mem)
	}
	if mem[0].Role != schema.RoleUser || mem[0].Content != "use tool" {
		t.Fatalf("mem[0] = %+v, want user 'use tool'", mem[0])
	}
	if mem[1].Role != schema.RoleAssistant || len(mem[1].ToolCalls) != 1 {
		t.Fatalf("mem[1] = %+v, want assistant with 1 tool call", mem[1])
	}
	if mem[2].Role != schema.RoleUser || mem[2].ToolCallID != "tc-1" || mem[2].Content != "fake result" {
		t.Fatalf("mem[2] = %+v, want tool result for tc-1", mem[2])
	}
	if mem[3].Role != schema.RoleAssistant || mem[3].Content != "final answer" {
		t.Fatalf("mem[3] = %+v, want assistant 'final answer'", mem[3])
	}
	assertNoConsecutiveUsers(t, mem)
}
