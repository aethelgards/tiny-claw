package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aethelgards/tiny-claw/internal/config"
	ctxpkg "github.com/aethelgards/tiny-claw/internal/context"
	"github.com/aethelgards/tiny-claw/internal/engine"
	"github.com/aethelgards/tiny-claw/internal/provider"
	"github.com/aethelgards/tiny-claw/internal/tools"
)

// main 是 claw 程序的入口函数，负责整个程序的初始化与启动流程：
//  1. 加载配置文件（如 API Key、工作目录等）
//  2. 初始化模型 Provider（OpenAI / Claude 等）
//  3. 注册可供 AI 调用的工具（读写文件、执行命令等）
//  4. 创建 Agent 引擎并运行，把命令行第一个参数作为 prompt 交给 AI 执行
//
// 任何一步失败都会记录错误日志并退出（退出码为 1）。
func main() {
	// 1. 加载配置：读取环境变量/配置文件，得到 API Key、工作目录等设置
	settings, err := config.Load()
	if err != nil {
		slog.Error("配置加载失败", "err", err)
		os.Exit(1)
	}

	// 2. 初始化 provider：根据配置创建对应的模型客户端（OpenAI / Claude）
	p, err := provider.NewProvider(*settings)
	if err != nil {
		slog.Error("provider 初始化失败", "err", err)
		os.Exit(1)
	}

	// 3. 注册工具：把内置工具（读文件、写文件、编辑文件、执行 bash）注册进工具注册表，
	//    这样 AI 在对话过程中才能调用它们操作工作区
	reg := tools.NewToolRegistry()
	for name, tool := range map[string]tools.BaseTool{
		"read_file":  tools.NewReadFileTool(settings.WorkDir),
		"write_file": tools.NewWriteFileTool(settings.WorkDir),
		"edit_file":  tools.NewEditFileTool(settings.WorkDir),
		"bash":       tools.NewBashTool(settings.WorkDir),
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

	composer := ctxpkg.NewPromptComposer(settings.WorkDir, settings.PlanMode)

	// 4. 创建 Agent 引擎，将 provider 与工具注册表绑定起来（CLI 模式用空实现 Reporter）
	agent := engine.NewAgentEngine(p, reg, *settings, engine.NewTerminalReporter(), composer)

	// 5. 校验命令行参数：必须提供 prompt，用法为 `claw <prompt>`
	if len(os.Args) < 2 {
		slog.Error("用法: claw <prompt>")
		os.Exit(1)
	}

	// 6. 启动 Agent：把命令行参数作为用户输入交给 AI 循环处理，直到任务完成
	if err := agent.Run(context.Background(), os.Args[1]); err != nil {
		slog.Error("运行失败", "err", err)
		os.Exit(1)
	}
}
