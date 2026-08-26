package context

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/aethelgards/tiny-claw/internal/memory"
	"github.com/aethelgards/tiny-claw/internal/schema"
)

type PromptComposer struct {
	workDir       string
	planMode      bool
	memoryInjector *memory.MemoryInjector
}

func NewPromptComposer(workDir string, planMode bool) *PromptComposer {
	return &PromptComposer{
		workDir:  workDir,
		planMode: planMode,
	}
}

func (c *PromptComposer) WithMemoryInjector(injector *memory.MemoryInjector) {
	c.memoryInjector = injector
}

// skillStrategy 是固定的专业技能使用策略文案（常驻，~100 tokens）。
// 技能正文不再注入 system prompt，而是作为 tool 按需加载。
var skillStrategy = `
## 专业技能使用策略
你拥有若干可调用的专业技能工具。当当前任务明确匹配某技能工具的 description 时，
必须调用该工具加载其执行指南并严格遵循；不要凭记忆猜测技能内容。
`

var promptCore = `
# 核心身份你名叫 tiny-claw，一个由驾驭工程驱动的骨灰级研发助手。
你具备极简主义哲学，拒绝废话。
你能通过系统提供的内置工具，创建、读取、修改和执行工作区中的代码。
# 核心纪律 (CRITICAL)
1. 如需检查文件是否存在，请使用 bash 的 ls 或 test -f，而不是对目录使用 read_file。
2. 创建新文件时，务必使用 write_file，并同时提供 path 和 content 参数。
3. 编辑文件前务必先读取现有文件，以理解上下文。
4. 无论何时你需要写代码或创建文件，都要直接使用 write_file 工具。
5. 遇到工具执行报错时，仔细阅读 stderr，尝试自己修正命令并重试。
6. 始终用中文回复，以便传达你的进展和想法。
`

var planMode = `
# 长程任务与状态外部化强制规范 (Plan Mode: ON)!!! 
警告：本模式下，你绝对不能依赖自己的短期记忆。你必须将所有的架构思路和执行进度持久化到物理文件中 !!!
当你收到一条新指令被唤醒时，你必须、且只能按照以下【绝对顺序】执行你的动作：
**[STEP 1: 强制环境嗅探 (Bootstrapping)]**
- 收到指令后，你必须第一时间使用 bash (如: ` + "`ls -la`" + `) 检查当前工作区根目录下是否已经存在 ` + "`PLAN.md`" + ` 和 ` + "`TODO.md`" + `。
- **分支 A (全新任务)**：如果这两个文件不存在，说明这是一个全新的任务。你必须使用 write_file 依次创建它们： 
	1. 先创建 ` + "`PLAN.md`" + `，写下你的理解、架构设计、技术选型。
	2. 再创建 ` + "`TODO.md`" + `，拆解出具体的可执行步骤（使用标准的 Markdown Checkbox 格式，如 ` + "`- [ ] 步骤1`" + `）。
- **分支 B (断点续传/任务唤醒)**：如果这两个文件已经存在，**绝对不要覆盖它们！** 这意味着系统刚刚重启，或者人类接管了进度。你必须立即使用 read_file 仔细阅读 ` + "`PLAN.md`" + ` 了解全局目标，并阅读 ` + "`TODO.md`" + ` 寻找第一个未被打勾的 ` + "`- [ ]`" + ` 任务，从那里直接继续干活。
**[STEP 2: 严格的单步执行与实时打勾]**
- 开始执行 ` + "`TODO.md`" + ` 中未完成的任务。- **强制约束**：每当你通过 write_file 或 bash 真正完成了一个子任务后，你**必须立即停下来**，优先使用 edit_file 工具（或 bash 的 sed 命令），将 ` + "`TODO.md`" + ` 中对应的行修改为 ` + "`- [x]`" + `。
- 绝对不允许“一口气写完所有代码最后再打勾”。做完一步，必须打勾一步！
**[STEP 3: 迷失时的自救]**
- 如果你在执行中遇到了报错，或者不知道下一步该干嘛了，立即使用 read_file 重新读取 ` + "`TODO.md`" + ` 确认自己的位置。
**[STEP 4: 及时清理完成的TODO和PLAN]**
- 如果所有的TODO都已完成，那么需要将当前的TODO.md和PLAN.md进行删除，避免下次执行时重复读取已经完成的任务。
`

func (c *PromptComposer) Build() schema.Message {
	var promptBuilder strings.Builder

	promptBuilder.WriteString(promptCore)

	if c.planMode {
		promptBuilder.WriteString("\n\n")
		promptBuilder.WriteString(planMode)
		promptBuilder.WriteString("\n\n")
	}

	projectAgentPath := filepath.Join(c.workDir, "AGENT.md")

	if raw, err := os.ReadFile(projectAgentPath); err == nil {
		promptBuilder.WriteString("\n# 项目专属指南(来自AGENT.md)\n")
		promptBuilder.WriteString("以下是当前工作区特有的架构规范与注意事项，你的行为必须绝对符合以下要求：\n")
		promptBuilder.WriteString("```markdown\n\n")
		promptBuilder.WriteString(string(raw))
		promptBuilder.WriteString("```\n")
	}

	promptBuilder.WriteString(skillStrategy)

	// 注入长期记忆块
	if c.memoryInjector != nil {
		if memBlock := c.memoryInjector.Recent(context.Background()); len(memBlock) > 0 {
			promptBuilder.WriteString("\n# 长期记忆（来自记忆系统）\n")
			promptBuilder.WriteString("以下是从历史会话中沉淀的经验，帮助你避免重复犯错、贴合用户习惯：\n")
			for _, m := range memBlock {
				promptBuilder.WriteString("- [")
				promptBuilder.WriteString(string(m.Type))
				promptBuilder.WriteString("] ")
				promptBuilder.WriteString(m.Content)
				promptBuilder.WriteString("\n")
			}
			promptBuilder.WriteString("\n以上记忆可能不完全适用当前任务，以实际代码为准。\n")
		}
	}

	slog.Debug("claw system prompt", slog.String("path", projectAgentPath), slog.String("prompt", promptBuilder.String()))

	return schema.Message{
		Role:    schema.RoleSystem,
		Content: promptBuilder.String(),
	}
}
