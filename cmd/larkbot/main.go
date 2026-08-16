package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/aethelgards/tiny-claw/internal/approval"
	"github.com/aethelgards/tiny-claw/internal/config"
	ctxpkg "github.com/aethelgards/tiny-claw/internal/context"
	"github.com/aethelgards/tiny-claw/internal/engine"
	"github.com/aethelgards/tiny-claw/internal/gateway/lark"
	"github.com/aethelgards/tiny-claw/internal/helper"
	"github.com/aethelgards/tiny-claw/internal/provider"
	"github.com/aethelgards/tiny-claw/internal/tools"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// main 是 Lark 机器人模式的入口：
//  1. 加载配置（含 larkAppId/larkAppSecret）
//  2. 初始化 provider 与工具注册表
//  3. 建立消息队列 + 单 worker 消费管道
//  4. 启动 WebSocket 长连接，事件 handler 只做「解析+入队」后立即返回
//  5. 监听 SIGINT/SIGTERM 优雅退出（worker 处理完当前消息后停止）
func main() {
	settings, err := config.Load()
	if err != nil {
		slog.Error("配置加载失败", "err", err)
		os.Exit(1)
	}
	if settings.LarkAppID == "" || settings.LarkAppSecret == "" {
		slog.Error("缺少 lark 配置：请设置 larkAppId/larkAppSecret 或 CLAW_LARK_APP_ID/CLAW_LARK_APP_SECRET")
		os.Exit(1)
	}

	p, err := provider.NewProvider(*settings)
	if err != nil {
		slog.Error("provider 初始化失败", "err", err)
		os.Exit(1)
	}

	reg := tools.NewToolRegistry()
	for name, tool := range map[string]tools.BaseTool{
		"read_file":      tools.NewReadFileTool(settings.WorkDir),
		"write_file":     tools.NewWriteFileTool(settings.WorkDir),
		"edit_file":      tools.NewEditFileTool(settings.WorkDir),
		"delete_file":    tools.NewDeleteFileTool(settings.WorkDir),
		"bash":           tools.NewBashTool(settings.WorkDir),
		"spawn_subagent": engine.NewSubAgent(settings, p),
	} {
		if err := reg.Registry(tool); err != nil {
			slog.Error("工具注册失败", "tool", name, "err", err)
			os.Exit(1)
		}
	}
	// 技能作为 tool 注册：模型按需调用加载正文，避免 v1 全量注入导致的 token 爆炸。
	if err := ctxpkg.RegisterSkills(reg, settings.WorkDir); err != nil {
		slog.Error("register skill failed", slog.String("err", err.Error()))
	}

	// 审批：创建审批管理器并接入注册表中间件（危险命令经卡片审批；非法超时回退默认 5m）
	approvalMgr := approval.NewApprovalManager(approval.ParseApprovalTimeout(settings.ApprovalTimeout))
	reg.Use(approval.ApprovalMiddleware(approvalMgr))

	queue := lark.NewMessageQueue(settings.LarkChannelSize)
	bot := lark.NewBot(settings.LarkAppID, settings.LarkAppSecret)

	worker := lark.NewWorker(queue,
		lark.NewEngineProcessor(p, reg, *settings, bot, ctxpkg.NewPromptComposer(settings.WorkDir, settings.PlanMode)),
		func(ctx context.Context, msg lark.IncomingMessage, err error) {
			reply := "处理消息失败：" + err.Error()
			if len(reply) > 400 {
				reply = reply[:400] + "…"
			}
			if sendErr := bot.SendMessage(ctx, msg.ChatID, msg.TenantKey, reply); sendErr != nil {
				slog.ErrorContext(ctx, "错误回复发送失败", "err", sendErr)
			}
		},
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := bot.Start(ctx, func(d *dispatcher.EventDispatcher) {
			d.OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
				slog.InfoContext(ctx, "recv lark event", slog.String("event", helper.Any2Json(event)))
				msg, ok := lark.ParseMessageEvent(event)
				if !ok {
					slog.WarnContext(ctx, "parse lark event failed", slog.String("event", helper.Any2Json(event)))
					return nil
				}
				if !queue.Enqueue(msg) {
					slog.WarnContext(ctx, "消息入队失败（重复或队列已满），丢弃",
						slog.String("msgID", msg.MessageID),
						slog.String("chatID", msg.ChatID))
				} else {
					slog.InfoContext(ctx, "message enqueue success", slog.String("msgID", msg.MessageID))
				}
				return nil
			}).OnP2CardActionTrigger(lark.NewApprovalCardHandler(approvalMgr))
		}); err != nil {
			slog.Error("lark bot 启动失败", "err", err)
			os.Exit(1)
		}
	}()

	go worker.Run(ctx)
	slog.Info("lark bot 已启动，等待消息…")

	<-ctx.Done()
	slog.Info("收到退出信号，正在关闭…")
	bot.Stop()
}
