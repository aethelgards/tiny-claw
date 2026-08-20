package lark

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aethelgards/tiny-claw/internal/reporter"
)

// messageSender 抽象 Bot 发送能力，便于测试注入 fake。
type messageSender interface {
	SendMessage(ctx context.Context, chatID, tenantKey, content string) error
	SendCardMessage(ctx context.Context, chatID, tenantKey, cardJSON string) error
}

// LarkReporter 实现 engine.Reporter，把引擎进度/结果回传到指定 chat。
// 发送通过互斥锁串行化——引擎的工具调用是并发的，防止消息乱序。
type LarkReporter struct {
	bot       messageSender
	chatID    string
	tenantKey string
	mu        sync.Mutex
}

func NewLarkReporter(bot *Bot, chatID, tenantKey string) reporter.Reporter {
	return newLarkReporter(bot, chatID, tenantKey)
}

// newLarkReporter 用接口注入（测试传 fake），NewLarkReporter 是生产入口。
func newLarkReporter(bot messageSender, chatID, tenantKey string) reporter.Reporter {
	return &LarkReporter{
		bot:       bot,
		chatID:    chatID,
		tenantKey: tenantKey,
	}
}

// send 串行发送一条文本消息到 chat；失败仅记录日志，不中断引擎主流程。
func (l *LarkReporter) send(ctx context.Context, content string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.bot.SendMessage(ctx, l.chatID, l.tenantKey, content); err != nil {
		slog.ErrorContext(ctx, "lark reporter send failed",
			slog.String("chatID", l.chatID),
			slog.String("err", err.Error()))
		return err
	}
	return nil
}

// sendCard 串行发送一张卡片消息到 chat；失败仅记录日志，不中断引擎主流程。
func (l *LarkReporter) sendCard(ctx context.Context, cardJSON string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.bot.SendCardMessage(ctx, l.chatID, l.tenantKey, cardJSON); err != nil {
		slog.ErrorContext(ctx, "lark reporter send card failed",
			slog.String("chatID", l.chatID),
			slog.String("err", err.Error()))
		return err
	}
	return nil
}

func (l *LarkReporter) OnThinking(ctx context.Context) {
	//_ = l.send(ctx, "🤔 思考中…")
}

func (l *LarkReporter) OnToolCall(ctx context.Context, toolName string, args string) {
	_ = l.send(ctx, "🔧 正在执行工具: "+toolName)
}

func (l *LarkReporter) OnToolResult(ctx context.Context, toolName string, result string, isError bool) {
	if isError {
		_ = l.send(ctx, "⚠️ 工具执行失败: "+toolName)
	}
}

// OnMessage 以 markdown 卡片发送引擎最终输出，支持渲染 md 格式内容；
// 卡片构建失败时回退为纯文本发送。
func (l *LarkReporter) OnMessage(ctx context.Context, content string) {
	if card := BuildMarkdownCard(content); card != "" {
		_ = l.sendCard(ctx, card)
		return
	}
	_ = l.send(ctx, content)
}

// SendApprovalMessage 发送审批卡片；构建失败返回错误（WaitingForApproval 据此 fail-closed）。
func (l *LarkReporter) SendApprovalMessage(ctx context.Context, taskID string, toolName string, args string) error {
	card := BuildApprovalCard(taskID, toolName, args)
	if card == "" {
		return fmt.Errorf("build approval card failed: taskID=%s", taskID)
	}
	return l.sendCard(ctx, card)
}
