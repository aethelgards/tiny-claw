package engine

import (
	"context"
	"fmt"

	"github.com/aethelgards/tiny-claw/internal/reporter"
)

type TerminalReporter struct {
}

func (t *TerminalReporter) OnThinking(ctx context.Context) {
	fmt.Println("🤔思考中...")
}

func (t *TerminalReporter) OnToolCall(ctx context.Context, toolName string, args string) {
	fmt.Printf("🛠️ 调用工具-> %s\n", toolName)
}

func (t *TerminalReporter) OnToolResult(ctx context.Context, toolName string, result string, isError bool) {

}

func (t *TerminalReporter) OnMessage(ctx context.Context, content string) {
	fmt.Printf("🦞-> %s\n", content)
}

func (l *TerminalReporter) SendApprovalMessage(ctx context.Context, taskID string, toolName string, args string) error {
	noticeMsg := fmt.Sprintf(
		`⚠️ **高危操作审批请求** 
Agent 试图执行以下动作:
- 工具: %s
- 参数: %s
任务 ID: **%s**
👉 请在此消息下方回复 "approve %s" 或 "reject %s" 来决定是否放行。`, toolName, args, taskID, taskID, taskID)
	fmt.Println(noticeMsg)
	return nil
}

func NewTerminalReporter() reporter.Reporter {
	return &TerminalReporter{}
}
