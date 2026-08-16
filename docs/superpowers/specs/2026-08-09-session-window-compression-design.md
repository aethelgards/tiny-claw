# Session 窗口压缩技术方案

> 日期：2026-08-09
> 状态：待评审（v2：阈值改为窗口百分比制）
> 范围：`internal/engine/session.go` 的窗口压缩机制 + 必要的基础设施补齐
> 说明：Session 仅作为独立组件交付，**不接入 AgentEngine**，由使用方自行集成

## 1. 背景与现状

`session.go` 目前是半成品：

- `Session` 结构体 + `Append`（内存追加 + JSONL 落盘）已实现
- `GetWorkingMemory(limit)` 是**空方法**，未实现
- `helper.AppendLine` 是**空 stub**，当前落盘实际是 no-op
- 无任何 token 计数 / 窗口管理逻辑

需求：为 LLM 会话设计窗口压缩机制 —— 会话内容达到**模型上下文窗口的指定百分比**（默认 80%）时触发压缩。压缩策略为**混合式（摘要 + 保留最近原始消息）**，触发点为 **Append 时主动压缩**。阈值百分比**可配置**。

## 2. 设计目标与约束

### 2.1 目标

1. 会话历史体积有界：总估算 token 不超过 `contextWindow × ratio / 100`（默认窗口 128k × 80% = 102.4k）
2. 信息保留最大化：旧消息压成一条摘要，最近消息保持原始细节
3. 对 LLM 调用透明：`GetWorkingMemory` 返回的序列可直接喂给 provider
4. 窗口大小与触发百分比均可配置，不硬编码

### 2.2 硬约束（来自现有代码）

| # | 约束 | 来源 | 影响 |
|---|------|------|------|
| C1 | Claude provider 只发一条 system，后者覆盖前者 | `claude.go:47-48, 115-119` | 摘要**禁止**用 `RoleSystem`，否则覆盖真实系统提示词 |
| C2 | 默认底座智谱 GLM 校验 user/assistant 严格交替 | OpenAI 兼容端点行为 | 摘要与相邻消息不能出现两个连续 User |
| C3 | 工具对（assistant `ToolCalls` → 对应 `ToolCallID` 的 User 结果）不可拆散 | `openai.go:52-54`、`claude.go:50-53` | 截断边界必须落在安全位置 |
| C4 | 摘要生成需要一次额外 LLM 调用，且可能失败 | 网络/API 不确定性 | 必须提供无 LLM 回退路径 |
| C5 | 磁盘 JSONL 无类型区分符，所有行都是 Message JSON | `session.go:44-45` | 摘要的持久化需要可区分的记录格式 |

## 3. 总体架构

```
                    ┌─────────────────────────────┐
   Append(msgs) ──▶ │           Session           │
                    │  ┌───────────────────────┐  │
                    │  │ history []schema.Message│ │  ← 只存原始消息，不含摘要
                    │  │ summary string         │  │  ← 摘要独立字段，单一事实源
                    │  │ estTokens()            │  │
                    │  │ compress()             │  │  ← 超阈值时触发
                    │  └───────────────────────┘  │
                    │  JSONL 追加 / 原子重写       │
                    └─────────────┬───────────────┘
                                  │ GetWorkingMemory()
                                  ▼
              [summary(User)] + history  →  使用方拼上 system 后喂给 LLM
```

核心原则：

- **摘要与原始消息分离存储**：`summary` 是 Session 的独立字符串字段，`history` 只存原始消息。`GetWorkingMemory` 时组合输出。避免"摘要混在消息里"带来的解析/标记问题。
- **system 提示词不归 Session 管**：由使用方（`PromptComposer`）自行重建，Session 只管理对话消息。
- **摘要用 `RoleUser` + 前置合并规则**（见 4.4），同时满足 C1、C2。

## 4. 核心设计

### 4.1 数据结构

```go
type Session struct {
    ID        string
    WorkDir   string
    CreatedAt time.Time
    UpdatedAt time.Time

    history []schema.Message // 仅原始对话消息（User/Assistant/Tool 结果）
    summary string           // 压缩摘要（可为空）

    contextWindow int  // 模型上下文窗口总 token 数
    compressRatio int  // 触发百分比，默认 80（即 80%）

    summarizer Summarizer // 摘要生成函数，nil 时退化为纯截断

    mu sync.Mutex
}

// 有效触发阈值：contextWindow × compressRatio / 100
func (s *Session) threshold() int { return s.contextWindow * s.compressRatio / 100 }
```

构造改为函数式选项：

```go
type Summarizer func(ctx context.Context, existingSummary string, msgs []schema.Message) (string, error)

func NewSession(sessionID, workDir string, opts ...Option) *Session
// 选项：
//   WithContextWindow(tokens int) 模型上下文窗口总 token 数（默认 128000）
//   WithCompressRatio(pct int)    触发百分比 1~100（默认 80）
//   WithModel(model string)       按模型名查表设定窗口（显式 WithContextWindow 优先）
//   WithSummarizer(fn Summarizer) 摘要生成函数（nil 时纯截断）
```

**窗口来源优先级**：显式 `WithContextWindow` > `WithModel` 查表 > 默认 128k。`LookupContextWindow(model string) int` 为导出的查表函数，内置常见模型映射（glm-4.x → 128k、claude 全系 → 200k、gpt-4o → 128k、qwen → 128k 等），未知模型返回默认 128k。

> 破坏性变更说明：`NewSession` 签名变化 + `GetWorkingMemory` 签名变化，但 `Session` 目前**无任何调用方**（grep 确认仅 session.go 内部），变更零成本。

### 4.2 Token 估算

不引入 tiktoken 等重依赖，用保守的轻量估算：

```go
// 每个字符约 0.5 token（对 CJK 偏保守，对 ASCII 高估，安全方向）
func estTokens(msg schema.Message) int {
    n := 4 // 每条消息固定开销
    n += utf8.RuneCountInString(msg.Content) / 2
    for _, tc := range msg.ToolCalls {
        n += utf8.RuneCountInString(string(tc.Arguments)) / 2
    }
    return n
}
```

估算精度要求不高 —— 只需"在阈值附近正确触发"，高估导致提前压缩是可接受的（更安全）。

### 4.3 压缩算法（Append 时主动触发）

`Append` 流程：

```
Append(msgs):
  lock
  history = append(history, msgs...)
  UpdatedAt = now
  saveToDisk(msgs)            // JSONL 追加（增量）
  unlock
  if totalTokens() > threshold():   // threshold = contextWindow × compressRatio / 100
      compress()              // 锁外触发，避免持有锁期间做 LLM 调用
```

`compress()` 边界选择（满足 C3、C2）：

1. 目标：丢弃前缀 `[0, cutoff)`，保留 `[cutoff:]` 使 `estTokens(后缀) ≤ threshold - 摘要预算`（摘要预算 = threshold/8）。
2. **从末尾向前累计**，找到最靠前的、满足预算的索引 `i`。
3. **边界修正**：若 `history[i].Role == User`（含工具结果），继续向前扩展到下一个 **Assistant** 消息（保证后缀以 assistant 开头）——工具结果属于其前一个 assistant 的调用对，前缀被整体丢弃时结果同弃（C3）；后缀以 assistant 开头则与前置摘要 User 形成 `User → Assistant` 交替（C2）。
4. 特例：若后缀起点始终是 User（历史最后一条是尚未回复的 User，如未来异步场景），则放弃"以 assistant 开头"，改用 **4.4 的前置合并规则**。
5. 若 `cutoff ≤ 0`（整个历史都超预算且摘要也放不下），仅告警不压缩——接受超限，避免死循环。

`compress()` 摘要生成：

```
compress():
  dropped = history[:cutoff]
  history = history[cutoff:]
  if summarizer != nil:
      newSummary, err := summarizer(ctx, s.summary, dropped)  // 旧摘要并入新摘要
      if err != nil: newSummary = ""  // 回退：纯截断
      s.summary = newSummary
  else:
      s.summary = ""                  // 无 summarizer：纯截断
  rewriteFile()                       // 原子重写 JSONL
```

### 4.4 摘要的表示与放置（满足 C1、C2）

- 摘要输出为 `schema.Message{Role: RoleUser, Content: "【历史会话摘要】\n" + summary}`。
- **放置规则**：
  - 若 `history` 以 **Assistant** 开头 → 摘要作为第一条独立 User 消息插入 → `User(摘要) → Assistant → ...` 交替成立；
  - 若 `history` 以 **User** 开头 → 将摘要文本**前置合并**进该 User 消息的 Content（`摘要 + "\n\n" + 原内容`），保持消息数不变 → 交替成立。
- 采用 `RoleUser` 而非 `RoleSystem` 的原因：Claude provider 会把所有 system 合并为一条且后者覆盖（C1）；User 消息在两个 provider 下都原样透传。

### 4.5 持久化

现状：`saveToDisk` 依赖未实现的 `helper.AppendLine`（no-op）。

- **补齐** `helper.AppendLine`：`os.OpenFile(path, O_APPEND|O_CREATE|O_WRONLY)` + 写一行 + `\n`。
- **摘要记录格式**：新增一种可区分的 JSONL 记录 `{"summary":"..."}`（无 `role` 键）。加载时按"是否有 role 键"区分消息行与摘要行（C5）。
- **压缩后重写**：压缩改变历史 → 不能只追加，需要原子重写整个文件：
  `write tmp → os.Rename`（会话文件体积有界 ≤ 阈值，重写成本可接受）。
  文件内容 = `[摘要记录?] + 每条原始消息一行`。
- **加载**：`LoadSession(sessionID, workDir string, opts ...Option) (*Session, error)`，逐行读入，`{"summary":...}` 恢复摘要字段，其余恢复 `history`（供进程重启后多轮续聊）。

### 4.6 接口一览

```go
// 变更
func NewSession(sessionID, workDir string, opts ...Option) *Session
func (s *Session) Append(ctx context.Context, msgs ...schema.Message)
func (s *Session) GetWorkingMemory(ctx context.Context) []schema.Message
    // 返回 [摘要(User)] + history；若 history 以 User 开头则合并进第一条

// 新增
func LoadSession(sessionID, workDir string, opts ...Option) (*Session, error)
func LookupContextWindow(model string) int
func WithContextWindow(tokens int) Option
func WithCompressRatio(pct int) Option
func WithModel(model string) Option
func WithSummarizer(fn Summarizer) Option

// 内部
func (s *Session) totalTokens() int
func (s *Session) threshold() int
func (s *Session) compress(ctx context.Context)
func (s *Session) rewriteFile() error
```

## 5. 集成说明（不在本方案范围内）

Session 作为独立组件交付，**不修改 `loop.go` / `EngineProcessor`**。使用方自行接入，参考用法：

```go
sess := engine.NewSession(chatID, workDir,
    engine.WithModel(model),               // 或 engine.WithContextWindow(200_000)
    engine.WithCompressRatio(80),          // 窗口 80% 触发压缩（默认值，可省略）
    engine.WithSummarizer(func(ctx context.Context, old string, msgs []schema.Message) (string, error) {
        // 使用方调用 provider.Generate 生成摘要
    }),
)
// 每轮：sess.Append(userMsg) / sess.Append(resp) / sess.Append(toolResults...)
// 组装请求：history = append([]schema.Message{systemMsg}, sess.GetWorkingMemory(ctx)...)
```

## 6. 模型窗口查表（`LookupContextWindow`）

| 模型特征 | 窗口（token） |
|----------|---------------|
| `glm-4` / `glm-4.5` / `glm-4.6` | 128000 |
| `claude`（sonnet/opus/haiku） | 200000 |
| `gpt-4o` / `gpt-4.1` | 128000 |
| `gpt-4o-mini` | 128000 |
| `deepseek` | 128000 |
| `qwen` | 128000 |
| 未知 | 128000（默认） |

匹配采用子串包含（大小写不敏感），首个命中生效。查表仅作便捷默认，精确窗口请用 `WithContextWindow` 显式传入。

## 7. 测试计划

| 用例 | 覆盖点 |
|------|--------|
| 阈值计算 | `threshold() = window × ratio / 100`；ratio 越界（>100 / <1）被钳制 |
| 阈值触发 | 低于阈值不压缩；追加后超阈值触发一次压缩；不同 ratio（如 50%/80%/100%）下触发点正确 |
| 窗口来源 | 显式 WithContextWindow 优先于 WithModel；未配置用默认 128k；LookupContextWindow 各模型映射 |
| 边界完整性 | 截断边界不落在工具对中间（构造 assistant ToolCalls + 2 条结果，断言整体保留或整体丢弃） |
| 交替性 | 压缩后输出序列无连续 User；history 以 User 开头时摘要正确合并 |
| 摘要生成 | fake summarizer 断言入参（旧摘要 + 被丢弃消息）；summarizer 返回错误 → 回退纯截断 |
| 无 summarizer | 未注入时压缩为纯截断，不 panic |
| 持久化 | Append 追加写盘；压缩后原子重写；LoadSession 恢复摘要 + 历史（round-trip） |
| 单轮超预算 | 单条消息超阈值：不压缩、仅告警、不 panic、不死循环 |

## 8. 实现阶段（评审通过后细化）

1. **P0 基础设施**：实现 `helper.AppendLine`；`Session` 选项式构造（含窗口/比例/查表）+ token 估算 + 阈值计算
2. **P0 压缩核心**：`compress()` 边界算法 + 摘要放置/合并规则 + 回退路径
3. **P0 持久化**：原子重写 + `LoadSession` + round-trip 测试
4. **P0 测试**：7 节全部用例

## 9. 备选方案回顾（已否决）

| 方案 | 否决原因 |
|------|----------|
| 纯截断（滑窗） | 丢失任务目标等早期关键信息，不符合"混合式"决策 |
| 纯摘要 | 无原始细节，调试/续写时信息不足 |
| 固定阈值（如 32k） | 无法适配不同模型窗口，应随窗口按比例触发（用户反馈） |
| 摘要用 RoleSystem | 违反 C1，Claude 下覆盖真实 system |
| 新增 RoleSummary 枚举 | 需改 2 个 provider 的翻译逻辑，破坏面大于收益 |
