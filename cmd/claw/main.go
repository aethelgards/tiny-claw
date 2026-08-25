package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/aethelgards/tiny-claw/internal/approval"
	"github.com/aethelgards/tiny-claw/internal/config"
	ctxpkg "github.com/aethelgards/tiny-claw/internal/context"
	"github.com/aethelgards/tiny-claw/internal/dashboard"
	"github.com/aethelgards/tiny-claw/internal/engine"
	"github.com/aethelgards/tiny-claw/internal/memory"
	"github.com/aethelgards/tiny-claw/internal/observability"
	"github.com/aethelgards/tiny-claw/internal/provider"
	"github.com/aethelgards/tiny-claw/internal/tools"
)

const usage = `用法: claw <command> [flags]

命令:
  serve      启动可视化观测面板 HTTP 服务
  <prompt>   交给 Agent 执行的提示词（默认行为）

示例:
  claw serve
  claw serve --port 3000
  claw serve --data-dir /path/to/traces
  claw "给这个项目添加单元测试"
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Fprint(os.Stderr, usage)
	default:
		runAgent(os.Args[1:])
	}
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 8080, "HTTP 服务监听端口")
	portShort := fs.Int("p", 8080, "HTTP 服务监听端口 (简写)")
	dataDir := fs.String("data-dir", ".claw/traces", "trace 数据存储目录")
	dataDirShort := fs.String("d", ".claw/traces", "trace 数据存储目录 (简写)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	actualPort := *port
	actualDataDir := *dataDir
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "p":
			actualPort = *portShort
		case "d":
			actualDataDir = *dataDirShort
		}
	})

	storage := observability.NewStorage(actualDataDir)
	srv := dashboard.NewServer(storage)

	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", actualPort),
		Handler: srv.Handler(),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		slog.Info("收到终止信号，正在关闭服务…")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutCtx); err != nil {
			slog.Error("服务关闭出错", "err", err)
		}
	}()

	addr := fmt.Sprintf("http://localhost:%d", actualPort)
	slog.Info("Dashboard 服务已启动", "addr", addr, "dataDir", actualDataDir)
	fmt.Printf("🦞 Dashboard 已启动: %s\n", addr)
	fmt.Printf("   数据目录: %s\n", actualDataDir)
	fmt.Println("   按 Ctrl+C 停止服务")

	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("服务启动失败", "err", err)
		os.Exit(1)
	}
}

func runAgent(args []string) {
	// 1. 加载配置
	settings, err := config.Load()
	if err != nil {
		slog.Error("配置加载失败", "err", err)
		os.Exit(1)
	}

	// 2. 初始化 provider
	p, err := provider.NewProvider(*settings)
	if err != nil {
		slog.Error("provider 初始化失败", "err", err)
		os.Exit(1)
	}

	// 3. 注册工具
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
	composer := ctxpkg.NewPromptComposer(settings.WorkDir, settings.PlanMode)

	// 记忆系统接线
	home, _ := os.UserHomeDir()
	memStore, err := memory.NewMemoryStore(
		filepath.Join(home, ".claw", "memory"),             // 全局
		filepath.Join(settings.WorkDir, ".claw", "memory"), // 项目
	)
	if err != nil {
		slog.Error("记忆存储初始化失败", "err", err)
		os.Exit(1)
	}

	// 记忆三工具注册
	for _, tool := range []tools.BaseTool{
		memory.NewSaveMemoryTool(memStore),
		memory.NewRecallMemoryTool(memStore),
		memory.NewForgetMemoryTool(memStore),
	} {
		if err := reg.Registry(tool); err != nil {
			slog.Error("记忆工具注册失败", "err", err)
			os.Exit(1)
		}
	}

	// Composer 注入记忆
	composer.WithMemoryInjector(memory.NewMemoryInjector(memStore, settings.Memory.MaxInjectTokens))

	// 4. 审批：危险命令经终端交互审批（非交互 stdin 一律拒绝；非法超时回退默认 5m）
	approvalMgr := approval.NewApprovalManager(approval.ParseApprovalTimeout(settings.ApprovalTimeout))
	reg.Use(approval.ApprovalMiddleware(approvalMgr))
	reporter := approval.NewTerminalReporter(approvalMgr)

	// 5. 创建 Agent 引擎
	agent := engine.NewAgentEngine(p, reg, *settings, reporter, composer,
		ctxpkg.NewRecoveryManager(),
		engine.NewReminderInjector(3),
	)

	// 6. 校验 prompt
	if len(args) == 0 {
		slog.Error("用法: claw <prompt>")
		os.Exit(1)
	}

	// 7. 启动 Agent
	runCtx := approval.WithApprovalContext(context.Background(), reporter, "local")
	if err := agent.Run(runCtx, args[0]); err != nil {
		slog.Error("运行失败", "err", err)
		os.Exit(1)
	}

	// 【R9】CLI 同步 Compact
	if _, err := memStore.Compact(settings.Memory.CompactThreshold); err != nil {
		slog.Warn("记忆整理失败", "err", err)
	}
}
