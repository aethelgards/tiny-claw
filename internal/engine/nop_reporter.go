package engine

import "context"

// NopReporter 是空实现 Reporter，供不需要回传进度的场景（如 CLI 模式）使用。
type NopReporter struct{}

func NewNopReporter() *NopReporter {
	return &NopReporter{}
}

func (NopReporter) OnThinking(context.Context)                 {}
func (NopReporter) OnToolCall(context.Context, string, string) {}
func (NopReporter) OnToolResult(context.Context, string, string, bool) {
}
func (NopReporter) OnMessage(context.Context, string) {}
