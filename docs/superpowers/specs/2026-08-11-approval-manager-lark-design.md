# ApprovalManager 接入飞书卡片审批 技术方案

> 日期：2026-08-11
> 状态：待评审
> 范围：`internal/gateway/lark/` 审批链路改造（ApprovalManager 增强 + 卡片发送/回调）+ 最小配置接入
> 说明：复用现有 `tools.MiddlewareFunc` 钩子，**引擎层零改动**；审批结果通过注册表中拦截返回

## 1. 背景与现状

`internal/gateway/lark/approval.go` 已有一个半成品的 `ApprovalManager`：

- `ApprovalResult{Allowed bool, RejectReason string}` 作为审批结论
- `ApprovalManager{mu sync.RWMutex, pendingTasks map[string]chan ApprovalResult}` + 全局单例 `GlobalApprovalManager`
- `WaitingForApproval(ctx, taskID, toolName, args, reporter)`：注册 channel → `reporter.SendApprovalMessage` 发送通知 → **阻塞等待** `<-ch`
- `ResolveApproval(ctx, taskID, allowed, reason)`：向 channel 投递结果，任务不存在时仅告警
- `isDangerousCommand(toolName, args)`：仅对 `bash` 匹配高危模式（`rm -r` / `sudo` / `drop` / `>.*\.go`），`write_file`/`edit_file` 恒放行

现状问题（本次要解决的）：

| # | 问题 | 现状 | 影响 |
|---|------|------|------|
| P1 | 通知形式 | `SendApprovalMessage` 发**纯文本**（"请回复 approve/reject"），`lark_reporter.go:60-68` | 无法点按审批，交互割裂 |
| P2 | 审批者范围 | 任何人看到消息都能批（无 open_id 校验） | 安全边界缺失 |
| P3 | 无限期阻塞 | `<-ch` 无超时 | worker 单协程串行（`worker.go`），一旦无响应则整条管道卡死 |
| P4 | 发送失败放行 | `return true, "发送审批消息失败…"`（fail-open） | 高危操作在通知失败时被放行，方向错误 |
| P5 | 拒绝原因 | 无结构化承载 | 无法告知模型为何被拒 |
| P6 | 未接线 | 无任何调用方 | 功能等于不存在 |

需求：危险命令执行前必须经**发起请求的用户**审批；审批以**飞书卡片**呈现（展示工具名/参数），支持**通过 / 拒绝（可填原因）**；超时自动拒绝（默认 5 分钟，可配置）。

## 2. 设计目标与约束

### 2.1 目标

1. 危险命令（`isDangerousCommand` 命中）执行前强制审批，**fail-closed**
2. 审批交互：卡片内一键通过/拒绝，拒绝原因在卡片内嵌输入框填写（单次交互完成）
3. 仅**发起消息的用户**（open_id 匹配）可审批，其他人点击被拒绝
4. 审批有超时（默认 5 分钟），超时自动拒绝并解除阻塞，**不拖死 worker**
5. 引擎层（`internal/engine/loop.go`）与工具实现零改动
6. 审批配置（超时）走现有配置分层体系（默认 → 文件 → 环境变量）

### 2.2 硬约束（来自现有代码）

| # | 约束 | 来源 | 影响 |
|---|------|------|------|
| C1 | `tools.MiddlewareFunc` 签名固定：`func(ctx, schema.ToolCall) (bool, error)`，false 时注册表用 `fmt.Sprintf("执行被系统拦截。原因: %s", rejectReason)` 构造结果 | `registry.go:14, 68-77` | 拒绝原因必须经 `error` 返回，文案直接成为工具结果 |
| C2 | worker 单协程串行消费所有 chat（`worker.go`），审批等待会阻塞后续消息 | `worker.go` | 等待必须被超时钳制；多 chat 头阻塞为已知限制（见 §8） |
| C3 | `EngineProcessor.Process` 每消息新建 `LarkReporter`（持有 bot/chatID/tenantKey），`OpenID` 已在 `IncomingMessage` 解析（备用字段） | `engine_processor.go:38`、`message.go:12-17` | reporter + openID 可从 processor 注入 ctx，中间件零依赖引擎 |
| C4 | `schema.ToolCall{ID, Name, Arguments}`；`Arguments` 为 `json.RawMessage` | `schema/message.go:22-26` | taskID 用 `call.ID`；args 传 `string(call.Arguments)` |
| C5 | 审批等待在中间件内阻塞执行 → 必须带 ctx 感知 + 超时，且超时后要从 map 清理，避免后到回调误投递 | `approval.go` 现有结构 | `WaitingForApproval` 改为 `select{ch, ctx.Done, time.After}` |
| C6 | SDK v3.9.10 的 `larkcard` 包**无 input 模块 builder** | 本地 module cache 确认 | 卡片用**原始 card JSON v2 字符串**，不依赖 builder |
| C7 | 卡片回调经 WS 进入 dispatcher：`d.OnP2CardActionTrigger(handler)` 返回的 response 可**原位更新卡片**（`CardActionTriggerResponse{Toast, Card:{Type:"raw", Data}}`） | SDK 事件分发 + 官方文档确认 | 点击后卡片即时变为结果卡，无按钮（防重复点击） |
| C8 | card JSON v2：input 与按钮组合**必须**包在 form 容器中（官方硬性要求）；form 内所有交互组件需全局唯一 `name`，提交按钮 `form_action_type: "submit"`，form 内必须含至少一个 submit 按钮；input 值经回调 `action.form_value` 返回 | 官方 form 容器/input/button 文档（v3.9.10 的 `larkcard` 亦无对应 builder，只能用原始 JSON） | §4.5 卡片定型：单 form + 双 submit 按钮 + `required:false` 的 input |

## 3. 总体架构

```
用户发消息 ──▶ ParseMessageEvent ──▶ queue ──▶ Worker(单协程)
                                                  │ Process(msg)
                                                  ▼
                              EngineProcessor: 注入 WithApprovalContext(ctx, reporter, msg.OpenID)
                                                  │ agent.Run(ctx, text)
                                                  ▼
                                     AgentEngine 循环 ──▶ registry.Execute(ctx, call)
                                                              │
                                                              ▼ 遍历 mws
                                               ApprovalMiddleware(approvalMgr)  ← 新
                                                  │ isDangerousCommand(call)?
                                                  │ 否 → 放行
                                                  │ 是 → WaitingForApproval(ctx, call.ID, name, args,
                                                  │         reporter, approverOpenID)
                                                  │        ├─ SendApprovalMessage ─▶ 卡片消息（新）
                                                  │        └─ select{ 审批结果 | 超时 | ctx.Done }
                                                  ▼
                               允许 → 继续执行工具；拒绝 → 返回"执行被系统拦截。原因: …"
```

卡片侧：

```
审批卡片（red header）──▶ 用户点击 通过/拒绝 ──▶ WS ──▶ OnP2CardActionTrigger
                                                        │ NewApprovalCardHandler(approvalMgr)  ← 新
                                                        │   ① 校验 operator.open_id == task.approverOpenID
                                                        │   ② ResolveApproval(taskID, allowed, reason)
                                                        ▼
                                        CardActionTriggerResponse{ Toast + Card(raw v2) }
                                        卡片原位更新为结果卡（green/red，无按钮）
```

## 4. 核心设计

### 4.1 ApprovalManager 增强（`approval.go`）

```go
type ApprovalResult struct {
    Allowed      bool
    RejectReason string
}

// pendingTasks 的 value 从 chan 升级为任务元数据
type approvalTask struct {
    ch             chan ApprovalResult
    approverOpenID string // 谁发的请求，只有 TA 能批
    toolName       string // 用于结果卡展示
    args           string
}

type ApprovalManager struct {
    mu           sync.RWMutex
    pendingTasks map[string]approvalTask
    timeout      time.Duration
}

func NewApprovalManager(timeout time.Duration) *ApprovalManager // timeout <= 0 视为 5 分钟默认

// 返回值：allowed, reason；reason 直接作为中间件拒绝文案
func (m *ApprovalManager) WaitingForApproval(ctx context.Context, taskID, toolName, args string, reporter engine.Reporter, approverOpenID string) (bool, string) {
    task := approvalTask{ch: make(chan ApprovalResult, 1), approverOpenID: approverOpenID, toolName: toolName, args: args}
    // ① 注册任务
    // ② SendApprovalMessage 失败 → fail-closed：清理任务，返回 (false, "审批通知发送失败，已拒绝执行")
    // ③ select {
    //      case res := <-task.ch:             → 清理任务，返回 res.Allowed, res.RejectReason
    //      case <-time.After(m.timeout):      → 清理任务，返回 (false, "审批超时（5 分钟），已自动拒绝")
    //      case <-ctx.Done():                 → 清理任务，返回 (false, "审批上下文已取消")
    //    }
    // 所有分支（成功/超时/取消）均执行 delete(m.pendingTasks, taskID)，保证任务生命周期终结
}

// 返回是否成功投递（handler 据此决定 toast：任务不存在 → "该请求已处理或不存在"）
func (m *ApprovalManager) ResolveApproval(ctx context.Context, taskID string, allowed bool, reason string) bool

// 供 handler 读取任务元数据（approverOpenID/toolName/args），用于身份校验与结果卡
func (m *ApprovalManager) getTask(taskID string) (approvalTask, bool)
```

要点：

- **超时清理**：超时/取消路径必须 `delete(pendingTasks, taskID)`，此后 `ResolveApproval` 查不到 → 返回 false，后到点击不会误投递。chan 容量 1 保证超时竞态下 `ResolveApproval` 的 send 不阻塞（结果落入已无等待者的缓冲，无害）。
- **`GlobalApprovalManager` 保留**：作为 CLI/测试缺省单例；`cmd/larkbot` 用 `NewApprovalManager(settings 超时)` 显式创建，避免隐式全局状态。
- **fail-closed**（P4）：发送失败不再 `return true`。

### 4.2 审批上下文（`approval_ctx.go`，新）

```go
type approvalCtxKey struct{}

type approvalContext struct {
    reporter engine.Reporter
    openID   string // 消息发送者 open_id
}

func WithApprovalContext(ctx context.Context, reporter engine.Reporter, openID string) context.Context
func approvalContextFrom(ctx context.Context) (approvalContext, bool)
```

`EngineProcessor.Process` 注入（`engine_processor.go:38` 处一行）：

```go
reporter := NewLarkReporter(p.bot, msg.ChatID, msg.TenantKey)
ctx = WithApprovalContext(ctx, reporter, msg.OpenID)
```

CLI（`cmd/claw`）不注册审批中间件（见 §5），因此该路径在 CLI 不触发；"缺上下文即拒绝"的 fail-closed 分支仅作**防御性兜底**——若将来有人注册了中间件却忘记注入上下文，危险命令会被安全拒绝而非放行。

### 4.3 审批中间件（`approval_middleware.go`，新）

```go
func ApprovalMiddleware(mgr *ApprovalManager) tools.MiddlewareFunc {
    return func(ctx context.Context, call schema.ToolCall) (bool, error) {
        if !isDangerousCommand(call.Name, string(call.Arguments)) {
            return true, nil // 非危险命令直接放行，零开销
        }
        ac, ok := approvalContextFrom(ctx)
        if !ok {
            return false, errors.New("缺少审批上下文，无法执行高危操作")
        }
        allowed, reason := mgr.WaitingForApproval(ctx, call.ID, call.Name, string(call.Arguments), ac.reporter, ac.openID)
        if !allowed {
            return false, errors.New(reason)
        }
        return true, nil
    }
}
```

拒绝时注册表产出（C1）：`执行被系统拦截。原因: <reason>`，作为工具结果回给模型。

### 4.4 卡片发送（`lark.go` + `lark_reporter.go`）

`Bot` 新增（`lark.go`）：

```go
// SendCardMessage 发送 interactive 卡片消息；content 为 card JSON v2 字符串（非二次编码）
func (b *Bot) SendCardMessage(ctx context.Context, chatID, tenantKey, cardJSON string) error {
    // b.cli.Im.Message.Create(ReceiveIdType("chat_id"),
    //   Body: MsgType(larkim.MsgTypeInteractive).ReceiveId(chatID).Content(cardJSON))
}
```

`LarkReporter.SendApprovalMessage` 改写（`lark_reporter.go:60-68`）：

```go
func (l *LarkReporter) SendApprovalMessage(ctx context.Context, taskID, toolName, args string) error {
    l.mu.Lock() // 与文本发送共享互斥锁，保持发送顺序
    defer l.mu.Unlock()
    return l.bot.SendCardMessage(ctx, l.chatID, l.tenantKey, BuildApprovalCard(taskID, toolName, args))
}
```

### 4.5 卡片 JSON（`approval_card.go`，新）

发送卡（card JSON v2，header `red`）：

```json
{
  "schema": "2.0",
  "header": {
    "template": "red",
    "title": { "tag": "plain_text", "content": "⚠️ 高危操作审批请求" }
  },
  "body": {
    "elements": [
      { "tag": "markdown", "content": "**Agent 请求执行以下操作：**\n**工具：** `bash`\n**参数：**\n```\n<args>\n```\n**任务 ID：** `<taskID>`" },
      { "tag": "hr" },
      {
        "tag": "form",
        "name": "approval_form",
        "elements": [
          { "tag": "input", "name": "reject_reason", "required": false, "label": { "tag": "plain_text", "content": "拒绝原因（选填）" }, "placeholder": { "tag": "plain_text", "content": "仅拒绝时填写" } },
          { "tag": "button", "name": "approve_btn", "form_action_type": "submit", "type": "primary", "text": { "tag": "plain_text", "content": "✅ 通过" }, "behaviors": [{ "type": "callback", "value": { "action": "approve", "task_id": "<taskID>" } }] },
          { "tag": "button", "name": "reject_btn", "form_action_type": "submit", "type": "danger", "text": { "tag": "plain_text", "content": "❌ 拒绝" }, "behaviors": [{ "type": "callback", "value": { "action": "reject", "task_id": "<taskID>" } }] }
        ]
      }
    ]
  }
}
```

要点（C8 落地）：

- **form 容器必须**（官方硬性要求：input 与按钮组合需同处 form；form 内必须含至少一个 submit 按钮）。两个 submit 按钮靠 `name`（`approve_btn`/`reject_btn`）+ `behaviors[].value`（`{"action": "approve"|"reject", "task_id": …}`）区分——点击任一按钮都会触发 form 提交，回调同时携带按钮 `name`/`value` 与全部 form 数据。
- **`required: false`**（缺省即 false，显式声明防误读）：若置 true，用户空原因点「通过」会被前端拦截、**不发回调**——审批流必须允许无条件通过。
- 并排布局可用 `column_set` 包裹两个按钮（官方 demo 用法）；纵排亦可，本方案不强制。

参数/任务 ID 直接拼入 markdown 时需做转义（`\`` 与 `\n` 安全处理）；args 过长（> 512 字符）截断加 `…`，避免卡片超限。

**拒绝原因的取值（设计决策）**：input 位于 form 内，回调经 `event.Event.Action.FormValue["reject_reason"]`（`map[string]interface{}`，取值后断言为 string）返回；`action.input_value` 仅适用于 form 外的独立 input，本方案不使用。取到空串视为未填（拒绝原因可选）。按钮身份以 `action.name`（`approve_btn`/`reject_btn`）为准，`action.value["action"]` 交叉校验。

结果卡（回调 response 原位更新，无按钮防重复点击）：

- 通过：header `green`，title `✅ 已通过`，正文含 审批人 open_id + 任务 ID + 工具/参数
- 拒绝：header `red`，title `❌ 已拒绝`，正文追加 拒绝原因（未填则省略）

```go
// 返回 {Toast, Card:{Type:"raw", Data: 结果卡}}
func BuildApprovalResultCard(task approvalTask, allowed bool, reason, operatorOpenID string) string
func BuildApprovalCard(taskID, toolName, args string) string
```

### 4.6 卡片回调处理（`approval_handler.go`，新）

```go
// 注册：d.OnP2CardActionTrigger(NewApprovalCardHandler(mgr))
// handler 签名：func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error)
// 官方 SDK 中无 OnP2CardActionTriggerV1（那是 Python SDK 命名）；Go SDK 只有 OnP2CardActionTrigger，
// 事件类型固定为 "card.action.trigger"（新版本回调，schema 2.0）。旧版 larkcard.NewCardActionHandler 不走 WS dispatcher，勿混用。
func NewApprovalCardHandler(mgr *ApprovalManager) func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error)
```

处理流程：

1. **解析**：`event.Event.Operator.OpenID`（恒存在，非指针）；按钮身份 `event.Event.Action.Name`（`approve_btn`/`reject_btn`），用 `event.Event.Action.Value["action"]`、`Value["task_id"]` 交叉校验；拒绝原因 `event.Event.Action.FormValue["reject_reason"]`（`interface{}` 断言为 string，空视为未填）
2. **任务存在性**：`mgr.getTask(taskID)` 失败 → 返回 `Toast{type:"warning", content:"该请求已处理或不存在"}`，不更新卡片
3. **身份校验**（P2）：`operator.OpenID != task.approverOpenID` → 返回 `Toast{type:"error", content:"只有发起请求的用户才能审批"}`，不更新卡片（结果卡不暴露给无关人）
4. **投递**：`mgr.ResolveApproval(ctx, taskID, approve, reason)`；approve 时 reason 恒空
5. **响应（主路径）**：`CardActionTriggerResponse{Toast{success, "已通过"/"已拒绝"}, Card{Type:"raw", Data: 结果卡JSON}}` —— 回调 3s 窗口内原子原位更新（C7），同时 worker 侧 `WaitingForApproval` 解除阻塞
6. **异步兜底**：若结果卡无法在回调窗口内生成，改用 `client.Im.V1.Message.Patch`（`larkim.NewPatchMessageReqBuilder().MessageId(event.Event.Context.OpenMessageID).Body(&larkim.PatchMessageReqBody{Content: 结果卡JSON})`，卡片 config 需 `update_multi: true`）事后更新。`event.token` 对应的 `interactive/v1/card/update` 延迟更新接口（30 分钟/2 次）当前 Go SDK 未封装，不采用

### 4.7 配置（`config.go`）

```go
type Settings struct {
    ...
    ApprovalTimeout string `json:"approvalTimeout"` // 审批超时（Go duration 字符串），默认 "5m"，<=0 视为默认
}
```

- `defaultSettings`：`ApprovalTimeout: "5m"`
- `applyLayer`：`layer["approvalTimeout"]` 键存在时覆盖
- `applyEnv`：`CLAW_APPROVAL_TIMEOUT`
- 解析失败/非法值 → 回退默认并 `slog.Warn`（配置层不因超时字段 fail）

## 5. 接线（`cmd/larkbot/main.go`）

```go
timeout, err := time.ParseDuration(settings.ApprovalTimeout)
if err != nil || timeout <= 0 {
    timeout = 5 * time.Minute
    slog.Warn("approvalTimeout 非法，使用默认 5m", slog.String("raw", settings.ApprovalTimeout))
}
approvalMgr := lark.NewApprovalManager(timeout)
reg.Use(lark.ApprovalMiddleware(approvalMgr)) // 注册表级中间件，engine 无感

// Start 内：
d.OnP2CardActionTrigger(lark.NewApprovalCardHandler(approvalMgr))
```

CLI（`cmd/claw`）不注册审批中间件（或注册恒放行空中间件），行为与现状一致。

## 6. 错误处理与竞态

| 场景 | 行为 |
|------|------|
| 卡片发送失败 | fail-closed 拒绝，返回原因；任务清理 |
| 审批超时 | 自动拒绝，任务清理；后到回调查不到 → "已处理或不存在" toast |
| 非审批人点击 | 拒绝并 toast，卡片不动，任务保持等待 |
| 超时与点击竞态 | chan 缓冲 1，send 不阻塞；结果落入无等待者缓冲，无害 |
| ctx 取消（进程退出/会话清理） | 按取消路径拒绝并清理 |
| 同一任务重复点击 | 首次投递后任务即删，第二次 → "已处理或不存在" |
| args 超长/含特殊字符 | 截断 + 转义后入卡 |

## 7. 测试计划

| 用例 | 覆盖点 |
|------|--------|
| Manager 注册/投递 | WaitingForApproval 阻塞、ResolveApproval 解除、返回正确 Allowed/Reason；ResolveApproval 返回 true |
| Manager 超时 | 短超时（如 50ms）自动拒绝、返回超时文案；任务从 map 清理；超时后 ResolveApproval 返回 false |
| Manager ctx 取消 | ctx.Done 路径返回取消文案并清理 |
| Manager fail-closed | fake reporter 返回 error → 返回 (false, …)，不阻塞 |
| Manager 重复投递 | 二次 ResolveApproval 返回 false |
| 中间件放行 | 非危险命令（read_file/write_file/edit_file/普通 bash）直接 allow，不触碰 manager |
| 中间件拒绝 | 危险命令 + 无审批 ctx → 拒绝"缺少审批上下文"；有 ctx + 拒绝 → "执行被系统拦截。原因: …" |
| 中间件通过 | 危险命令 + ctx + fake manager 允许 → allow |
| isDangerousCommand | 模式矩阵：`rm -r`、`sudo`、`drop`、`>.*\.go` 命中；良性命令、write/edit 不命中 |
| 卡片构建 | BuildApprovalCard：schema 2.0、header/body、input name、双按钮 value 含 task_id/action、args 截断转义 |
| 卡片回调 | approve / reject（含/不含原因）→ 正确 ResolveApproval + 结果卡 + toast；非审批人 → error toast 不更新卡；task 不存在 → warning toast |
| 配置 | approvalTimeout 层覆盖 + CLAW_APPROVAL_TIMEOUT env + 非法值回退默认 |
| 注册表集成 | registry.Execute 带中间件：拦截结果 IsError=true 且文案含"执行被系统拦截" |

## 8. 已知限制（接受，不在本方案范围）

- **多 chat 头阻塞**：worker 单协程（C2），任一 chat 的审批等待（最长 5 分钟）会阻塞其他 chat 消息入处理。缓解：超时钳制 + 队列有界（`queue.go` 满则丢）。根治需 worker-per-chat/会话级并发，属独立改造，另行设计。
- **审批人与卡片接收人**：卡片发给消息所在 chat（群聊全员可见），但**仅发起者可操作**（身份校验）。@ 指定人/私聊直达不在本方案范围。
- **拒绝原因必填校验**：当前可选。如需必填（无原因点拒绝 → 提示），属后续增强。

## 9. 实现阶段（评审通过后细化）

1. **P0 ApprovalManager 增强**：approvalTask 结构、timeout/ctx select、fail-closed、ResolveApproval 返回 bool、getTask
2. **P0 卡片构建 + 发送**：BuildApprovalCard/BuildApprovalResultCard、Bot.SendCardMessage、LarkReporter.SendApprovalMessage 改卡片
3. **P0 上下文 + 中间件**：approval_ctx.go、ApprovalMiddleware、EngineProcessor 注入
4. **P0 回调处理**：approval_handler.go + main.go 接线 + 配置项
5. **P0 测试**：§7 全部用例
6. **P1 文档**：更新 README 审批说明（如需）

## 10. 备选方案回顾

| 方案 | 结论 | 原因 |
|------|------|------|
| 引擎层钩子（loop.go 内嵌审批） | 否决 | 侵入引擎核心循环，与"引擎无关"现状冲突；中间件已提供同等能力 |
| Module 级拦截至 bash 工具内部 | 否决 | 只覆盖 bash 工具，写文件类危险操作无法统一拦截；审批逻辑散落工具实现 |
| 审批通过后重发消息恢复执行 | 否决 | 破坏工具调用对（C3 类约束），需要暂停/恢复会话，复杂度高 |
| 卡片状态用 `Message.Patch` 异步更新 | 否决（作主路径） | 回调 response 原位更新原子且省一次 API；Patch（`PatchMessageReqBody` + `update_multi: true`）仅作异步兜底（§4.6 步骤 6） |
