package engine

import (
	"context"
	"fmt"
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

func NewTerminalReporter() Reporter {
	return &TerminalReporter{}
}
