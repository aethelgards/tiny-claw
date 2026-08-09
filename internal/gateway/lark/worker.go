package lark

import (
	"context"
	"fmt"
	"log/slog"

	ctxpkg "github.com/aethelgards/tiny-claw/internal/context"
	"github.com/aethelgards/tiny-claw/internal/tookit"
)

// Processor 消费端处理单元，由装配方注入（测试时可注入 fake）。
type Processor interface {
	Process(ctx context.Context, msg IncomingMessage) error
}

// onError 可选回调：处理失败/panic 时由装配方注入，用于回复错误提示。
type onError func(ctx context.Context, msg IncomingMessage, err error)

// Worker 单 goroutine 串行消费队列消息。
// 同一时间只处理一条消息，天然保证处理顺序；
// ctx 取消时处理完当前消息后退出，不中断进行中的引擎任务。
type Worker struct {
	queue          *MessageQueue
	processor      Processor
	onError        onError
	promptComposer *ctxpkg.PromptComposer
}

func NewWorker(q *MessageQueue, p Processor, onError ...onError) *Worker {
	w := &Worker{queue: q, processor: p}
	if len(onError) > 0 {
		w.onError = onError[0]
	}
	return w
}

// Run 阻塞消费直到 ctx 取消。处理完当前消息才退出。
func (w *Worker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "worker stopped by ctx cancel")
			return
		case msg, ok := <-w.queue.Messages():
			if !ok {
				return
			}
			w.safeProcess(ctx, msg)
		}
	}
}

// safeProcess 包裹 processor.Process，捕获 panic 保证 worker 不因单个消息崩溃。
func (w *Worker) safeProcess(ctx context.Context, msg IncomingMessage) {
	slog.InfoContext(ctx, "worker processing message", slog.String("msg", tookit.Any2Json(msg)))
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic: %v", r)
			slog.ErrorContext(ctx, "worker recovered panic",
				slog.String("msgID", msg.MessageID),
				slog.Any("panic", r))
			w.notifyError(ctx, msg, err)
		}
	}()

	if err := w.processor.Process(ctx, msg); err != nil {
		slog.ErrorContext(ctx, "process message failed",
			slog.String("msgID", msg.MessageID),
			slog.String("chatID", msg.ChatID),
			slog.String("err", err.Error()))
		w.notifyError(ctx, msg, err)
	}
}

func (w *Worker) notifyError(ctx context.Context, msg IncomingMessage, err error) {
	if w.onError != nil {
		w.onError(ctx, msg, err)
	}
}
