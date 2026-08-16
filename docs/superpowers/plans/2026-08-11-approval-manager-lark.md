# ApprovalManager 接入飞书卡片审批 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `internal/gateway/lark/approval.go` 的半成品 `ApprovalManager` 改造为完整的**飞书卡片审批**链路：危险命令执行前经卡片审批（通过/拒绝+原因），发起人身份校验，5 分钟超时自动拒绝，发送失败 fail-closed，全部经 `tools.MiddlewareFunc` 接入注册表（引擎层零改动），卡片 JSON 手工构建（`larkcard` 包无 form 容器 builder）。

**Architecture:**

```
tools.Registry.Execute ──> ApprovalMiddleware (reg.Use) ──> isDangerousCommand
                                        │ 命中危险命令
                                        ▼
                        ApprovalManager.WaitingForApproval (阻塞)
                                        │ ① 注册 task ② SendApprovalMessage 发卡片
                           ┌────────────┴─────────────────┐
                           │ select: res.ch / timeout / done │
                           └────────────┬─────────────────┘
                                        ▼
         Worker 侧解除阻塞 → 返回 (allow, reason) → 中间件放行/拦截
                        ▲
  卡片按钮点击 ──> OnP2CardActionTrigger handler ──> getTask 校验身份 ──> ResolveApproval ──> CardActionTriggerResponse{Toast, Card 结果卡}
```

**Tech Stack:** Go 1.26.3；larksuite/oapi-sdk-go/v3 v3.9.10（已有依赖，不新增）。复用：`tools.MiddlewareFunc`、`engine.Reporter`、SDK `event/dispatcher/callback`、`larkim.MsgTypeInteractive`。

## Global Constraints

- **不引入新依赖**；只用标准库 + 已有 larksuite SDK v3.9.10
- **卡片 JSON 必须手工构建**（`map[string]any` + `json.Marshal`）：已确认 `larkcard` 包无 input/form 容器 builder；v2 不支持 `{"tag":"action","actions":[...]}` 旧模块
- **回调事件固定走 `d.OnP2CardActionTrigger`**（事件类型 `"card.action.trigger"`），Go SDK 无 `OnP2CardActionTriggerV1`
- **回调字段（已从 SDK 源码 v3.9.10 验证）**：`event.Event.Action.Name` / `Action.Value map[string]interface{}` / `Action.FormValue map[string]interface{}` / `Operator.OpenID`（普通 string，非指针）；按钮身份以 `Action.Name` 为准、`Value["action"]` 交叉校验；拒绝原因取 `FormValue["reject_reason"]`
- **审批 fail-closed**：通知发送失败 / reporter 缺失 / 超时 / ctx 取消 → 一律拒绝执行
- **`WaitingForApproval` 阻塞期间任务必须可从 map 清除**，超时竞态下 `ResolveApproval` 的 channel send 不阻塞（chan 缓冲 1 + 非阻塞 select）
- **结果卡异步兜底（`Message.Patch` + `update_multi`）不在本次实现范围**：结果卡为纯内存字符串构建（微秒级），必然落在回调 3s 窗口内；handler 签名（仅拿 `*ApprovalManager`）无 client 访问，YAGNI
- **本仓库存在大量未提交改动**：验证统一用 `gofmt -l internal/ cmd/` + `go build ./...` + `go test ./...`，**不执行 git commit**（沿用 2026-08-08-config-system.md 的验证约定）
- **测试复用 `waitFor`**（`worker_test.go` 已定义 `func waitFor(t *testing.T, timeout time.Duration, cond func() bool)`，package lark），不得重复声明
- Go 1.26.3 内置 `new(value)` 返回 `*T`（现有测试已使用，如 `new("ou_123")`），测试中可直接用
- **引擎层（internal/engine）与工具实现零改动**；只改 `internal/gateway/lark/`、`internal/config`、`cmd/larkbot/main.go`

---

### Task 1: `approval.go` — ApprovalManager 增强（任务元数据 + 超时 + fail-closed + bool 投递）

**Files:**
- Modify: `internal/gateway/lark/approval.go`
- Test: `internal/gateway/lark/approval_test.go`（新建）

**Interfaces:**
- 产出：
  - `type approvalTask struct { ch chan ApprovalResult; taskID, approverOpenID, toolName, args string }`（比设计稿 §4.1 草稿多 `taskID` 字段——结果卡需展示任务 ID，`BuildApprovalResultCard(task approvalTask, ...)` 签名需要它）
  - `func NewApprovalManager(timeout time.Duration) *ApprovalManager`（`timeout <= 0` → 默认 5 分钟）
  - `func (m *ApprovalManager) WaitingForApproval(ctx, taskID, toolName, args string, reporter engine.Reporter, approverOpenID string) (bool, string)`
  - `func (m *ApprovalManager) ResolveApproval(ctx context.Context, taskID string, allowed bool, reason string) bool`
  - `func (m *ApprovalManager) getTask(taskID string) (approvalTask, bool)`
  - `func ParseApprovalTimeout(raw string) time.Duration`（导出，供 main.go 使用；非法/<=0 回退默认 5m + slog.Warn）
  - `func isDangerousCommand(toolName, args string) bool`（保留现有语义）

**Steps:**

1. 先写失败测试 `internal/gateway/lark/approval_test.go`（package lark）：
   - `TestApprovalResolve`：goroutine 里 `WaitingForApproval`，`waitFor` 轮询 `getTask` 出现后 `ResolveApproval(true,"")` → 返回 `(true, "")`，`ResolveApproval` 返回 true，任务已清理
   - `TestApprovalResolveRejectReason`：`ResolveApproval(false,"原因X")` → `(false, "原因X")`
   - `TestApprovalTimeoutDeny`：`NewApprovalManager(50*time.Millisecond)` → `(false, 含"审批超时")`，随后 `getTask` 不存在
   - `TestApprovalCtxCancelDeny`：`context.WithCancel` 后 cancel → `(false, 含"审批上下文已取消")`，任务已清理
   - `TestApprovalSendFailClosed`：fake reporter 返回 error → `(false, "审批通知发送失败，已拒绝执行")`，任务已清理，不阻塞
   - `TestApprovalNilReporterFailClosed`：`reporter` 传 nil → 立即 `(false, "审批通知发送失败，已拒绝执行")`，任务已清理，不阻塞
   - `TestResolveUnknownTask`：未知 taskID → `false`
   - `TestResolveAfterTimeout`：超时后再 `ResolveApproval` → `false`（不误投递）
   - `TestResolveTwiceSecondFalse`：同一任务连续两次 `ResolveApproval` → 第一次 true、第二次 false（首次投递后任务已删/缓冲已满，不重复投递；对应设计稿 §7「Manager 重复投递」）
   - `TestParseApprovalTimeout`：`"5m"→5m`、`"abc"→5m`、`"0s"→5m`、`"-1s"→5m`
   - `TestIsDangerousCommand` 表驱动：`("bash","rm -rf /")→true`、`("bash","sudo apt")→true`、`("bash","drop table")→true`、`("bash","cat > x.go")→true`、`("bash","ls -la")→false`、`("write_file","rm -r")→false`、`("edit_file", anything)→false`、`("read_file", anything)→false`
   - 测试文件内定义 fake reporter（Task 5 复用；`SendApprovalMessage` 记录调用并透传注入的错误）：

```go
type fakeApprovalReporter struct {
	sendErr error
	calls   int
}

func (f *fakeApprovalReporter) OnThinking(ctx context.Context)                        {}
func (f *fakeApprovalReporter) OnToolCall(ctx context.Context, toolName, args string) {}
func (f *fakeApprovalReporter) OnToolResult(ctx context.Context, toolName, result string, isError bool) {
}
func (f *fakeApprovalReporter) OnMessage(ctx context.Context, content string) {}
func (f *fakeApprovalReporter) SendApprovalMessage(ctx context.Context, taskID, toolName, args string) error {
	f.calls++
	return f.sendErr
}
```

   - 运行 `go test ./internal/gateway/lark/ -run 'TestApproval|TestIsDangerous' -v` → 编译失败（approvalTask 未定义/签名不符）
2. 重写 `internal/gateway/lark/approval.go`：

```go
package lark

import (
	"context"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/aethelgards/tiny-claw/internal/engine"
)

type ApprovalResult struct {
	Allowed      bool
	RejectReason string
}

var dangerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+-r`),
	regexp.MustCompile(`sudo`),
	regexp.MustCompile(`\bdrop\b`),
	regexp.MustCompile(`>.*\.go`),
}

// isDangerousCommand 判断工具调用是否命中高危模式；仅 bash 参与判断。
func isDangerousCommand(toolName string, args string) bool {
	if toolName != "bash" {
		return false
	}
	for _, re := range dangerPatterns {
		if re.MatchString(args) {
			return true
		}
	}
	return false
}

// approvalTask 单个待审批任务：结果 channel + 元数据（结果卡展示/身份校验用）。
type approvalTask struct {
	ch             chan ApprovalResult
	taskID         string
	approverOpenID string // 谁发的请求，只有 TA 能批
	toolName       string
	args           string
}

type ApprovalManager struct {
	mu           sync.RWMutex
	pendingTasks map[string]approvalTask
	timeout      time.Duration
}

// GlobalApprovalManager 兼容既有引用；等价 NewApprovalManager(5 * time.Minute)。
var GlobalApprovalManager = NewApprovalManager(5 * time.Minute)

// NewApprovalManager 创建审批管理器；timeout <= 0 视为默认 5 分钟。
func NewApprovalManager(timeout time.Duration) *ApprovalManager {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &ApprovalManager{
		pendingTasks: make(map[string]approvalTask),
		timeout:      timeout,
	}
}

// WaitingForApproval 注册审批任务并阻塞等待结论。
// 返回 (allowed, reason)；reason 直接作为中间件拒绝文案。
// 所有出口（成功/发送失败/超时/ctx 取消）都会清理任务。
func (m *ApprovalManager) WaitingForApproval(ctx context.Context, taskID, toolName, args string, reporter engine.Reporter, approverOpenID string) (bool, string) {
	task := approvalTask{
		ch:             make(chan ApprovalResult, 1),
		taskID:         taskID,
		approverOpenID: approverOpenID,
		toolName:       toolName,
		args:           args,
	}
	m.mu.Lock()
	m.pendingTasks[taskID] = task
	m.mu.Unlock()

	// 通知发送失败（含 reporter 缺失）→ fail-closed：清理任务并拒绝执行
	if reporter == nil {
		slog.ErrorContext(ctx, "approval reporter is nil", slog.String("taskId", taskID))
		m.deleteTask(taskID)
		return false, "审批通知发送失败，已拒绝执行"
	}
	if err := reporter.SendApprovalMessage(ctx, taskID, toolName, args); err != nil {
		slog.ErrorContext(ctx, "sendApprovalMessage",
			slog.String("taskId", taskID), slog.String("toolName", toolName),
			slog.String("args", args), slog.String("err", err.Error()))
		m.deleteTask(taskID)
		return false, "审批通知发送失败，已拒绝执行"
	}

	select {
	case res := <-task.ch:
		m.deleteTask(taskID)
		return res.Allowed, res.RejectReason
	case <-time.After(m.timeout):
		slog.WarnContext(ctx, "approval timeout",
			slog.String("taskId", taskID), slog.Duration("timeout", m.timeout))
		m.deleteTask(taskID)
		return false, "审批超时，已自动拒绝"
	case <-ctx.Done():
		slog.WarnContext(ctx, "approval ctx canceled",
			slog.String("taskId", taskID), slog.String("err", ctx.Err().Error()))
		m.deleteTask(taskID)
		return false, "审批上下文已取消"
	}
}

func (m *ApprovalManager) deleteTask(taskID string) {
	m.mu.Lock()
	delete(m.pendingTasks, taskID)
	m.mu.Unlock()
}

// ResolveApproval 投递审批结果；返回是否成功投递（任务不存在/已处理 → false）。
func (m *ApprovalManager) ResolveApproval(ctx context.Context, taskID string, allowed bool, reason string) bool {
	m.mu.RLock()
	task, ok := m.pendingTasks[taskID]
	m.mu.RUnlock()
	if !ok {
		slog.WarnContext(ctx, "resolveApproval: task not exist, maybe timeout or already handled",
			slog.String("taskId", taskID))
		return false
	}
	slog.InfoContext(ctx, "resolveApproval",
		slog.String("taskId", taskID), slog.Bool("allowed", allowed), slog.String("reason", reason))
	select {
	case task.ch <- ApprovalResult{Allowed: allowed, RejectReason: reason}:
		return true
	default:
		return false // 缓冲已满（已被并发 resolve），不阻塞
	}
}

// getTask 供卡片回调处理读取任务元数据（身份校验 + 结果卡展示）。
func (m *ApprovalManager) getTask(taskID string) (approvalTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.pendingTasks[taskID]
	return t, ok
}

// ParseApprovalTimeout 解析审批超时配置；非法值/<=0 回退默认 5 分钟并告警。
func ParseApprovalTimeout(raw string) time.Duration {
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		slog.Warn("approvalTimeout 非法，回退默认 5m", slog.String("raw", raw))
		return 5 * time.Minute
	}
	return d
}
```

3. 运行 `go test ./internal/gateway/lark/ -run 'TestApproval|TestIsDangerous' -v` → 全绿

**Verification:** `gofmt -l internal/ cmd/`（无输出）+ `go build ./...` + `go test ./...`
（注意：重写把 `WaitingForApproval` 签名加了 `approverOpenID` 参数——此前无任何调用方，grep 确认只有 approval.go 自身引用，无破坏。）

---

### Task 2: `approval_card.go` — 卡片 JSON 构建（发送卡 + 结果卡 + 转义/截断）

**Files:**
- Create: `internal/gateway/lark/approval_card.go`
- Test: `internal/gateway/lark/approval_card_test.go`（新建）

**Interfaces:**
- `func BuildApprovalCard(taskID, toolName, args string) string`（返回 card JSON v2 字符串；header `red`）
- `func BuildApprovalResultCard(task approvalTask, allowed bool, reason, operatorOpenID string) string`（允许→`green`/`✅ 已通过`；拒绝→`red`/`❌ 已拒绝`，正文含审批人 + 任务 ID + 工具/参数 + 拒绝原因（未填省略））
- 内部：`escapeMarkdown(s string) string`（反引号 → 反引号+零宽空格 `\u200b`，破坏围栏序列但视觉不变）、`truncateArgs(args string) string`（>512 字符截断加 `…`）、`mustJSON(v any) string`

**Steps:**

1. 先写失败测试 `approval_card_test.go`：
   - `TestBuildApprovalCardStructure`：`json.Unmarshal` 后断言——`schema=="2.0"`；`header.template=="red"`；body elements[0] 为 markdown 且 content 含 `task-t-1`/`bash`/args；elements[1] `tag=="hr"`；elements[2] 为 form（`name=="approval_form"`），其 elements 共 3 项：第一个 input（`name=="reject_reason"`、`required==false`），后两个 button 分别 `name=="approve_btn"/"reject_btn"`、`form_action_type=="submit"`、`behaviors[0].value.action=="approve"/"reject"`、`value.task_id=="task-t-1"`
   - `TestBuildApprovalCardNoActionModule`：序列化字符串**不含** `"tag":"action"`（v2 不支持旧模块）
   - `TestBuildApprovalCardTruncate`：args 长度 600 → markdown content 含截断后 512 字符 + `…`；短 args 原样
   - `TestBuildApprovalCardEscape`：args 含 `"````"`（三反引号）→ markdown content 中不出现连续三反引号
   - `TestBuildApprovalResultCard`：allowed→ header green + title 含 `✅ 已通过` + content 含 审批人 open_id/任务 ID/工具/参数；!allowed + reason → red + `❌ 已拒绝` + content 含 `拒绝原因` 与原因文本；!allowed + 空 reason → content **不含** `拒绝原因`
   - 运行 `go test ./internal/gateway/lark/ -run 'TestBuildApproval' -v` → 编译失败（文件不存在）
2. 实现 `internal/gateway/lark/approval_card.go`：

```go
package lark

import (
	"encoding/json"
	"fmt"
	"strings"
)

const maxArgsLen = 512

// BuildApprovalCard 构建审批请求卡（card JSON v2，header red）。
func BuildApprovalCard(taskID, toolName, args string) string {
	card := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"template": "red",
			"title":    map[string]any{"tag": "plain_text", "content": "⚠️ 高危操作审批请求"},
		},
		"body": map[string]any{
			"elements": []any{
				map[string]any{
					"tag": "markdown",
					"content": fmt.Sprintf("**Agent 请求执行以下操作：**\n**工具：** `%s`\n**参数：**\n```\n%s\n```\n**任务 ID：** `%s`",
						escapeMarkdown(toolName), escapeMarkdown(truncateArgs(args)), taskID),
				},
				map[string]any{"tag": "hr"},
				map[string]any{
					"tag":  "form",
					"name": "approval_form",
					"elements": []any{
						map[string]any{
							"tag":         "input",
							"name":        "reject_reason",
							"required":    false,
							"label":       map[string]any{"tag": "plain_text", "content": "拒绝原因（选填）"},
							"placeholder": map[string]any{"tag": "plain_text", "content": "仅拒绝时填写"},
						},
						buildSubmitButton("approve_btn", "primary", "✅ 通过", "approve", taskID),
						buildSubmitButton("reject_btn", "danger", "❌ 拒绝", "reject", taskID),
					},
				},
			},
		},
	}
	return mustJSON(card)
}

// buildSubmitButton 构建 form 内 submit 按钮：name + form_action_type + behaviors callback。
func buildSubmitButton(name, btnType, text, action, taskID string) map[string]any {
	return map[string]any{
		"tag":              "button",
		"name":             name,
		"form_action_type": "submit",
		"type":             btnType,
		"text":             map[string]any{"tag": "plain_text", "content": text},
		"behaviors": []any{map[string]any{
			"type":  "callback",
			"value": map[string]any{"action": action, "task_id": taskID},
		}},
	}
}

// BuildApprovalResultCard 构建结果卡：允许 → green 通过；拒绝 → red 已拒绝（追加原因，未填省略）。
func BuildApprovalResultCard(task approvalTask, allowed bool, reason, operatorOpenID string) string {
	template, title := "green", "✅ 已通过"
	if !allowed {
		template, title = "red", "❌ 已拒绝"
	}
	lines := []string{
		fmt.Sprintf("**审批人：** `%s`", escapeMarkdown(operatorOpenID)),
		fmt.Sprintf("**任务 ID：** `%s`", escapeMarkdown(task.taskID)),
		fmt.Sprintf("**工具：** `%s`", escapeMarkdown(task.toolName)),
		fmt.Sprintf("**参数：**\n```\n%s\n```", escapeMarkdown(truncateArgs(task.args))),
	}
	if !allowed && reason != "" {
		lines = append(lines, fmt.Sprintf("**拒绝原因：** %s", escapeMarkdown(reason)))
	}
	card := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"template": template,
			"title":    map[string]any{"tag": "plain_text", "content": title},
		},
		"body": map[string]any{
			"elements": []any{
				map[string]any{"tag": "markdown", "content": strings.Join(lines, "\n")},
			},
		},
	}
	return mustJSON(card)
}

// escapeMarkdown 反引号后接零宽空格：视觉不变，但破坏 ``` 围栏序列，防嵌入内容提前闭合代码块。
func escapeMarkdown(s string) string {
	return strings.ReplaceAll(s, "`", "`\u200b")
}

// truncateArgs 超长参数截断加省略号，避免卡片超限（消息 30KB 上限）。
func truncateArgs(args string) string {
	if len(args) <= maxArgsLen {
		return args
	}
	return args[:maxArgsLen] + "…"
}

// mustJSON 序列化卡片；结构固定，marshal 理论上不可能失败。失败返回空串（调用方 fail-closed）。
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
```

3. 运行 `go test ./internal/gateway/lark/ -run 'TestBuildApproval' -v` → 全绿

**Verification:** `gofmt -l internal/ cmd/` + `go build ./...` + `go test ./...`

---

### Task 3: 卡片发送 — `lark.go` SendCardMessage + `lark_reporter.go` 改卡片

**Files:**
- Modify: `internal/gateway/lark/lark.go`（新增 `SendCardMessage`）
- Modify: `internal/gateway/lark/lark_reporter.go`（`LarkReporter.bot` 类型抽象为接口、`SendApprovalMessage` 改发卡片）
- Test: `internal/gateway/lark/lark_reporter_test.go`（新建）

**Interfaces:**
- 产出：
  - `func (b *Bot) SendCardMessage(ctx context.Context, chatID, tenantKey, cardJSON string) error`（`MsgTypeInteractive`，content 直接为 card JSON 字符串，非二次编码）
  - `type messageSender interface { SendMessage(...); SendCardMessage(...) }`（放在 lark_reporter.go，测试注入 fake）
  - `LarkReporter.bot` 字段类型 `*Bot` → `messageSender`；`NewLarkReporter(bot *Bot, ...)` 签名不变（`*Bot` 满足接口）
  - `SendApprovalMessage` 改为：`BuildApprovalCard(...)` → `SendCardMessage`；空串（构建失败）返回 error
- 说明：`LarkReporter.send`（文本）继续走 `SendMessage`，互斥锁串行化保留

**Steps:**

1. 先写失败测试 `lark_reporter_test.go`：
   - fake 实现 `messageSender`（记录文本与卡片调用，可注入发送错误）：

```go
type fakeMessageSender struct {
	mu         sync.Mutex
	texts      []string
	cards      []string
	chatIDs    []string
	tenantKeys []string
	sendErr    error
}

func (f *fakeMessageSender) SendMessage(ctx context.Context, chatID, tenantKey, content string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.texts = append(f.texts, content)
	f.chatIDs = append(f.chatIDs, chatID)
	f.tenantKeys = append(f.tenantKeys, tenantKey)
	return f.sendErr
}

func (f *fakeMessageSender) SendCardMessage(ctx context.Context, chatID, tenantKey, cardJSON string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cards = append(f.cards, cardJSON)
	f.chatIDs = append(f.chatIDs, chatID)
	f.tenantKeys = append(f.tenantKeys, tenantKey)
	return f.sendErr
}
```

   - `TestSendApprovalMessageCard`：`newLarkReporter(fake, "chat1", "tk1")` 调 `SendApprovalMessage` → 断言 `cards[0]` 可被 json.Unmarshal、`schema=="2.0"`、含 form 容器；断言 `chatIDs[0]=="chat1"`、`tenantKeys[0]=="tk1"`（透传正确）
   - `TestSendApprovalMessageError`：fake 注入错误 → `SendApprovalMessage` 返回该错误
   - `TestReporterTextSendStillWorks`：`OnMessage(ctx, "hi")` 走 `SendMessage` → `texts[0]=="hi"`
   - 运行 `go test ./internal/gateway/lark/ -run 'TestSendApprovalMessage|TestReporterText' -v` → 编译失败（`messageSender`/`newLarkReporter` 未定义）
2. 实现：
   - `lark.go` 追加：

```go
// SendCardMessage 发送互动卡片消息（interactive）；cardJSON 为 card JSON v2 字符串。
func (b *Bot) SendCardMessage(ctx context.Context, chatID, tenantKey, cardJSON string) error {
	resp, err := b.cli.Im.Message.Create(
		ctx,
		larkim.NewCreateMessageReqBuilder().
			ReceiveIdType("chat_id").
			Body(larkim.NewCreateMessageReqBodyBuilder().
				MsgType(larkim.MsgTypeInteractive).
				ReceiveId(chatID).
				Content(cardJSON).
				Build()).
			Build(),
		larkcore.WithTenantKey(tenantKey),
	)
	if err != nil {
		return errors.WithStack(err)
	}
	if !resp.Success() {
		return errors.Errorf("send card message failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	slog.InfoContext(ctx, "card message sent", slog.String("chatID", chatID))
	return nil
}
```

   - `lark_reporter.go` 改写（保留 `send` 与各 `OnXxx`，仅替换 bot 类型、构造器、SendApprovalMessage）：

```go
package lark

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aethelgards/tiny-claw/internal/engine"
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

func NewLarkReporter(bot *Bot, chatID, tenantKey string) engine.Reporter {
	return newLarkReporter(bot, chatID, tenantKey)
}

// newLarkReporter 用接口注入（测试传 fake），NewLarkReporter 是生产入口。
func newLarkReporter(bot messageSender, chatID, tenantKey string) engine.Reporter {
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

func (l *LarkReporter) OnThinking(ctx context.Context) {
	_ = l.send(ctx, "🤔 思考中…")
}

func (l *LarkReporter) OnToolCall(ctx context.Context, toolName string, args string) {
	_ = l.send(ctx, "🔧 正在执行工具: "+toolName)
}

func (l *LarkReporter) OnToolResult(ctx context.Context, toolName string, result string, isError bool) {
	if isError {
		_ = l.send(ctx, "⚠️ 工具执行失败: "+toolName)
	}
}

func (l *LarkReporter) OnMessage(ctx context.Context, content string) {
	_ = l.send(ctx, content)
}

// SendApprovalMessage 发送审批卡片；构建失败返回错误（WaitingForApproval 据此 fail-closed）。
func (l *LarkReporter) SendApprovalMessage(ctx context.Context, taskID string, toolName string, args string) error {
	card := BuildApprovalCard(taskID, toolName, args)
	if card == "" {
		return fmt.Errorf("build approval card failed: taskID=%s", taskID)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.bot.SendCardMessage(ctx, l.chatID, l.tenantKey, card); err != nil {
		slog.ErrorContext(ctx, "lark reporter send approval card failed",
			slog.String("chatID", l.chatID), slog.String("err", err.Error()))
		return err
	}
	return nil
}
```

   - 检查 `lark.go` 现有 import 是否含 `larkcore`/`errors`（SendMessage 同款调用已有，若无则补）
3. 运行 `go test ./internal/gateway/lark/ -run 'TestSendApprovalMessage|TestReporterText' -v` → 全绿

**Verification:** `gofmt -l internal/ cmd/` + `go build ./...` + `go test ./...`

---

### Task 4: `approval_ctx.go` — 审批上下文 + EngineProcessor 注入

**Files:**
- Create: `internal/gateway/lark/approval_ctx.go`
- Modify: `internal/gateway/lark/engine_processor.go`
- Test: `internal/gateway/lark/approval_ctx_test.go`（新建）

**Interfaces:**
- 产出：
  - `func WithApprovalContext(ctx context.Context, reporter engine.Reporter, openID string) context.Context`
  - `func approvalContextFrom(ctx context.Context) (approvalContext, bool)`
  - `type approvalContext struct { reporter engine.Reporter; approverOpenID string }`
- `EngineProcessor.Process` 在创建 reporter 后注入 `ctx = WithApprovalContext(ctx, reporter, msg.OpenID)`（审批中间件需要 reporter 发卡片 + 发起人 open_id）

**Steps:**

1. 先写失败测试 `approval_ctx_test.go`：
   - `TestApprovalContextRoundtrip`：`WithApprovalContext(ctx, fakeReporter, "ou_x")` → `approvalContextFrom` 返回 `(ac, true)`，字段一致
   - `TestApprovalContextMissing`：裸 ctx → `!ok`
   - 运行 `go test ./internal/gateway/lark/ -run 'TestApprovalContext' -v` → 编译失败
2. 实现 `approval_ctx.go`：

```go
package lark

import (
	"context"

	"github.com/aethelgards/tiny-claw/internal/engine"
)

type approvalCtxKey struct{}

type approvalContext struct {
	reporter       engine.Reporter
	approverOpenID string
}

// WithApprovalContext 把审批所需上下文（reporter + 发起人 open_id）注入 ctx。
func WithApprovalContext(ctx context.Context, reporter engine.Reporter, openID string) context.Context {
	return context.WithValue(ctx, approvalCtxKey{}, approvalContext{
		reporter:       reporter,
		approverOpenID: openID,
	})
}

func approvalContextFrom(ctx context.Context) (approvalContext, bool) {
	ac, ok := ctx.Value(approvalCtxKey{}).(approvalContext)
	return ac, ok
}
```

   - `engine_processor.go` 的 `Process` 中 reporter 创建之后、agent 创建之前插入一行（现有第 38 行与第 39 行之间）：

```go
	reporter := NewLarkReporter(p.bot, msg.ChatID, msg.TenantKey)
	// 注入审批上下文：审批中间件通过 ctx 取 reporter（发卡片）与发起人 open_id（身份校验）
	ctx = WithApprovalContext(ctx, reporter, msg.OpenID)
	agent := engine.NewAgentEngine(p.provider, p.registry, p.settings, reporter, p.promptComposer, ctxpkg.NewRecoveryManager(), engine.NewReminderInjector(3))
```

3. 运行 `go test ./internal/gateway/lark/ -run 'TestApprovalContext' -v` → 全绿

**Verification:** `gofmt -l internal/ cmd/` + `go build ./...` + `go test ./...`

---

### Task 5: `approval_middleware.go` — 审批中间件 + 注册表集成

**Files:**
- Create: `internal/gateway/lark/approval_middleware.go`
- Test: `internal/gateway/lark/approval_middleware_test.go`（新建）

**Interfaces:**
- 产出：`func ApprovalMiddleware(mgr *ApprovalManager) tools.MiddlewareFunc`
- 行为：
  - 非危险命令（`!isDangerousCommand`）→ 直接 `(true, nil)`，不触碰 manager、不需要 ctx
  - 危险命令 + 无审批上下文 → `(false, error("缺少审批上下文，无法执行高危操作"))`（fail-closed 兜底）
  - 危险命令 + 有 ctx → `mgr.WaitingForApproval(ctx, call.ID, call.Name, string(call.Arguments), ac.reporter, ac.approverOpenID)`；`allowed=false` → `(false, errors.New(reason))`

**Steps:**

1. 先写失败测试 `approval_middleware_test.go`（复用 `fakeApprovalReporter`、`waitFor`；额外定义 fake 工具）：

```go
// fakeBashTool 供注册表集成测试用：名称命中危险命令判定，执行返回 ok。
type fakeBashTool struct{}

func (fakeBashTool) Name() string { return "bash" }
func (fakeBashTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{Name: "bash", Description: "fake bash"}
}
func (fakeBashTool) Execute(ctx context.Context, args json.RawMessage) (string, error) { return "ok", nil }
```

   - `TestMiddlewarePassNonDangerous`：裸 ctx + `("read_file", …)` → `(true, nil)`
   - `TestMiddlewareDenyNoCtx`：裸 ctx + `("bash", "rm -rf /")` → `(false, err)`，err 含 `"缺少审批上下文"`
   - `TestMiddlewareApprove`：`WithApprovalContext(ctx, fakeReporter, "ou_approver")`；goroutine 里调 middleware（bash 危险命令），`waitFor` `mgr.getTask` 出现后 `ResolveApproval(ctx, taskID, true, "")` → middleware 返回 `(true, nil)`，任务已清理
   - `TestMiddlewareReject`：同上但 `ResolveApproval(false,"原因Y")` → `(false, err 含"原因Y")`
   - `TestRegistryIntegrationIntercepted`：`reg := tools.NewToolRegistry()`；`reg.Registry(fakeBashTool{})`；`reg.Use(ApprovalMiddleware(mgr))`；注入 ctx；goroutine 里 `reg.Execute(ctx, schema.ToolCall{ID:"call-r1", Name:"bash", Arguments: json.RawMessage(`{"command":"rm -rf /tmp"}`)})`；`waitFor getTask` 后 `ResolveApproval(false,"")` → 结果 `IsError==true` 且 `Output` 含 `"执行被系统拦截"`（registry 对拒绝中间件的固定文案，已核实 `fmt.Sprintf("执行被系统拦截。原因: %s", rejectReason)`）
   - 运行 `go test ./internal/gateway/lark/ -run 'TestMiddleware|TestRegistry' -v` → 编译失败
2. 实现 `approval_middleware.go`：

```go
package lark

import (
	"context"
	"errors"

	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/aethelgards/tiny-claw/internal/tools"
)

// ApprovalMiddleware 危险命令审批中间件：命中高危模式必须经发起人卡片审批。
// 非危险命令零开销放行；危险命令缺审批上下文 → 拒绝（fail-closed 兜底）。
func ApprovalMiddleware(mgr *ApprovalManager) tools.MiddlewareFunc {
	return func(ctx context.Context, call schema.ToolCall) (bool, error) {
		if !isDangerousCommand(call.Name, string(call.Arguments)) {
			return true, nil
		}
		ac, ok := approvalContextFrom(ctx)
		if !ok {
			return false, errors.New("缺少审批上下文，无法执行高危操作")
		}
		allowed, reason := mgr.WaitingForApproval(ctx, call.ID, call.Name, string(call.Arguments), ac.reporter, ac.approverOpenID)
		if !allowed {
			return false, errors.New(reason)
		}
		return true, nil
	}
}
```

3. 运行 `go test ./internal/gateway/lark/ -run 'TestMiddleware|TestRegistry' -v` → 全绿

**Verification:** `gofmt -l internal/ cmd/` + `go build ./...` + `go test ./...`

---

### Task 6: `approval_handler.go` — 卡片回调处理

**Files:**
- Create: `internal/gateway/lark/approval_handler.go`
- Test: `internal/gateway/lark/approval_handler_test.go`（新建）

**Interfaces:**
- 产出：`func NewApprovalCardHandler(mgr *ApprovalManager) func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error)`
- 流程（对上位设计 §4.6）：
  1. nil 防护：`event` / `event.Event` / `Action` / `Operator` 任一 nil → `Toast{type:"error", content:"回调数据缺失"}`
  2. 解析：`Action.Name`（`approve_btn`/`reject_btn`）为主；`Action.Value["action"]` 交叉校验（若存在且与 name 不符 → 未知操作）；`Value["task_id"]` 取 taskID
  3. 任务存在性：`getTask(taskID)` 失败 → `Toast{type:"warning", content:"该请求已处理或不存在"}`，不更新卡片
  4. 身份校验：`Operator.OpenID != task.approverOpenID` → `Toast{type:"error", content:"只有发起请求的用户才能审批"}`，不更新卡片（结果卡不暴露给无关人）
  5. 拒绝原因：reject 分支从 `Action.FormValue["reject_reason"]` 取（`interface{}` 断言 string，空视为未填）；approve 分支 reason 恒空
  6. 投递：`mgr.ResolveApproval(ctx, taskID, approve, reason)`；返回 false（竞态/已处理）→ 同步骤 3 的 warning toast
  7. 响应：`CardActionTriggerResponse{Toast{success, "已通过"/"已拒绝"}, Card{Type:"raw", Data: BuildApprovalResultCard(...)}}`，原子原位更新
- 边界：发起人 `OpenID` 可能为空（sender 无 open_id）→ `approverOpenID==""` 时任何非空 operator 都无法通过身份校验，最终超时自动拒绝——fail-safe，无需特殊处理

**Steps:**

1. 先写失败测试 `approval_handler_test.go`：
   - 辅助函数：

```go
func buildActionEvent(openID, taskID, name, action string, formValue map[string]any) *callback.CardActionTriggerEvent {
	return &callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Operator: &callback.Operator{OpenID: openID},
			Action: &callback.CallBackAction{
				Name:      name,
				Value:     map[string]any{"action": action, "task_id": taskID},
				FormValue: formValue,
			},
		},
	}
}
```

   - 每例模式：goroutine 里 `WaitingForApproval` 注册任务 → `waitFor` `getTask` 出现 → 调 handler → 断言响应 → 从 goroutine 收结果；**用例末尾若任务仍挂起，用 `ResolveApproval` 释放阻塞 goroutine（防泄漏）**
   - `TestHandlerApprove`：approve_btn/approve → Toast success `"已通过"`、`Card.Type=="raw"`、`Card.Data` 含 `✅ 已通过`；WaitingForApproval 返回 `(true,"")`；任务已清理
   - `TestHandlerRejectWithReason`：reject_btn/reject + `FormValue{"reject_reason": "不想放"}`（`map[string]any` 直接放 string）→ Toast success `"已拒绝"`、Data 含 `❌ 已拒绝` 与 `不想放`；Waiting 返回 `(false,"不想放")`
   - `TestHandlerRejectNoReason`：reject + 空 FormValue → Data 不含 `拒绝原因`；Waiting 返回 `(false,"")`
   - `TestHandlerNotApprover`：operator OpenID != approverOpenID → Toast error 含 `"只有发起请求的用户才能审批"`、`resp.Card == nil`；随后 `ResolveApproval` 释放 goroutine（此时任务应仍在 → 返回 true）
   - `TestHandlerTaskNotFound`：任意 openID + 未知 taskID → Toast warning 含 `"该请求已处理或不存在"`、`resp.Card == nil`（无挂起任务，无需释放）
   - `TestHandlerActionMismatch`：`Action.Name=="approve_btn"` 但 `Value["action"]=="reject"` → Toast error 含 `"未知操作"`、Card nil（无挂起任务，无需释放）
   - 运行 `go test ./internal/gateway/lark/ -run 'TestHandler' -v` → 编译失败
2. 实现 `approval_handler.go`：

```go
package lark

import (
	"context"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

// NewApprovalCardHandler 处理卡片审批回调（card.action.trigger）：
// 校验任务存在与发起人身份 → ResolveApproval 投递 → 原位更新结果卡（原子更新）。
func NewApprovalCardHandler(mgr *ApprovalManager) func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	return func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
		req := event.Event
		if event == nil || req == nil || req.Action == nil || req.Operator == nil {
			return errorToast("回调数据缺失"), nil
		}

		// 按钮身份：Action.Name 为主，Value["action"] 交叉校验
		actionName, _ := req.Action.Value["action"].(string)
		var approve bool
		switch req.Action.Name {
		case "approve_btn":
			if actionName != "" && actionName != "approve" {
				return errorToast("未知操作"), nil
			}
			approve = true
		case "reject_btn":
			if actionName != "" && actionName != "reject" {
				return errorToast("未知操作"), nil
			}
			approve = false
		default:
			return errorToast("未知操作"), nil
		}

		taskID, _ := req.Action.Value["task_id"].(string)
		if taskID == "" {
			return errorToast("任务 ID 缺失"), nil
		}

		// 任务存在性
		task, ok := mgr.getTask(taskID)
		if !ok {
			return warningToast("该请求已处理或不存在"), nil
		}

		// 身份校验：只有发起请求的用户能审批（open_id 为空时任何非空 operator 都过不了 → 超时拒绝，fail-safe）
		if req.Operator.OpenID != task.approverOpenID {
			return errorToast("只有发起请求的用户才能审批"), nil
		}

		// 拒绝原因（可选；approve 恒空）
		reason := ""
		if !approve {
			if v, ok := req.Action.FormValue["reject_reason"].(string); ok {
				reason = v
			}
		}

		// 投递（竞态/已处理 → false → warning）
		if !mgr.ResolveApproval(ctx, taskID, approve, reason) {
			return warningToast("该请求已处理或不存在"), nil
		}

		toastContent := "已通过"
		if !approve {
			toastContent = "已拒绝"
		}
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{Type: "success", Content: toastContent},
			Card: &callback.Card{
				Type: "raw",
				Data: BuildApprovalResultCard(task, approve, reason, req.Operator.OpenID),
			},
		}, nil
	}
}

func errorToast(content string) *callback.CardActionTriggerResponse {
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{Type: "error", Content: content},
	}
}

func warningToast(content string) *callback.CardActionTriggerResponse {
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{Type: "warning", Content: content},
	}
}
```

3. 运行 `go test ./internal/gateway/lark/ -run 'TestHandler' -v` → 全绿

**Verification:** `gofmt -l internal/ cmd/` + `go build ./...` + `go test ./...`

---

### Task 7: `config.go` — ApprovalTimeout 配置项

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`（追加）

**Interfaces:**
- `Settings` 追加：`ApprovalTimeout string \`json:"approvalTimeout"\``（Go duration 字符串，默认 `"5m"`，`<=0` 视为默认）
- `defaultSettings()`：`ApprovalTimeout: "5m"`
- `applyLayer`：键 `"approvalTimeout"` 覆盖（`json.Unmarshal` 进 string）
- `applyEnv`：`CLAW_APPROVAL_TIMEOUT`（非空则覆盖；环境变量命名沿用现有 `CLAW_LARK_*` 蛇形惯例）
- 配置层存字符串不解析；非法值回退默认由 `lark.ParseApprovalTimeout`（Task 1）处理

**Steps:**

1. 先追加失败测试（复用 config_test.go 现有 helper，如 `mustWrite`/`applyLayer` 直调）：
   - `TestDefaultApprovalTimeout`：`defaultSettings().ApprovalTimeout == "5m"`
   - `TestApplyLayerApprovalTimeout`：`applyLayer(&s, []byte("{\"approvalTimeout\":\"10m\"}"))` → `"10m"`
   - `TestApplyEnvApprovalTimeout`：`t.Setenv("CLAW_APPROVAL_TIMEOUT", "2m")` + `applyEnv(&s)` → `"2m"`
   - `TestLoadKeepsApprovalTimeout`：`loadFrom` 三层合并后字段保留（复用现有 mustWrite 模式）
   - 运行 `go test ./internal/config/ -run 'TestApprovalTimeout|TestApplyLayerApprovalTimeout|TestApplyEnvApprovalTimeout' -v` → 编译失败（字段未定义）
2. 实现：
   - `Settings` 结构体追加字段（Lark 配置块内，`LarkChannelSize` 之后）：

```go
	// Lark 机器人模式配置（仅 cmd/larkbot 使用，CLI 模式可缺省）
	LarkAppID       string `json:"larkAppId"`
	LarkAppSecret   string `json:"larkAppSecret"`
	LarkChannelSize int    `json:"larkChannelSize"` // 消息队列容量，默认 64
	ApprovalTimeout string `json:"approvalTimeout"` // 审批超时（Go duration 字符串），默认 "5m"，<=0 视为默认
```

   - `defaultSettings()` 追加 `ApprovalTimeout: "5m",`
   - `applyLayer` 内追加（与其他键同一模式）：

```go
	if v, ok := layer["approvalTimeout"]; ok {
		if err := json.Unmarshal(v, &s.ApprovalTimeout); err != nil {
			return err
		}
	}
```

   - `applyEnv` 内追加：

```go
	if v := os.Getenv("CLAW_APPROVAL_TIMEOUT"); v != "" {
		s.ApprovalTimeout = v
	}
```

3. 运行 `go test ./internal/config/ -run 'TestApprovalTimeout|TestApplyLayerApprovalTimeout|TestApplyEnvApprovalTimeout|TestLoad' -v` → 全绿

**Verification:** `gofmt -l internal/ cmd/` + `go build ./...` + `go test ./...`

---

### Task 8: `cmd/larkbot/main.go` — 接线（中间件 + 卡片回调注册）

**Files:**
- Modify: `cmd/larkbot/main.go`
- 无新测试（package main 不做单测；验证为 build + 全量测试）

**Interfaces:**
- 装配：`reg.Use(lark.ApprovalMiddleware(approvalMgr))`
- 注册：`d.OnP2CardActionTrigger(lark.NewApprovalCardHandler(approvalMgr))`
- 超时解析：`lark.ParseApprovalTimeout(settings.ApprovalTimeout)`（非法值自动回退默认）
- CLI（`cmd/claw/main.go`）**不改**：CLI 不注册审批中间件

**Steps:**

1. 在工具/技能注册之后（现有第 58 行 `RegisterSkills` 之后）、`queue := lark.NewMessageQueue(...)`（第 60 行）之前插入：

```go
	// 审批：创建审批管理器并接入注册表中间件（危险命令经卡片审批；非法超时回退默认 5m）
	approvalMgr := lark.NewApprovalManager(lark.ParseApprovalTimeout(settings.ApprovalTimeout))
	reg.Use(lark.ApprovalMiddleware(approvalMgr))
```

2. 在 `bot.Start(ctx, func(d *dispatcher.EventDispatcher) { ... })` 的 register 回调内、现有 `d.OnP2MessageReceiveV1(...)` 之后追加：

```go
		d.OnP2CardActionTrigger(lark.NewApprovalCardHandler(approvalMgr))
```

3. `approvalMgr` 在 `bot.Start` 之前声明（第 61 行 `bot := lark.NewBot(...)` 附近），register 闭包内可直接引用，作用域满足。

**Verification:** `gofmt -l internal/ cmd/` + `go build ./...` + `go test ./...`（全绿）
- 人工核对：`go vet ./...` 无告警；`cmd/claw/main.go` 未改动
- 冒烟（可选，需真实凭据）：配置 larkAppId/larkAppSecret 后跑 `go run ./cmd/larkbot`，触发 `bash` 危险命令（如 `rm -rf /tmp/xxx`）应收到审批卡片，点通过/拒绝后卡片变结果卡

---

## 自检清单（向设计稿对齐）

- [ ] §4.1 approvalTask/NewApprovalManager/WaitingForApproval/ResolveApproval/getTask 全部落地（多 taskID 字段——结果卡必需，见 Task 1 说明）
- [ ] §4.2 approval_ctx.go + EngineProcessor 注入
- [ ] §4.3 ApprovalMiddleware 接入 reg.Use
- [ ] §4.4 SendCardMessage（MsgTypeInteractive）+ SendApprovalMessage 改卡片
- [ ] §4.5 BuildApprovalCard/BuildApprovalResultCard：form 容器、form_action_type、required:false、转义/截断、v2 无 tag:action
- [ ] §4.6 NewApprovalCardHandler：Name 主身份 + Value 交叉校验 + FormValue 取原因 + 身份校验 + 原位更新（Message.Patch 兜底不在本期，见 Global Constraints）
- [ ] §4.7 Settings.ApprovalTimeout + CLAW_APPROVAL_TIMEOUT + ParseApprovalTimeout
- [ ] §5 main.go 接线：reg.Use + OnP2CardActionTrigger；CLI 不动
- [ ] §7 测试：manager 注册/投递/超时/取消/fail-closed/nil-reporter/重复投递、中间件放行/拒绝/通过、isDangerousCommand 矩阵、卡片构建、卡片回调、配置、注册表集成
- [ ] P1 问题全消：P1 卡片通知 ✅ / P2 身份校验 ✅ / P3 超时 ✅ / P4 fail-closed ✅ / P5 拒绝原因 ✅ / P6 接线 ✅
