# 终端模式（cmd/claw）审批适配 技术方案

> 日期：2026-08-11
> 状态：待评审
> 范围：审批核心抽离为共享包 `internal/approval` + 终端交互式审批（y/N）+ `cmd/claw` 接线；`cmd/larkbot` 改为引用共享包（行为零变化）
> 说明：复用 Lark 版同一套 `ApprovalManager`/`ApprovalMiddleware`/`WithApprovalContext`，引擎层零改动；终端无卡片回调，审批以**阻塞读 stdin 的 y/N 交互**完成

## 1. 背景与现状

Lark 版审批（`2026-08-11-approval-manager-lark-design.md`）已落地：危险命令经 `ApprovalMiddleware` 拦截 → `WaitingForApproval` 发卡片 → 回调 `ResolveApproval`。该链路核心（`ApprovalManager`/`ApprovalMiddleware`/`WithApprovalContext`/`isDangerousCommand`）目前位于 `internal/gateway/lark/`，但**零 lark 依赖**（仅 import `engine`/`tools`/`schema`）。

终端模式（`cmd/claw`）现状：

| # | 问题 | 现状 | 影响 |
|---|------|------|------|
| P1 | 未接线 | `cmd/claw/main.go` 未注册审批中间件 | CLI 下危险命令无审批直接执行 |
| P2 | 无解析机制 | `TerminalReporter.SendApprovalMessage` 打印 "请回复 approve <taskID>"，但终端无任何通道能解析该审批 | 即使接线中间件，审批也会挂满 5 分钟超时拒绝，且文案误导 |
| P3 | 无审批上下文 | CLI 走 `agent.Run` 直通 loop，没有 lark 的 `engine_processor` 注入点 | 中间件 `approvalContextFrom(ctx)` 必失败 → 恒 "缺少审批上下文" 拒绝 |
| P4 | 包依赖方向 | 审批核心在 `gateway/lark` 包内 | 若 CLI import lark 包，二进制拖入整个 larksuite SDK（事件调度/WS），且依赖方向颠倒 |

需求：终端模式下 Agent 触发危险命令时，本地用户经**交互式 y/N 提示**审批；stdin 非终端（CI/管道）时**直接拒绝**（fail-closed）；超时语义与 Lark 版一致（默认 5 分钟，可配置）；CLI 二进制不依赖 lark SDK。

## 2. 设计目标与约束

### 2.1 目标

1. 审批核心抽离为共享包 `internal/approval`，`cmd/claw` 与 `cmd/larkbot` 共同引用，CLI 不依赖 lark
2. 终端交互：打印危险命令详情 + `(y/N)` 提示，阻塞读 stdin；`y`/`Y`/`yes` 放行，其余拒绝
3. 非交互 stdin（非 TTY）→ 直接拒绝，不阻塞（CI/管道安全默认）
4. 读失败/EOF → fail-closed 拒绝
5. 超时（默认 5m，可配置）与 ctx 取消沿用 `WaitingForApproval` 现有语义，不阻塞挂死
6. 引擎层（`internal/engine`）零改动；`cmd/larkbot` 行为零变化

### 2.2 硬约束（来自现有代码）

| # | 约束 | 来源 | 影响 |
|---|------|------|------|
| C1 | `tools.MiddlewareFunc` 签名固定 `func(ctx, schema.ToolCall) (bool, error)`，拒绝原因经 `error` 成为工具结果 | `registry.go` | 终端拒绝文案直接进入模型可见的工具结果 |
| C2 | `AgentEngine.Run(ctx, prompt)` 的 ctx 原样贯穿 `parallelExecTools → registry.Execute` | `loop.go:56, 131, 140` | 审批 ctx 可在 `cmd/claw` 的 `Run` 调用点注入，引擎零改动 |
| C3 | `engine.Reporter.SendApprovalMessage(ctx, taskID, toolName, args) error` 是 `WaitingForApproval` 的通知钩子；reporter 无 manager 引用 | `reporter.go:14`、`approval.go:85` | 终端 reporter 需持有 `*ApprovalManager` 才能在通知钩子内完成交互+`ResolveApproval`（注册任务先于钩子执行，`ResolveApproval` 必命中） |
| C4 | 审批核心仅依赖 `engine`/`tools`/`schema`，`approval_card.go`/`approval_handler.go` 绑定 lark SDK 类型 | 现有代码 | 核心可整体平移；卡片/回调留 lark |
| C5 | 交互阻塞发生在引擎执行协程内（单进程 CLI，无 worker 并发） | `cmd/claw/main.go` | 阻塞读 stdin 可接受（审批本就是门禁）；lark 版同类阻塞已接受 |
| C6 | TTY 检测：`os.Stdin.Stat()` 的 `ModeCharDevice` | Go 标准库 | 构造时检测一次，测试可注入 fake reader + interactive 标志 |

## 3. 总体架构

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
                             │                          ├─ y → ResolveApproval(true, "")
                             │                          ├─ 其他 → ResolveApproval(false, "终端用户拒绝")
                             │                          ├─ EOF/读失败 → ResolveApproval(false, "终端输入读取失败")
                             │                          └─ 非 TTY → ResolveApproval(false, "非交互终端，自动拒绝")
                             └─ select{ 结果 | 超时 | ctx.Done } → 放行或"执行被系统拦截。原因: …"
```

包依赖（单向）：

```
internal/engine ──▶（无新依赖）
internal/approval ──▶ engine, tools, schema
internal/gateway/lark ──▶ approval, engine, tools, larksuite SDK
cmd/claw ──▶ approval, engine, tools, provider, config, context
cmd/larkbot ──▶ approval, lark, engine, tools, provider, config, context
```

## 4. 核心设计

### 4.1 包结构重构：`internal/approval`（新）

从 `internal/gateway/lark` 平移 3 个文件（内容不变，仅包名 + 必要导出）：

| 来源 | 目标 | 变更 |
|------|------|------|
| `lark/approval.go` | `approval/approval.go` | `approvalTask` → 导出 `Task`（handler/结果卡跨包用）；`getTask` → 导出 `GetTask` |
| `lark/approval_ctx.go` | `approval/approval_ctx.go` | 无（API 不变） |
| `lark/approval_middleware.go` | `approval/approval_middleware.go` | 无（API 不变） |

`internal/gateway/lark` 删除上述 3 文件，改为引用共享包：

- `approval_handler.go`：`mgr *approval.ApprovalManager`；`mgr.GetTask(taskID)` 取元数据；`BuildApprovalResultCard(task approval.Task, ...)`
- `engine_processor.go`：`WithApprovalContext` → `approval.WithApprovalContext`
- `approval_card.go`：`BuildApprovalResultCard` 签名改用 `approval.Task`

导出面（`internal/approval`）：`ApprovalResult`、`Task`、`ApprovalManager`、`NewApprovalManager`、`WaitingForApproval`、`ResolveApproval`、`GetTask`、`ParseApprovalTimeout`、`ApprovalMiddleware`、`WithApprovalContext`、`NewTerminalReporter`。`isDangerousCommand`/`dangerPatterns`/`approvalContextFrom` 保持未导出（仅包内/中间件使用）。

测试随迁：`approval_test.go` / `approval_ctx_test.go` / `approval_middleware_test.go` → `internal/approval`（package approval，断言不变）；lark 保留 `approval_card_test.go` / `approval_handler_test.go` / `lark_reporter_test.go`（更新 `approval.Task`/`approval.ApprovalManager` 引用）。

### 4.2 交互式终端 reporter（`approval/terminal_reporter.go`，新）

```go
// TerminalReporter 终端版审批 reporter：包装 engine.TerminalReporter，
// SendApprovalMessage 改为交互式 y/N 提示 + 直接 ResolveApproval。
type TerminalReporter struct {
	base        engine.Reporter // 转发 OnThinking/OnToolCall/OnToolResult/OnMessage
	mgr         *ApprovalManager
	in          io.Reader // 默认 os.Stdin；测试注入
	interactive bool      // 构造时检测 stdin 是否为字符设备
}

func NewTerminalReporter(mgr *ApprovalManager) engine.Reporter
func newTerminalReporterForTest(mgr *ApprovalManager, in io.Reader, interactive bool) engine.Reporter
```

`SendApprovalMessage(ctx, taskID, toolName, args)` 流程：

1. 打印审批请求（工具名 + 参数 + 任务 ID + `允许执行? (y/N): `）
2. **非交互**（`interactive == false`）→ `ResolveApproval(ctx, taskID, false, "非交互终端，自动拒绝")`，返回 nil（不阻塞，CI 不挂 5m）
3. 交互 → 阻塞读一行（`bufio.Reader.ReadString('\n')`）：`strings.TrimSpace` 后匹配 `y`/`Y`/`yes` → `ResolveApproval(ctx, taskID, true, "")`；其余（含 `n`）→ `ResolveApproval(ctx, taskID, false, "终端用户拒绝")`
4. 读失败/EOF → fail-closed：`ResolveApproval(ctx, taskID, false, "终端输入读取失败")`，返回 nil

时序保证（C3）：`WaitingForApproval` 先注册 `pendingTasks[taskID]` 再调 `SendApprovalMessage`，故钩子内 `ResolveApproval` 必命中任务；钩子返回 nil 后 `WaitingForApproval` 从 channel 立即取到结论。重复投递安全：`ResolveApproval` 缓冲满返回 false 不阻塞（现有语义），且钩子内只投递一次。

超时：用户 5m 不输入 → `WaitingForApproval` 的 `time.After` 自动拒绝（终端与卡片同语义）。

### 4.3 语义重命名：`approverOpenID` → `approverID`

终端审批者身份是本地用户（`"local"`），`approverOpenID` 为 lark 专属命名，挪入共享包后改名更准确。波及：

- `approval.Task.ApproverID`、`WithApprovalContext(ctx, reporter, approverID string)`
- lark `engine_processor.go` 传 `msg.OpenID`（值不变）；lark handler 校验 `req.Operator.OpenID != task.ApproverID`
- 相关测试断言同步

### 4.4 `cmd/claw/main.go` 接线

```go
// 审批：危险命令经终端交互审批（非交互 stdin 一律拒绝；非法超时回退默认 5m）
approvalMgr := approval.NewApprovalManager(approval.ParseApprovalTimeout(settings.ApprovalTimeout))
reg.Use(approval.ApprovalMiddleware(approvalMgr))
reporter := approval.NewTerminalReporter(approvalMgr) // 替换 engine.NewTerminalReporter()
agent := engine.NewAgentEngine(p, reg, *settings, reporter, composer,
	ctxpkg.NewRecoveryManager(), engine.NewReminderInjector(3))
// Run 入口注入审批 ctx（loop 的 ctx 原样贯穿到 registry.Execute，loop.go:140）
runCtx := approval.WithApprovalContext(context.Background(), reporter, "local")
agent.Run(runCtx, os.Args[1])
```

审批者身份固定 `"local"`（单机本地用户；终端无多用户概念）。

## 5. 接线（`cmd/larkbot/main.go`）

- `lark.NewApprovalManager` → `approval.NewApprovalManager`；`lark.ApprovalMiddleware` → `approval.ApprovalMiddleware`；新增 `internal/approval` import
- `lark.NewApprovalCardHandler(approvalMgr)` 签名不变（参数类型改为 `*approval.ApprovalManager`），行为零变化
- `internal/gateway/lark` 内 `approval.go`/`approval_ctx.go`/`approval_middleware.go` 删除（已平移）

## 6. 错误处理与竞态

| 场景 | 处理 | 保证 |
|------|------|------|
| 非 TTY（CI/管道） | 钩子内立即 `ResolveApproval(false, "非交互终端，自动拒绝")` | 不阻塞，fail-closed |
| 读失败/EOF | `ResolveApproval(false, "终端输入读取失败")` | fail-closed |
| 用户输入非 y/yes | `ResolveApproval(false, "终端用户拒绝")` | 拒绝文案进入模型可见工具结果 |
| 5m 无输入 | `WaitingForApproval` `time.After` 超时拒绝 | 不挂死 |
| ctx 取消 | `WaitingForApproval` `ctx.Done` 分支 | 引擎退出不泄漏任务 |
| 重复投递 | `ResolveApproval` 缓冲满返回 false（钩子内仅投递一次，正常路径不会触发） | 不阻塞 |
| 任务已清理后投递 | `ResolveApproval` 查无任务 → false + 告警 | 与 lark 现有语义一致 |

## 7. 测试计划

| 文件 | 变更 | 用例 |
|------|------|------|
| `internal/approval/approval_test.go` | 随迁（改包名/类型引用） | 既有 manager 全套（注册/投递/超时/取消/fail-closed/重复投递/ParseApprovalTimeout/isDangerousCommand） |
| `internal/approval/approval_ctx_test.go` | 随迁 | 既有 Roundtrip/Missing |
| `internal/approval/approval_middleware_test.go` | 随迁 | 既有放行/拒绝/通过/registry 集成 |
| `internal/approval/terminal_reporter_test.go` | 新增 | ① y 放行（WaitingForApproval 返回 true）② n 拒绝（reason=终端用户拒绝）③ 大小写/yes 变体 ④ EOF → fail-closed ⑤ 非交互（interactive=false）→ 拒绝不阻塞 ⑥ 超时路径（不输入，短 timeout → 超时拒绝） |
| `internal/gateway/lark/approval_card_test.go` | 更新引用 | 既有断言不变 |
| `internal/gateway/lark/approval_handler_test.go` | 更新引用 | 既有断言不变 |
| `internal/gateway/lark/lark_reporter_test.go` | 不动 | — |

验证门：`gofmt -l internal/ cmd/` + `go build ./...` + `go vet ./...` + `go test ./...` 全绿；`cmd/claw` 与 `cmd/larkbot` 均编译通过（无 lark 依赖进 CLI：`go list -deps ./cmd/claw | grep larksuite` 为空）。

## 8. 已知限制（接受，不在本方案范围）

1. `engine.TerminalReporter.SendApprovalMessage` 现有误导文案（"请回复 approve <taskID>"）保留——`cmd/claw` 改用 `approval.NewTerminalReporter` 后该路径不再被主流程使用；`engine.Reporter` 接口仍需其实现（NopReporter 同理）。清理属后续可选
2. 终端审批无自由文本拒绝原因（卡片表单可填，终端保持简单 y/N）；拒绝原因固定文案
3. 交互阻塞引擎执行协程（lark 版 worker 阻塞同类，已接受）；无输入时最长挂 5m
4. 审批者身份固定 `"local"`，不做终端多用户/多终端身份区分

## 9. 实现阶段（评审通过后细化）

1. 抽包：平移 3 文件 → `internal/approval`，导出 `Task`/`GetTask`，lark 引用改 import（TDD：先跑随迁测试红 → 改 → 绿）
2. `terminal_reporter.go` + 测试（TDD）
3. `cmd/claw` 接线 + `cmd/larkbot` 调整
4. 全量验证门 + `go list -deps` 无 lark 检查

## 10. 备选方案回顾

| 方案 | 说明 | 结论 |
|------|------|------|
| A. 抽 `internal/approval`（采用） | 核心平移到共享包，CLI 不依赖 lark；lark 保留卡片/回调 | 依赖单向干净，CLI 二进制小；代价是 lark 内 3 文件+测试的 import 改动 |
| B. `cmd/claw` import lark | 零重构，但依赖方向颠倒，CLI 拖入整个 larksuite SDK | 拒绝 |
| C. 卡片/回调也进共享包 | 卡片 JSON 与 callback 类型是 lark SDK 专属，污染中性包 | 拒绝 |
