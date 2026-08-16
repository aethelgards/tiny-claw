# 终端模式（cmd/claw）审批适配 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 Lark 审批核心（`ApprovalManager`/`ApprovalMiddleware`/`WithApprovalContext`/`isDangerousCommand`，零 lark 依赖）抽离为共享包 `internal/approval`，新增终端交互式 reporter（y/N 读 stdin），`cmd/claw` 接线（危险命令本地审批、非 TTY 直接拒绝），`cmd/larkbot` 改为引用共享包（行为零变化）。验证：`gofmt -l internal/ cmd/` + `go build ./...` + `go vet ./...` + `go test ./...` 全绿，`go list -deps ./cmd/claw | grep larksuite` 为空。

**Architecture:**

```
cmd/claw/main.go
 ├─ approval.NewApprovalManager(ParseApprovalTimeout(settings.ApprovalTimeout))
 ├─ reg.Use(approval.ApprovalMiddleware(mgr))
 ├─ reporter := approval.NewTerminalReporter(mgr)      // 实现 engine.Reporter
 ├─ agent := engine.NewAgentEngine(..., reporter, ...)
 └─ runCtx := approval.WithApprovalContext(context.Background(), reporter, "local")
    └─ agent.Run(runCtx, prompt)
       └─ loop ──▶ registry.Execute(ctx, call) ──▶ ApprovalMiddleware
                      ├─ isDangerousCommand? 否 → 放行
                      └─ 是 → WaitingForApproval(ctx, call.ID, name, args, reporter, "local")
                             ├─ SendApprovalMessage ──▶ 打印详情 + (y/N) 读 stdin
                             │                          ├─ y/yes → ResolveApproval(true, "")
                             │                          ├─ 其余 → ResolveApproval(false, "终端用户拒绝")
                             │                          ├─ EOF/读失败 → ResolveApproval(false, "终端输入读取失败")
                             │                          └─ 非 TTY → ResolveApproval(false, "非交互终端，自动拒绝")
                             └─ select{ 结果 | 超时 | ctx.Done } → 放行或 "执行被系统拦截。原因: …"
```

包依赖（单向）：

```
internal/engine ──▶（无新依赖）
internal/approval ──▶ engine, tools, schema
internal/gateway/lark ──▶ approval, engine, tools, larksuite SDK
cmd/claw ──▶ approval, engine, tools, provider, config, context
cmd/larkbot ──▶ approval, lark, engine, tools, provider, config, context
```

**Tech Stack:** Go 1.26.3；不新增依赖。复用：`tools.MiddlewareFunc`、`engine.Reporter`、`engine.NewTerminalReporter`（转发基类）。

## Global Constraints

- **不引入新依赖**；审批核心仅 import `engine`/`tools`/`schema`（与现状一致，`internal/approval` 零 lark 依赖）
- **验证门（沿用 2026-08-08-config-system.md 约定）**：`gofmt -l internal/ cmd/` 空输出 + `go build ./...` 通过 + `go vet ./...` 通过 + `go test ./...` 全绿；CLI 无 lark 依赖检查：`go list -deps ./cmd/claw | grep larksuite` 为空
- **不执行 git commit**（仓库存在大量未提交改动；本计划仅改文件、不提交）
- **引擎层（internal/engine）零改动**；`internal/config` 零改动；`cmd/larkbot` 行为零变化（仅换 import 路径）
- **TDD 纪律**：每个 Task 先写/迁移测试 → 跑红（编译失败或断言失败）→ 实现 → 跑绿
- **随迁测试的 `waitFor` 问题**：`worker_test.go` 的 `waitFor` 属 package lark，`internal/approval` 跨包不可复用 → 随迁测试文件需自带本地 `waitFor` 副本（从 `worker_test.go:44-54` 原样拷贝）；lark 的 `worker_test.go` 不动，lark 内 `waitFor` 继续供 `approval_handler_test.go` 使用
- **`fakeApprovalReporter` 问题**：随 `approval_test.go` 迁走，package lark 失去该类型 → `approval_handler_test.go`（留 lark）需在文件内定义最小本地 fake（仅 `SendApprovalMessage` 返回 nil）
- **`Task` 结构体 `ch` 字段保持未导出**：Go 允许跨包 composite literal 只写导出字段 → lark 的 `approval_card_test.go` 用 `approval.Task{TaskID:…, ToolName:…, Args:…}` 合法（不写 `ch`）
- **终端 reporter 用持久 `bufio.Reader`**（构造时创建一次，字段持有）：避免每次 `bufio.NewReader(os.Stdin)` 过度缓冲吞掉后续输入的 bug（交互连续审批场景）
- **`NewTerminalReporter` 需要 `*ApprovalManager`**（C3）：`SendApprovalMessage` 钩子内完成交互 + `ResolveApproval`；注册任务先于钩子执行，`ResolveApproval` 必命中
- **审批者身份固定 `"local"`**（终端无多用户概念，设计稿 §4.4/§8.4）
- 现有误导文案 `engine.TerminalReporter.SendApprovalMessage`（"请回复 approve <taskID>"）保留不动（设计稿 §8.1：CLI 改用新 reporter 后该路径不再被主流程使用，清理属后续可选）

---

### Task 1: 抽包 `internal/approval`（核心平移 + 导出 `Task`/`GetTask` + 语义重命名 `approverOpenID`→`approverID`）

**Files:**
- Create: `internal/approval/approval.go`（从 `lark/approval.go` 平移 + 重命名）
- Create: `internal/approval/approval_ctx.go`（从 `lark/approval_ctx.go` 平移 + 重命名）
- Create: `internal/approval/approval_middleware.go`（从 `lark/approval_middleware.go` 平移）
- Create: `internal/approval/approval_test.go`（从 `lark/approval_test.go` 迁移 + 自带 `waitFor`）
- Create: `internal/approval/approval_ctx_test.go`（从 `lark/approval_ctx_test.go` 迁移）
- Create: `internal/approval/approval_middleware_test.go`（从 `lark/approval_middleware_test.go` 迁移）
- Delete: `internal/gateway/lark/approval.go`, `approval_ctx.go`, `approval_middleware.go`（已平移，删原件）
- Modify: `internal/gateway/lark/approval_handler.go`（引用 `approval.ApprovalManager`/`GetTask`/`ApproverID`）
- Modify: `internal/gateway/lark/approval_card.go`（`BuildApprovalResultCard` 签名改用 `approval.Task`）
- Modify: `internal/gateway/lark/engine_processor.go`（`approval.WithApprovalContext`）
- Modify: `internal/gateway/lark/approval_card_test.go`（`approval.Task` literal + import）
- Modify: `internal/gateway/lark/approval_handler_test.go`（`approval.NewApprovalManager`/`GetTask` + 本地 fake reporter + import）
- Untouched: `internal/gateway/lark/worker_test.go`, `lark_reporter.go`, `lark_reporter_test.go`

**Interfaces**（`internal/approval` 导出面，见设计稿 §4.1）:
- `type ApprovalResult struct { Allowed bool; RejectReason string }`
- `type Task struct { ch chan ApprovalResult; TaskID, ApproverID, ToolName, Args string }`（`ch` 未导出，仅包内投递）
- `type ApprovalManager struct{…}`；`func NewApprovalManager(timeout time.Duration) *ApprovalManager`
- `func (m *ApprovalManager) WaitingForApproval(ctx, taskID, toolName, args string, reporter engine.Reporter, approverID string) (bool, string)`
- `func (m *ApprovalManager) ResolveApproval(ctx context.Context, taskID string, allowed bool, reason string) bool`
- `func (m *ApprovalManager) GetTask(taskID string) (Task, bool)`（原 `getTask` 导出）
- `func ParseApprovalTimeout(raw string) time.Duration`
- `func isDangerousCommand(toolName, args string) bool`（未导出，语义不变）
- `func WithApprovalContext(ctx context.Context, reporter engine.Reporter, approverID string) context.Context`
- `func ApprovalMiddleware(mgr *ApprovalManager) tools.MiddlewareFunc`
- `approvalContext`/`approvalContextFrom`/`approvalCtxKey`（未导出，包内使用）

**Steps:**

1. 写随迁测试（先红）：把 lark 的 `approval_test.go` / `approval_ctx_test.go` / `approval_middleware_test.go` 复制到 `internal/approval/` 同名文件，应用以下机械替换：
   - `package lark` → `package approval`
   - `mgr.getTask(` → `mgr.GetTask(`（全部出现处，如 `waitFor` 条件与清理断言）
   - `ac.approverOpenID` → `ac.approverID`（`approval_ctx_test.go:15,16`）
   - 在 `approval_test.go` 顶部新增本地 `waitFor`（原样拷贝 `worker_test.go:44-54`）：

```go
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("等待条件超时")
}
```

   - 运行 `go test ./internal/approval/` → **红**（`no non-test Go files` 或 `undefined: NewApprovalManager` 等）
2. 写 `internal/approval/approval.go`（完整内容，含重命名；`ch` 保持未导出）：

```go
package approval

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

// Task 单个待审批任务：结果 channel（未导出，仅包内投递）+ 元数据（结果卡展示/身份校验用）。
type Task struct {
	ch         chan ApprovalResult
	TaskID     string
	ApproverID string // 谁发的请求，只有 TA 能批
	ToolName   string
	Args       string
}

type ApprovalManager struct {
	mu           sync.RWMutex
	pendingTasks map[string]Task
	timeout      time.Duration
}

// NewApprovalManager 创建审批管理器；timeout <= 0 视为默认 5 分钟。
func NewApprovalManager(timeout time.Duration) *ApprovalManager {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &ApprovalManager{
		pendingTasks: make(map[string]Task),
		timeout:      timeout,
	}
}

// WaitingForApproval 注册审批任务并阻塞等待结论。
// 返回 (allowed, reason)；reason 直接作为中间件拒绝文案。
// 所有出口（成功/发送失败/超时/ctx 取消）都会清理任务。
func (m *ApprovalManager) WaitingForApproval(ctx context.Context, taskID, toolName, args string, reporter engine.Reporter, approverID string) (bool, string) {
	task := Task{
		ch:         make(chan ApprovalResult, 1),
		TaskID:     taskID,
		ApproverID: approverID,
		ToolName:   toolName,
		Args:       args,
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

// GetTask 供回调处理读取任务元数据（身份校验 + 结果卡展示）。
func (m *ApprovalManager) GetTask(taskID string) (Task, bool) {
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

3. 写 `internal/approval/approval_ctx.go`（完整内容，含重命名）：

```go
package approval

import (
	"context"

	"github.com/aethelgards/tiny-claw/internal/engine"
)

type approvalCtxKey struct{}

type approvalContext struct {
	reporter   engine.Reporter
	approverID string
}

// WithApprovalContext 把审批所需上下文（reporter + 发起人 ID）注入 ctx。
func WithApprovalContext(ctx context.Context, reporter engine.Reporter, approverID string) context.Context {
	return context.WithValue(ctx, approvalCtxKey{}, approvalContext{
		reporter:   reporter,
		approverID: approverID,
	})
}

func approvalContextFrom(ctx context.Context) (approvalContext, bool) {
	ac, ok := ctx.Value(approvalCtxKey{}).(approvalContext)
	return ac, ok
}
```

4. 写 `internal/approval/approval_middleware.go`（完整内容）：

```go
package approval

import (
	"context"
	"errors"

	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/aethelgards/tiny-claw/internal/tools"
)

// ApprovalMiddleware 危险命令审批中间件：命中高危模式必须经发起人审批。
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
		allowed, reason := mgr.WaitingForApproval(ctx, call.ID, call.Name, string(call.Arguments), ac.reporter, ac.approverID)
		if !allowed {
			return false, errors.New(reason)
		}
		return true, nil
	}
}
```

5. 运行 `go test ./internal/approval/` → **绿**（随迁测试全部通过）
6. 改 `internal/gateway/lark/approval_handler.go`（lark 侧引用共享包）：
   - import 增加 `"github.com/aethelgards/tiny-claw/internal/approval"`
   - 签名 `func NewApprovalCardHandler(mgr *approval.ApprovalManager) …`（原 `*ApprovalManager`）
   - `task, ok := mgr.getTask(taskID)` → `task, ok := mgr.GetTask(taskID)`
   - `req.Operator.OpenID != task.approverOpenID` → `req.Operator.OpenID != task.ApproverID`
   - 其余（`errorToast`/`warningToast`/按钮判定/`ResolveApproval`/结果卡）不动
7. 改 `internal/gateway/lark/approval_card.go`：
   - import 增加 `"github.com/aethelgards/tiny-claw/internal/approval"`
   - `func BuildApprovalResultCard(task approval.Task, allowed bool, reason, operatorOpenID string) string`（原 `approvalTask`）
   - 函数体字段访问：`task.taskID`→`task.TaskID`、`task.toolName`→`task.ToolName`、`task.args`→`task.Args`（共 3 处，在 markdown 拼接中）
   - `BuildApprovalCard`/`buildSubmitButton`/`escapeMarkdown`/`truncateArgs`/`mustJSON`/`maxArgsLen` 不动
8. 改 `internal/gateway/lark/engine_processor.go`：
   - import 增加 `"github.com/aethelgards/tiny-claw/internal/approval"`
   - `ctx = WithApprovalContext(ctx, reporter, msg.OpenID)` → `ctx = approval.WithApprovalContext(ctx, reporter, msg.OpenID)`
   - 注释同步：`// 注入审批上下文：…发起人 open_id` → `…发起人 ID`
9. 改 `internal/gateway/lark/approval_card_test.go`：
   - import 增加 `"github.com/aethelgards/tiny-claw/internal/approval"`
   - `TestBuildApprovalResultCard` 内 `task := approvalTask{…}` → `task := approval.Task{TaskID: "task-t-1", ToolName: "bash", Args: "rm -rf /"}`（不写 `ch` 字段，合法）
   - 其余断言不变
10. 改 `internal/gateway/lark/approval_handler_test.go`：
    - import 增加 `"github.com/aethelgards/tiny-claw/internal/approval"` 与 `"github.com/aethelgards/tiny-claw/internal/engine"`（本地 fake 需要）
    - `NewApprovalManager(` → `approval.NewApprovalManager(`（全部 6 处）
    - `mgr.getTask(` → `mgr.GetTask(`（全部 5 处，在 `waitFor` 条件中）
    - 文件内新增最小本地 fake（原 `fakeApprovalReporter` 已随迁，lark 包不再有该类型）：

```go
// fakeReporter 仅供审批 handler 测试满足 WaitingForApproval 的 reporter 参数。
type fakeReporter struct{}

func (fakeReporter) OnThinking(ctx context.Context)                     {}
func (fakeReporter) OnToolCall(ctx context.Context, n string, a string) {}
func (fakeReporter) OnToolResult(ctx context.Context, n string, r string, isErr bool) {
}
func (fakeReporter) OnMessage(ctx context.Context, content string) {}
func (fakeReporter) SendApprovalMessage(ctx context.Context, taskID, toolName, args string) error {
	return nil
}
```

    - `rep := &fakeApprovalReporter{}` → `rep := &fakeReporter{}`（全部 4 处：TestHandlerApprove/RejectWithReason/RejectNoReason/NotApprover）
    - 其余断言不动
11. 删除 lark 原件：`internal/gateway/lark/approval.go`、`approval_ctx.go`、`approval_middleware.go`
12. 全量验证门：`gofmt -l internal/ cmd/`（空）+ `go build ./...` + `go vet ./...` + `go test ./...` 全绿

**Verification:** Task 1 完成后，`go test ./internal/approval/` 与 `go test ./internal/gateway/lark/` 各自通过；`grep -r "larksuite" internal/approval/` 无输出（核心包零 lark 依赖）；`go list -deps ./cmd/claw` 尚无 lark（Task 3 再查）。

---

### Task 2: `terminal_reporter.go` + `terminal_reporter_test.go`（新增，TDD）

**Files:**
- Create: `internal/approval/terminal_reporter.go`
- Create: `internal/approval/terminal_reporter_test.go`

**Interfaces**（设计稿 §4.2）:
- `type TerminalReporter struct { base engine.Reporter; mgr *ApprovalManager; reader *bufio.Reader; interactive bool }`
- `func NewTerminalReporter(mgr *ApprovalManager) engine.Reporter`（stdin 非字符设备 → `interactive=false`）
- `func newTerminalReporterForTest(mgr *ApprovalManager, in io.Reader, interactive bool) engine.Reporter`（未导出，测试注入）
- 方法：`OnThinking`/`OnToolCall`/`OnToolResult`/`OnMessage` 转发 `base`；`SendApprovalMessage` 覆盖为 y/N 交互

**Steps:**

1. 写失败测试 `internal/approval/terminal_reporter_test.go`：

```go
package approval

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aethelgards/tiny-claw/internal/engine"
)

// neverRead 永不返回的 reader：模拟用户不输入（超时路径）。
type neverRead struct{}

func (neverRead) Read(p []byte) (int, error) {
	<-make(chan struct{})
	return 0, io.EOF
}

// runWaitingForApproval 在 goroutine 里跑 WaitingForApproval（终端 reporter 在钩子内自行 ResolveApproval），
// 带 2s 兜底超时，返回结论。
func runWaitingForApproval(t *testing.T, mgr *ApprovalManager, rep engine.Reporter) (bool, string) {
	t.Helper()
	got := make(chan struct {
		allowed bool
		reason  string
	}, 1)
	go func() {
		allowed, reason := mgr.WaitingForApproval(context.Background(), "task-1", "bash", "rm -rf /", rep, "local")
		got <- struct {
			allowed bool
			reason  string
		}{allowed, reason}
	}()
	select {
	case r := <-got:
		return r.allowed, r.reason
	case <-time.After(2 * time.Second):
		t.Fatal("WaitingForApproval 未在兜底超时内返回")
		return false, ""
	}
}

func TestTerminalReporterApprove(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := newTerminalReporterForTest(mgr, strings.NewReader("y\n"), true)
	allowed, reason := runWaitingForApproval(t, mgr, rep)
	if !allowed || reason != "" {
		t.Fatalf("y 应放行，得到 (%v, %q)", allowed, reason)
	}
}

func TestTerminalReporterReject(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := newTerminalReporterForTest(mgr, strings.NewReader("n\n"), true)
	allowed, reason := runWaitingForApproval(t, mgr, rep)
	if allowed || reason != "终端用户拒绝" {
		t.Fatalf("n 应拒绝，得到 (%v, %q)", allowed, reason)
	}
}

func TestTerminalReporterInputVariants(t *testing.T) {
	cases := []struct {
		input   string
		allowed bool
		reason  string
	}{
		{"Y\n", true, ""},
		{"yes\n", true, ""},
		{"YES\n", true, ""},
		{"Yes\n", true, ""},
		{"N\n", false, "终端用户拒绝"},
		{"no\n", false, "终端用户拒绝"},
		{"whatever\n", false, "终端用户拒绝"},
		{"\n", false, "终端用户拒绝"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			mgr := NewApprovalManager(time.Minute)
			rep := newTerminalReporterForTest(mgr, strings.NewReader(c.input), true)
			allowed, reason := runWaitingForApproval(t, mgr, rep)
			if allowed != c.allowed || reason != c.reason {
				t.Fatalf("输入 %q 期望 (%v, %q)，得到 (%v, %q)", c.input, c.allowed, c.reason, allowed, reason)
			}
		})
	}
}

func TestTerminalReporterEOF(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := newTerminalReporterForTest(mgr, strings.NewReader("y"), true) // 无换行 → EOF
	allowed, reason := runWaitingForApproval(t, mgr, rep)
	if allowed || reason != "终端输入读取失败" {
		t.Fatalf("EOF 应 fail-closed，得到 (%v, %q)", allowed, reason)
	}
}

func TestTerminalReporterNonInteractive(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := newTerminalReporterForTest(mgr, strings.NewReader("y\n"), false)
	start := time.Now()
	allowed, reason := runWaitingForApproval(t, mgr, rep)
	if allowed || reason != "非交互终端，自动拒绝" {
		t.Fatalf("非交互应拒绝，得到 (%v, %q)", allowed, reason)
	}
	if time.Since(start) > time.Second {
		t.Fatal("非交互拒绝不应阻塞")
	}
}

func TestTerminalReporterTimeout(t *testing.T) {
	mgr := NewApprovalManager(50 * time.Millisecond)
	rep := newTerminalReporterForTest(mgr, neverRead{}, true)
	allowed, reason := runWaitingForApproval(t, mgr, rep)
	if allowed || !strings.Contains(reason, "审批超时") {
		t.Fatalf("不输入应超时拒绝，得到 (%v, %q)", allowed, reason)
	}
}
```

   运行 `go test ./internal/approval/ -run TestTerminalReporter` → **红**（`undefined: newTerminalReporterForTest` / `NewTerminalReporter`）
2. 写 `internal/approval/terminal_reporter.go`（完整内容）：

```go
package approval

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aethelgards/tiny-claw/internal/engine"
)

// TerminalReporter 终端版审批 reporter：包装 engine.TerminalReporter，
// SendApprovalMessage 改为交互式 y/N 提示 + 直接 ResolveApproval。
type TerminalReporter struct {
	base        engine.Reporter // 转发 OnThinking/OnToolCall/OnToolResult/OnMessage
	mgr         *ApprovalManager
	reader      *bufio.Reader // 持久 reader：构造时创建一次，避免过度缓冲吞掉后续输入
	interactive bool          // 构造时检测 stdin 是否为字符设备
}

// NewTerminalReporter 创建终端审批 reporter；stdin 非字符设备（CI/管道）时审批直接拒绝（fail-closed）。
func NewTerminalReporter(mgr *ApprovalManager) engine.Reporter {
	fi, _ := os.Stdin.Stat()
	interactive := fi != nil && fi.Mode()&os.ModeCharDevice != 0
	return &TerminalReporter{
		base:        engine.NewTerminalReporter(),
		mgr:         mgr,
		reader:      bufio.NewReader(os.Stdin),
		interactive: interactive,
	}
}

// newTerminalReporterForTest 供测试注入 reader 与 interactive 标志。
func newTerminalReporterForTest(mgr *ApprovalManager, in io.Reader, interactive bool) engine.Reporter {
	return &TerminalReporter{
		base:        engine.NewTerminalReporter(),
		mgr:         mgr,
		reader:      bufio.NewReader(in),
		interactive: interactive,
	}
}

// 编译期断言：实现 engine.Reporter。
var _ engine.Reporter = (*TerminalReporter)(nil)

func (t *TerminalReporter) OnThinking(ctx context.Context) {
	t.base.OnThinking(ctx)
}

func (t *TerminalReporter) OnToolCall(ctx context.Context, toolName string, args string) {
	t.base.OnToolCall(ctx, toolName, args)
}

func (t *TerminalReporter) OnToolResult(ctx context.Context, toolName string, result string, isError bool) {
	t.base.OnToolResult(ctx, toolName, result, isError)
}

func (t *TerminalReporter) OnMessage(ctx context.Context, content string) {
	t.base.OnMessage(ctx, content)
}

// SendApprovalMessage 打印审批请求并交互式读取一行 y/N：
//   - y/Y/yes → 放行
//   - 其余（含 n、空行）→ 拒绝（"终端用户拒绝"）
//   - 读失败/EOF → fail-closed 拒绝（"终端输入读取失败"）
//   - 非交互 stdin → 立即拒绝不阻塞（"非交互终端，自动拒绝"）
//   - 超时/ctx 取消 → 拒绝（文案与 WaitingForApproval 一致）
//
// 读操作放 goroutine + select 兜底：用户不输入时由 mgr.timeout / ctx 取消解阻塞（不挂死）。
// 超时后遗留的阻塞读 goroutine 属可接受（任务已被清理，后续 ResolveApproval 返回 false 无副作用）。
func (t *TerminalReporter) SendApprovalMessage(ctx context.Context, taskID, toolName, args string) error {
	fmt.Printf("⚠️ 高危操作审批请求\n工具: %s\n参数: %s\n任务 ID: %s\n允许执行? (y/N): ", toolName, args, taskID)
	if !t.interactive {
		t.mgr.ResolveApproval(ctx, taskID, false, "非交互终端，自动拒绝")
		return nil
	}

	lineCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		line, err := t.reader.ReadString('\n')
		if err != nil {
			errCh <- err
			return
		}
		lineCh <- line
	}()

	select {
	case line := <-lineCh:
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			t.mgr.ResolveApproval(ctx, taskID, true, "")
		default:
			t.mgr.ResolveApproval(ctx, taskID, false, "终端用户拒绝")
		}
	case <-errCh:
		t.mgr.ResolveApproval(ctx, taskID, false, "终端输入读取失败")
	case <-time.After(t.mgr.timeout):
		t.mgr.ResolveApproval(ctx, taskID, false, "审批超时，已自动拒绝")
	case <-ctx.Done():
		t.mgr.ResolveApproval(ctx, taskID, false, "审批上下文已取消")
	}
	return nil
}
```

3. 运行 `go test ./internal/approval/ -run TestTerminalReporter -v` → **绿**（6 个用例全过）
4. 全量验证门：`gofmt -l internal/approval/`（空）+ `go build ./...` + `go vet ./...` + `go test ./...` 全绿

**Verification:** Task 2 完成后 `go test ./internal/approval/ -run TestTerminalReporter -v` 显示 6 个测试全过；`go vet ./internal/approval/` 通过。

---

### Task 3: `cmd/claw` 接线 + `cmd/larkbot` 调整 + 全量验证门

**Files:**
- Modify: `cmd/claw/main.go`
- Modify: `cmd/larkbot/main.go`

**Steps:**

1. 改 `cmd/claw/main.go`：
   - import 增加 `"github.com/aethelgards/tiny-claw/internal/approval"`
   - 在「技能注册」之后、「创建 Agent 引擎」之前插入审批接线（替换原 `engine.NewTerminalReporter()` 用法）：

```go
	// 4. 审批：危险命令经终端交互审批（非交互 stdin 一律拒绝；非法超时回退默认 5m）
	approvalMgr := approval.NewApprovalManager(approval.ParseApprovalTimeout(settings.ApprovalTimeout))
	reg.Use(approval.ApprovalMiddleware(approvalMgr))
	reporter := approval.NewTerminalReporter(approvalMgr)
```

   - 引擎构造改用 `reporter`（替换 `engine.NewTerminalReporter()`）：

```go
	// 5. 创建 Agent 引擎，将 provider 与工具注册表绑定起来（审批 reporter 转发 CLI 输出 + y/N 交互）
	agent := engine.NewAgentEngine(p, reg, *settings, reporter, composer,
		ctxpkg.NewRecoveryManager(),
		engine.NewReminderInjector(3),
	)
```

   - `agent.Run` 调用点注入审批 ctx（loop 的 ctx 原样贯穿到 `registry.Execute`，`loop.go:140`）：

```go
	// 7. 启动 Agent：注入审批上下文（reporter + 本地审批者 "local"），交给 AI 循环处理
	runCtx := approval.WithApprovalContext(context.Background(), reporter, "local")
	if err := agent.Run(runCtx, os.Args[1]); err != nil {
```

   - 其余（配置加载/provider/工具注册/参数校验）不动；步骤注释序号顺延（原 4→5、5→6、6→7）
2. 改 `cmd/larkbot/main.go`：
   - import 增加 `"github.com/aethelgards/tiny-claw/internal/approval"`
   - `lark.NewApprovalManager(lark.ParseApprovalTimeout(settings.ApprovalTimeout))` → `approval.NewApprovalManager(approval.ParseApprovalTimeout(settings.ApprovalTimeout))`
   - `reg.Use(lark.ApprovalMiddleware(approvalMgr))` → `reg.Use(approval.ApprovalMiddleware(approvalMgr))`
   - `lark.NewApprovalCardHandler(approvalMgr)` 签名不变（参数类型自动变为 `*approval.ApprovalManager`），调用点不动
   - 注释「非法超时回退默认 5m」保留
3. 全量验证门（最终）：
   - `gofmt -l internal/ cmd/` → 空输出
   - `go build ./...` → 通过
   - `go vet ./...` → 通过
   - `go test ./...` → 全绿
   - `go list -deps ./cmd/claw | grep larksuite` → 空（CLI 无 lark 依赖）
   - `go list -deps ./cmd/larkbot | grep larksuite` → 非空（larkbot 保留 lark，正常）

**Verification:** 上述 6 条全部满足；`go test ./internal/approval/` 与 `go test ./internal/gateway/lark/` 独立跑均绿。

---

## 验收清单（全部 Task 完成后）

- [x] `internal/approval/` 下 5 个源文件 + 4 个测试文件存在（实际 4 源 + 4 测试：`approval.go`/`approval_ctx.go`/`approval_middleware.go`/`terminal_reporter.go` + 4 个 `_test.go`，计划计数偏差不影响验收）；`internal/gateway/lark/` 下 3 个审批核心文件已删除
- [x] `grep -r "larksuite" internal/approval/` 无输出
- [x] `grep -rn "approverOpenID\|approvalTask\b\|mgr\.getTask" internal/ cmd/` 无输出（旧名全清除）
- [x] `gofmt -l internal/ cmd/` 空输出
- [x] `go build ./...`、`go vet ./...`、`go test ./...` 全绿
- [x] `go list -deps ./cmd/claw | grep larksuite` 为空；`go list -deps ./cmd/larkbot | grep larksuite` 非空
- [x] 未执行任何 git commit

## 自检（writing-plans 强制）

- **无占位符**：所有新文件/改动文件均给出完整代码或精确替换指令（Task 1 随迁测试为「复制 + 机械替换表」，Task 2/3 为完整代码）；无 "TODO"/"…"
- **可验证**：每个 Task 的 Verification 均为可执行命令 + 明确预期输出；最终验收清单逐项可勾
- **TDD**：Task 1 先迁移测试（红）再平移实现（绿）；Task 2 先写测试（红）再实现（绿）
- **范围锁定**：引擎层/配置零改动；lark 行为零变化；不引入新依赖；不 git commit
- **已知边界**：`neverRead` 读 goroutine 在超时用例后遗留阻塞（测试进程退出即回收，无副作用）；`engine.TerminalReporter` 旧误导文案保留不动
