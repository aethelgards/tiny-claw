<div align="center">

# 🦞 tiny-claw

**轻量级 AI 编程 Agent 框架** —— 一个用 Go 编写的个人 AI 助手，可在终端或飞书（Lark）中驱动大模型完成编程任务。

![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)
![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)

[✨ 特性](#-特性) · [🚀 快速开始](#-快速开始) · [⚙️ 配置](#️-配置) · [🏗️ 架构](#️-架构总览) · [📚 技能系统](#-技能系统) · [🛡️ 审批机制](#️-审批机制) · [🧠 记忆系统](#-记忆系统) · [📁 项目结构](#-项目结构)

</div>

---

## ✨ 特性

- **双入口模式**
  - 🖥️ **CLI 模式**：`claw <prompt>`，一行命令把任务交给 Agent 在终端中执行
  - 💬 **飞书（Lark）机器人模式**：在聊天中直接对话，支持卡片式审批
- **多模型 Provider**：OpenAI 兼容协议（默认智谱 GLM 底座，兼容 DeepSeek 等）与 Anthropic Claude
- **Agent 循环引擎**：系统提示词构建 → LLM 生成 → **并行工具调用** → 结果回填，循环直至任务完成
- **内置工具集**：读/写/编辑/删除文件、执行 shell 命令、派生子 Agent
- **技能系统（Skill-as-Tool）**：`.claw/skills/` 下的技能按需加载，避免全量注入导致的 token 爆炸
- **危险操作审批**：`rm -r`、`sudo` 等高危命令须经人工确认（终端交互 / 飞书审批卡片），超时 fail-closed
- **多轮会话与上下文压缩**：会话持久化 + LLM 摘要压缩，长对话不失控
- **工程化安全设计**：工作区路径逃逸防护、原子写入、文件读写锁、输出截断、优雅退出
- **可观测性**：结构化日志（slog）、Span 追踪导出、Token 成本追踪
- **可视化 Dashboard**：内置 Web 界面查看会话追踪、成本分析、性能分析、工具统计
- **记忆系统**：持久化长期记忆层，支持关键词 + 向量混合检索、LLM 自动提取、时间衰减归档，让 Agent 跨会话积累经验

## 🚀 快速开始

### 环境要求

- Go 1.26+
- 一个大模型 API Key（OpenAI 兼容端点或 Anthropic）

### 安装

```bash
git clone https://github.com/aethelgards/tiny-claw.git
cd tiny-claw
go build -o claw ./cmd/claw
go build -o larkbot ./cmd/larkbot
```

### 配置

配置按 **默认值 → 全局 → 项目 → 本地 → 环境变量** 的优先级分层合并（与 Claude Code 的 settings 模式同构）：

```
~/.claw/settings.json           # 全局配置（可选）
.claw/settings.json             # 项目配置（可选）
.claw/settings.local.json       # 本地私有配置（可选，通常 gitignore）
CLAW_* 环境变量                 # 最高优先级
```

最小配置示例（`~/.claw/settings.json` 或 `CLAW_*` 环境变量）：

```json
{
  "provider": "openai",
  "model": "glm-4.6",
  "apiKey": "your-api-key",
  "baseURL": "https://open.bigmodel.cn/api/paas/v4/",
  "workDir": "."
}
```

对应环境变量：

```bash
export CLAW_PROVIDER=openai
export CLAW_MODEL=glm-4.6
export CLAW_API_KEY=your-api-key
export CLAW_BASE_URL=https://open.bigmodel.cn/api/paas/v4/
export CLAW_WORK_DIR=.
```

### 运行 CLI

```bash
./claw "给这个项目添加单元测试"
```

Agent 会自主读取代码、修改文件、执行命令，直到完成任务。遇到高危命令（如 `rm -r`）会暂停并请求你的确认：

```
⚠️ 高危操作审批请求
工具: bash
参数: rm -rf build/
任务 ID: 123456
允许执行? (y/N): 
```

### 运行飞书机器人

1. 在[飞书开放平台](https://open.feishu.cn/)创建应用，获取 `App ID` 与 `App Secret`
2. 开启机器人能力，并添加 `im:message` 等事件订阅（WebSocket 长连接方式，无需公网回调地址）
3. 配置并启动：

```json
{
  "larkAppId": "cli_xxxxxxxx",
  "larkAppSecret": "xxxxxxxxxxxxxxxx",
  "larkChannelSize": 64
}
```

```bash
./larkbot
```

之后在飞书中私聊或群聊 @机器人 即可对话。危险命令会以**审批卡片**的形式推送到会话中，点击「允许/拒绝」完成审批。

### 运行可视化 Dashboard

```bash
# 构建前端（首次运行或前端代码变更时）
./scripts/build-dashboard.sh

# 启动 Dashboard 服务器
./claw serve
```

启动后访问 `http://localhost:8080` 查看 Dashboard。

**Dashboard 功能：**
- **Dashboard** - 概览统计卡片 + 最近会话列表
- **Sessions** - 会话列表（分页、过滤、搜索）
- **Session Detail** - 会话详情（消息流、工具调用树、Span 瀑布图）
- **Cost Analytics** - 成本趋势、模型成本分布、TOP 会话
- **Performance Analytics** - 耗时趋势、成功率、耗时分布
- **Tool Analytics** - 工具调用次数、平均耗时、错误率

**截图预览：**

![会话摘要](docs/session_summary.jpeg)

![会话 Span 瀑布图](docs/sessson_span.jpeg)

![会话工具使用详情](docs/session_tool_use.jpeg)

![全局工具使用统计](docs/global_tool_use_statistic.jpeg)

**常用参数：**
```bash
./claw serve                    # 默认端口 8080，数据目录 .claw/traces
./claw serve --port 3000        # 自定义端口
./claw serve -d /path/to/traces # 自定义数据目录
```

## ⚙️ 配置

| 字段 | JSON 键 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|---|
| Provider | `provider` | `CLAW_PROVIDER` | `openai` | `openai` \| `claude` |
| 模型 | `model` | `CLAW_MODEL` | — | 必填，如 `glm-4.6` |
| API Key | `apiKey` | `CLAW_API_KEY` | — | 必填 |
| API 端点 | `baseURL` | `CLAW_BASE_URL` | 智谱兼容端点 | OpenAI 兼容地址 |
| 最大 Token | `maxTokens` | `CLAW_MAX_TOKENS` | `4096` | 单次生成上限 |
| 慢思考 | `enableThinking` | `CLAW_ENABLE_THINKING` | `false` | 是否开启思考过程 |
| 工作目录 | `workDir` | `CLAW_WORK_DIR` | `.` | Agent 操作的工作区 |
| 计划模式 | `planMode` | — | `false` | 影响系统提示词策略 |
| 飞书 App ID | `larkAppId` | `CLAW_LARK_APP_ID` | 空 | 仅 larkbot 模式 |
| 飞书 App Secret | `larkAppSecret` | `CLAW_LARK_APP_SECRET` | 空 | 仅 larkbot 模式 |
| 队列容量 | `larkChannelSize` | `CLAW_LARK_CHANNEL_SIZE` | `64` | 消息队列缓冲上限 |
| 审批超时 | `approvalTimeout` | `CLAW_APPROVAL_TIMEOUT` | `5m` | Go duration 字符串 |
| 日志 | `log` | — | 见下方 | `level` / `format` / `logDir` 等 |
| Embedding 模型 | `embedding.model` | `CLAW_EMBEDDING_MODEL` | — | 向量模型名，如 `text-embedding-3-small` |
| Embedding API Key | `embedding.apiKey` | `CLAW_EMBEDDING_API_KEY` | — | 不填则复用主 provider key |
| Embedding 端点 | `embedding.baseURL` | `CLAW_EMBEDDING_BASE_URL` | — | OpenAI 兼容端点 |
| Embedding 超时 | `embedding.timeout` | — | `5s` | Go duration 字符串 |

## 🔧 内置工具

| 工具 | 说明 | 安全约束 |
|---|---|---|
| `read_file` | 读取文件内容 | 限 64 MiB，路径必须位于工作区内 |
| `write_file` | 写入文件 | 原子写入（临时文件 + rename），工作区路径约束 |
| `edit_file` | 精确字符串替换编辑 | 工作区路径约束 + 文件锁 |
| `delete_file` | 删除文件 | 工作区路径约束 |
| `bash` | 执行 shell 命令 | 默认 120s 超时，输出截断 8 KB，全局文件锁 |
| `spawn_subagent` | 派生子 Agent 并行处理 | 复用 provider 配置 |
| 技能工具 | 按需加载 `.claw/skills/` 技能正文 | 正文截断 32 KB |

**并发安全**：进程级读写锁（`internal/tools/filelock.go`）保证同一路径的读/写串行化、不同路径并行化；`bash` 因可能触碰任意文件而持有全局排他锁。

## 🏗️ 架构总览

```mermaid
flowchart LR
    subgraph Entry["入口 (cmd/)"]
        CLI["claw<br/>CLI 模式"]
        BOT["larkbot<br/>飞书机器人"]
    end

    subgraph Core["核心 (internal/)"]
        CFG["config<br/>分层配置"]
        PRV["provider<br/>OpenAI / Claude<br/>Embedding"]
        ENG["engine<br/>Agent 循环"]
        TOOL["tools<br/>工具注册表 + 中间件"]
        CTX["context<br/>会话 / 提示词 / 恢复"]
        MEM["memory<br/>长期记忆<br/>提取 / 检索 / 归档"]
        OBS["observability<br/>Span 追踪 / 成本<br/>会话持久化"]
    end

    subgraph Gateway["飞书网关 (internal/gateway/lark)"]
        WS["WebSocket 事件"]
        Q["消息队列<br/>(有界 channel + 去重)"]
        W["Worker<br/>串行消费"]
        R["LarkReporter<br/>进度回传"]
    end

    subgraph Dashboard["Dashboard (internal/dashboard)"]
        API["REST API<br/>会话 / 追踪 / 统计"]
        SPA["React SPA<br/>go:embed 嵌入"]
    end

    CLI --> CFG
    BOT --> CFG
    CFG --> PRV
    CLI --> ENG
    BOT --> WS --> Q --> W --> ENG
    ENG --> TOOL
    ENG --> CTX
    ENG --> PRV
    ENG --> OBS
    CTX --> MEM
    MEM --> PRV
    OBS --> API
    SPA --> API
    W --> R
```

**Agent 循环**（`internal/engine/loop.go`）：

```mermaid
flowchart TD
    A["构建系统提示词<br/>(promptCore + AGENT.md + 技能策略<br/>+ <b>长期记忆注入</b>)"] --> B["LLM Generate<br/>(携带可用工具定义)"]
    B --> C{"有工具调用?"}
    C -- 否 --> D["输出最终结果<br/>完成 ✅"]
    C -- 是 --> E["并行执行工具<br/>(goroutine + 文件锁)"]
    E --> H["记录 Span + 耗时<br/>(observability.Storage)"]
    H --> F["失败工具错误分析<br/>+ 恢复提示注入"]
    F --> G["结果回填上下文"]
    G --> B

    D --> I["会话压缩检查<br/>(token 超阈值?)"]
    I -- 是 --> J["LLM 摘要压缩<br/>+ <b>SessionHook 自动提取记忆</b>"]
    I -- 否 --> K["结束"]
    J --> K
```

**飞书异步管道**：事件回调只做「解析 → 过滤 → 去重 → 入队」，**微秒级返回**，绝不阻塞 WebSocket；有界队列（默认 64）满则丢弃并记日志，单 worker 串行消费保证全量有序；收到 `SIGINT`/`SIGTERM` 时处理完当前消息再优雅退出。

## 📚 技能系统

技能以 `技能名/SKILL.md` 的形式组织在 `.claw/skills/` 下，frontmatter 声明 `name` 与 `description`：

```markdown
---
name: tdd-workflow
description: 编写新功能或修复 bug 时使用，强制测试驱动开发
---

# TDD 工作流
（完整执行指南...）
```

设计要点：

- **Skill-as-Tool**：每个技能注册为一个工具，模型每轮只看到「技能名 + description」（O(N×描述)），仅在任务匹配时才调用工具加载正文 —— 相比全量注入（O(N×正文)）避免了 token 爆炸
- **零架构侵入**：复用引擎已有的 tool call → 执行 → 回填循环
- **可靠性**：技能正文 32 KB 截断保护，坏格式技能跳过并告警，重名技能跳过不 panic

## 🛡️ 审批机制

命中高危模式（`rm -r`、`sudo`、`drop`、重定向写 `.go` 文件等）的 `bash` 调用会被 `ApprovalMiddleware` 拦截：

- **CLI 模式**：终端交互 `[y/N]` 确认；非交互 stdin 一律拒绝
- **飞书模式**：审批卡片推送到会话，仅发起者本人可审批
- **超时 fail-closed**：默认 5 分钟（`approvalTimeout` 可调），超时自动拒绝；审批通知发送失败同样拒绝执行

## 🧠 会话与上下文管理

- **多轮会话**：`Session` 持久化到磁盘（`.claw/sessions/`），跨请求保留工作记忆
- **上下文压缩**：会话达到阈值（基于模型 context window × 压缩比例）时，由 LLM 摘要器将旧对话压缩为中文摘要，防止长对话撑爆上下文；摘要失败自动回退为纯截断
- **成本追踪**：`CostTracker` 记录每轮 token 用量与费用
- **错误自愈**：`RecoveryManager` 分析失败的工具调用并向模型注入修复提示；连续失败的指纹去重避免重复注入

## 🧠 记忆系统

tiny-claw 内置了一套持久化的长期记忆层（`internal/memory/`），让 Agent 能跨会话积累和复用经验，而非每次从零开始。

### 设计哲学

传统 Agent 的上下文窗口是「金鱼记忆」——会话结束即遗忘，下次对话从零开始。tiny-claw 的记忆系统借鉴了 **RAG（检索增强生成）** 的思路，将长期记忆从上下文窗口中解耦出来：

```
┌─────────────────────────────────────────────────────┐
│                  上下文窗口（有限）                    │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │
│  │ 系统提示词 │  │ 对话历史  │  │ 注入的记忆（≤400t）│  │
│  └──────────┘  └──────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────┘
                         ↑ MemoryInjector.Recent()
                         │
              ┌──────────┴──────────┐
              │   长期记忆（无限）    │
              │  JSONL 文件持久化    │
              │  关键词 + 向量混合检索 │
              └─────────────────────┘
```

**核心权衡**：

- **解耦而非堆叠**：记忆不占用上下文窗口配额，而是按需检索注入。Agent 的有效工作记忆 = 上下文窗口 + 无限外挂记忆
- **零配置降级**：不配置 embedding 时自动回退为纯关键词检索，记忆功能完整可用，只是少了语义模糊匹配
- **异步无阻塞**：记忆提取在 goroutine 中执行，不持有会话锁，不阻塞 Agent 主循环
- **归档不删除**：过期记忆移至 `archive/` 目录而非物理删除，可随时恢复

### 记忆类型与作用域

| 类型 | 文件 | 作用域 | 用途 |
|---|---|---|---|
| `preferences` | `preferences.jsonl` | **全局**（`~/.claw/memory/`） | 用户偏好：编码风格、语言习惯等，跨项目共享 |
| `project` | `project.jsonl` | **项目**（`.claw/memory/`） | 架构决策、技术栈、目录约定 |
| `errors` | `errors.jsonl` | 项目 | 错误模式与解决方案，避免重复踩坑 |
| `tools` | `tools.jsonl` | 项目 | 特定任务下有效的工具调用策略 |

每条记忆包含：确定性 ID（type+content 的 SHA-256 前 16 位）、内容、可选向量嵌入、来源追踪（自动/显式）、访问计数和时间戳。

### 工作流程

记忆系统有三条数据通路，各司其职：

**写入通路 A — LLM 自动提取**（被动积累）：
```
会话压缩触发 → 克隆被丢弃的消息 → SessionHook.Extract() [异步 goroutine, 无锁]
    → LLM 从即将遗忘的对话中提取结构化记忆 → 批量向量化 → store.Save()
```

**写入通路 B — Agent 显式操作**（主动积累）：
```
Agent 判断有价值的信息 → 调用 save_memory → inferType() 自动推断类型
    → scope 按类型路由（preferences → 全局，其余 → 项目） → 原子写入 JSONL
```

**读取通路 — 上下文注入**（按需检索）：
```
PromptComposer.Build() → MemoryInjector.Recent()
    → 项目记忆优先 + 全局记忆合并 → 关键词/向量混合检索 top-50
    → token 预算截断（默认 400 tokens） → 注入系统提示词 "长期记忆" 区块
```

> **设计亮点**：注入时不调用 `Touch()` —— 避免「富者愈富」正反馈循环：被注入的记忆不会因访问计数增加而更频繁地被注入，保证记忆池的多样性。

### 检索策略

**混合关键词 + 向量检索**（`MemoryStore.Recall()`）：

1. **关键词优先**：大小写不敏感的 token 匹配，带词边界评分
   - ASCII 文本：词边界加分（`word_boundary_score = 1.0 + 0.5 × boundary_hits`）
   - 中文文本：独立字单元检测（非 ASCII 字符逐字切分，无分词依赖）
   - 排序：关键词得分降序 → 时间降序
2. **向量回退**：当关键词结果不足 `limit` 且 embedder 已配置时，启用余弦相似度检索
   - L2 归一化向量，阈值 `minScore`（默认 0.35）
   - 补充填充至 `limit` 槽位

**确定性 ID 设计**：每条记忆的 ID = `SHA256(type + content)[:16]`，同类型同内容 → 同 ID → 幂等 upsert，天然去重。

### Agent 工具

Agent 可通过以下工具直接操作记忆：

| 工具 | 说明 |
|---|---|
| `save_memory` | 保存记忆，type 可自动推断，scope 按类型自动路由 |
| `recall_memory` | 按关键词检索记忆（空查询 = 最近），返回结果并更新访问计数 |
| `forget_memory` | 按 ID 删除指定记忆 |

### 自动提取与压缩

当会话上下文压缩时，被丢弃的消息会异步触发 `SessionHook.Extract()` —— LLM 从即将遗忘的对话中提取有价值的记忆，自动保存。整个过程在会话锁外执行，带 panic 恢复，不影响主流程。

### 衰减归档

记忆不是永恒的 —— 不常用的记忆会被自然淘汰，保持记忆池的信噪比。

**评分公式**：`score = accessCount × (1 / (1 + 0.05 × daysSinceLastAccess))`

- 被频繁访问的记忆得分高，长期驻留
- 从未访问的记忆随时间衰减，30 天后归档
- 归档 ≠ 删除：移至 `archive/<type>.jsonl`，可随时恢复

**归档策略**：
| 条件 | 动作 |
|---|---|
| `accessCount == 0` 且 `age > 30d` | 直接归档 |
| `score < threshold` 且 `lastAccess > 30d` | 归档 |
| 其他 | 保留 |

### 配置

启用向量检索需配置 embedding provider：

```json
{
  "embedding": {
    "model": "text-embedding-3-small",
    "apiKey": "your-api-key",
    "baseURL": "https://api.openai.com/v1/",
    "timeout": "5s"
  }
}
```

不配置 embedding 时，系统仅使用关键词检索，记忆功能仍完全可用。

## 📁 项目结构

```
├── cmd/
│   ├── claw/                  # CLI 模式入口
│   └── larkbot/               # 飞书机器人入口
├── internal/
│   ├── approval/              # 审批管理器、中间件、终端/卡片 reporter
│   ├── config/                # 分层配置加载 + slog 初始化
│   ├── context/               # 会话、提示词组合器、技能加载、错误恢复
│   ├── dashboard/             # Dashboard HTTP 服务器 + API（go:embed SPA）
│   ├── domainerr/             # 结构化领域错误（ToolNotFound、PermissionDeny）
│   ├── engine/                # Agent 循环、摘要器、子 Agent、提醒注入
│   ├── gateway/lark/          # 飞书网关：消息解析、队列、worker、审批卡片
│   ├── helper/                # JSON / Map 工具
│   ├── memory/                # 长期记忆：提取、检索、注入、衰减归档
│   ├── observability/         # 成本追踪、会话/Span 持久化存储
│   ├── provider/              # OpenAI 兼容 / Claude Provider / Embedding
│   ├── reporter/              # Reporter 接口
│   ├── schema/                # 消息 / 工具调用 / 工具定义模型
│   ├── subagent/              # 子 Agent 相关
│   ├── tools/                 # 工具注册表、内置工具、文件锁
│   └── trace/                 # Span 追踪与导出
```

## 🧪 测试

```bash
go build ./...   # 编译检查
go vet ./...     # 静态检查
go test ./...    # 运行测试
```

## 📄 License

[MIT](LICENSE) © 2026 aethelgards
