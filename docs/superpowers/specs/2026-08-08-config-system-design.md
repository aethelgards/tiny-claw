# tiny-claw 配置系统设计

日期：2026-08-08
状态：已获用户确认

## 背景与目标

tiny-claw 当前的大模型配置散落四处：API Key 硬读 `ZHIPU_API_KEY` 环境变量、BaseURL 硬编码在 `internal/provider/openai.go:15`、Model 走构造函数入参、MaxTokens 硬编码 4096（`claude.go:101`）、enableThinking/workDir 走构造函数入参。

目标：建立一套与 Claude Code 同构的配置系统（层级文件 + 环境变量覆盖），将全部可变模型配置归一化到配置文件。

## 1. 配置文件层级

与 Claude Code 同构的三层配置，JSON 格式：

```
~/.claw/settings.json          # 全局（用户级）
.claw/settings.json            # 项目级
.claw/settings.local.json      # 本地级（应加入 .gitignore）
```

合并语义与 Claude Code 一致：**深合并**（嵌套字段各自覆盖，非整层覆盖），优先级 `local > project > global`。环境变量为最高优先级。

完整优先级链（高 → 低）：

```
环境变量 > .claw/settings.local.json > .claw/settings.json > ~/.claw/settings.json > 内置默认值
```

## 2. Settings 结构

```go
type Settings struct {
    Provider       string `json:"provider"`       // "openai" | "claude"
    Model          string `json:"model"`          // 如 "glm-4.6"
    APIKey         string `json:"apiKey"`
    BaseURL        string `json:"baseURL"`        // 默认 "https://open.bigmodel.cn/api/paas/v4/"
    MaxTokens      int    `json:"maxTokens"`      // 默认 4096
    EnableThinking bool   `json:"enableThinking"` // 默认 false
    WorkDir        string `json:"workDir"`        // 默认 "."
}
```

示例 `settings.json`：

```json
{
  "provider": "openai",
  "model": "glm-4.6",
  "apiKey": "sk-xxx",
  "baseURL": "https://open.bigmodel.cn/api/paas/v4/",
  "maxTokens": 4096,
  "enableThinking": false,
  "workDir": "."
}
```

默认值策略：
- `baseURL` 默认 `https://open.bigmodel.cn/api/paas/v4/`（智谱底座兼容端点，保留现有行为）
- `maxTokens` 默认 4096
- `enableThinking` 默认 false
- `workDir` 默认 `.`
- `provider` 缺省为 `openai`
- `model` 为空 → 启动报错
- `apiKey` 为空 → 启动报错（错误返回，不再 `panic`）

## 3. 环境变量覆盖

全部使用 `CLAW_` 前缀（与 Claude Code 的 `ANTHROPIC_*` 模式同构）：

| 环境变量 | 对应字段 |
|---|---|
| `CLAW_PROVIDER` | provider |
| `CLAW_MODEL` | model |
| `CLAW_API_KEY` | apiKey |
| `CLAW_BASE_URL` | baseURL |
| `CLAW_MAX_TOKENS` | maxTokens |
| `CLAW_ENABLE_THINKING` | enableThinking |
| `CLAW_WORK_DIR` | workDir |

`CLAW_ENABLE_THINKING` 布尔解析规则：`1/true/yes/on` 视为 true（不区分大小写），其余为 false。`CLAW_MAX_TOKENS` 解析失败则忽略该变量并使用配置值。

## 4. 组件改造

### 4.1 新增 `internal/config` 包

- `config.go`：`Settings` 结构体、默认值、`Load() (*Settings, error)`
- `Load()` 流程：
  1. 以默认值初始化
  2. 读 `~/.claw/settings.json`（存在则合并）
  3. 读 `<cwd>/.claw/settings.json`（存在则合并）
  4. 读 `<cwd>/.claw/settings.local.json`（存在则合并）
  5. 应用环境变量覆盖
  6. 校验：`model` 为空 → error；`apiKey` 为空 → error
- 深合并实现：每层 JSON 反序列化到独立 `Settings`，非零字段覆盖当前值（字段级合并，语义等价深合并；当前 Settings 为扁平结构，无嵌套对象）

注意：workDir 本身可能是配置项，因此层级读取路径的基准目录在读取 global 层时无法依赖 workDir。基准目录取当前进程工作目录（`os.Getwd()`），配置文件路径为 `<cwd>/.claw/settings.json` 与 `<cwd>/.claw/settings.local.json`。

### 4.2 `internal/provider` 改造

- 构造器改为接收 `config.Settings`：
  - `NewProvider(settings config.Settings) (LLMProvider, error)` 工厂：按 `Provider` 字段路由到 openai/claude 实现
  - `NewOpenAIProvider(settings config.Settings) (*OpenAIProvider, error)`
  - `NewClaudeProvider(settings config.Settings) (*ClaudeProvider, error)`
- 删除包级 `var baseURL`
- `panic("请设置 ZHIPU_API_KEY 环境变量")` → 返回 `error`（与 `Load()` 校验配合，正常路径不会走到）
- provider 字段为未知值时工厂返回 error

### 4.3 `internal/engine` 改造

`NewAgentEngine` 去掉 `workDir`、`enableThinking` 两个散传入参，改为接收 `config.Settings`：

```go
func NewAgentEngine(p provider.LLMProvider, r tools.Registry, settings config.Settings) *AgentEngine
```

`WorkDir` / `EnableThink` 从 `settings.WorkDir` / `settings.EnableThinking` 读取。

### 4.4 `cmd/claw/main.go` 装配

```go
func main() {
    settings, err := config.Load()
    if err != nil { slog.Error(...); os.Exit(1) }

    p, err := provider.NewProvider(*settings)
    if err != nil { slog.Error(...); os.Exit(1) }

    engine := engine.NewAgentEngine(p, tools.NewRegistry(), *settings)
    if err := engine.Run(ctx, os.Args[1]); err != nil { slog.Error(...); os.Exit(1) }
}
```

## 5. 测试

TDD 先行，`internal/config/config_test.go` 覆盖：

1. 层级合并：global + project + local 字段各自生效，local 覆盖 project
2. 环境变量覆盖：`CLAW_API_KEY` 覆盖配置文件中的 apiKey
3. 默认值：空环境 + 无配置文件时返回默认 baseURL/maxTokens/enableThinking/workDir/provider
4. 校验错误：model 为空返回 error；apiKey 为空返回 error
5. `CLAW_ENABLE_THINKING` 布尔解析：`1/true/yes/on` 大小写不敏感

测试策略：为避免污染真实 `~/.claw/` 与 cwd，`Load()` 拆出内部函数 `loadFrom(globalPath, projectDir string)`，测试传临时目录。

## 6. 范围边界

- 不做系统提示词归一化（保持 `loop.go` 硬编码）
- 不做多 provider 配置块共存（未来需求，YAGNI）
- 不引入外部配置库（标准库 `encoding/json` + `os` 足够，与项目零依赖风格一致）
- `internal/tools` 现有实现不受影响

## 7. 涉及文件

| 文件 | 动作 |
|---|---|
| `internal/config/config.go` | 新增 |
| `internal/config/config_test.go` | 新增 |
| `internal/provider/interface.go` | 新增工厂签名（`NewProvider`） |
| `internal/provider/openai.go` | 改造构造器、删 baseURL |
| `internal/provider/claude.go` | 改造构造器 |
| `internal/engine/loop.go` | 改造构造器 |
| `cmd/claw/main.go` | 装配配置加载 |
| `.gitignore` | 加 `.claw/settings.local.json` |
