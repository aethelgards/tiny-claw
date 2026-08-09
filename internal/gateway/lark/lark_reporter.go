package lark

import (
	"context"
	"log/slog"
	"sync"

	"github.com/aethelgards/tiny-claw/internal/engine"
)

// LarkReporter 实现 engine.Reporter，把引擎进度/结果回传到指定 chat。
// 发送通过互斥锁串行化——引擎的工具调用是并发的，防止消息乱序。
type LarkReporter struct {
	bot       *Bot
	chatID    string
	tenantKey string
	mu        sync.Mutex
}

func NewLarkReporter(bot *Bot, chatID, tenantKey string) engine.Reporter {
	return &LarkReporter{
		bot:       bot,
		chatID:    chatID,
		tenantKey: tenantKey,
	}
}

// send 串行发送一条文本消息到 chat；失败仅记录日志，不中断引擎主流程。
func (l *LarkReporter) send(ctx context.Context, content string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.bot.SendMessage(ctx, l.chatID, l.tenantKey, content); err != nil {
		slog.ErrorContext(ctx, "lark reporter send failed",
			slog.String("chatID", l.chatID),
			slog.String("err", err.Error()))
	}
}

func (l *LarkReporter) OnThinking(ctx context.Context) {
	l.send(ctx, "🤔 思考中…")
}

func (l *LarkReporter) OnToolCall(ctx context.Context, toolName string, args string) {
	l.send(ctx, "🔧 正在执行工具: "+toolName)
}

func (l *LarkReporter) OnToolResult(ctx context.Context, toolName string, result string, isError bool) {
	if isError {
		l.send(ctx, "⚠️ 工具执行失败: "+toolName)
	}
}

func (l *LarkReporter) OnMessage(ctx context.Context, content string) {
	l.send(ctx, content)
}
