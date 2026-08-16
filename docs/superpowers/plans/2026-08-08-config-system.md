# tiny-claw 配置系统实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 tiny-claw 建立与 Claude Code 同构的配置系统（`~/.claw/` + 项目 `.claw/` 三层 JSON 合并 + `CLAW_` 环境变量覆盖），将散落的模型配置（apiKey/baseURL/model/maxTokens/enableThinking/workDir）全部归一化到配置文件。

**Architecture:** 新增 `internal/config` 包负责层级加载与合并（环境变量 > local > project > global > 内置默认值），`provider`/`engine` 构造器改为接收 `config.Settings`，`cmd/claw/main.go` 装配加载流程。系统提示词保持硬编码（spec 范围边界）。

**Tech Stack:** Go 1.26.3 标准库（`encoding/json`、`os`、`path/filepath`、`strconv`、`strings`）；openai-go v3（`MaxTokens param.Opt[int64]`，用 `param.NewOpt[int64]` 赋值）；anthropic-sdk-go（`MaxTokens int64`）。不引入任何新依赖。

## Global Constraints

- 不引入外部配置库；只用标准库
- `Load()` 内部实现细节：读取基准目录为进程 cwd（`os.Getwd()`），**不是** `settings.WorkDir`（workDir 本身是配置项，读取层级时不可依赖）
- 合并语义：**按键存在性合并**（presence-based）——某层显式写了 `"enableThinking": false` 就必须覆盖上层 `true`（不能用"非零值才覆盖"，否则 bool false 永远无法覆盖）
- provider 工厂遇到未知 provider 字符串 → 返回 error（支持 `openai`、`claude`）
- `apiKey`/`model` 为空 → 启动报错，**不再 panic**
- **本仓库不是 git 仓库**：所有任务末尾的验证用 `gofmt -l internal/ cmd/`、`go build ./...`、`go test ./...`，不执行 git commit
- 每个任务结束必须 `go build ./...` + `go test ./...` 全绿才能进入下一任务

---

### Task 1: `internal/config` — Settings 结构体 + 默认值 + 层级合并加载

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `type Settings struct { Provider, Model, APIKey, BaseURL string; MaxTokens int; EnableThinking bool; WorkDir string }`（全部带 `json` tag，key 为 camelCase：`provider`/`model`/`apiKey`/`baseURL`/`maxTokens`/`enableThinking`/`workDir`）
- Produces: `func defaultSettings() Settings`（`Provider="openai"`、`BaseURL="https://open.bigmodel.cn/api/paas/v4/"`、`MaxTokens=4096`、`EnableThinking=false`、`WorkDir="."`）
- Produces: `func applyLayer(s *Settings, data []byte) error`（按键存在性合并，键不存在于该层则跳过）
- Produces: `func loadFrom(globalPath, projectDir string) (*Settings, error)`（内部函数，测试传临时目录；globalPath 是全局配置文件的完整路径，projectDir 是项目目录）
- Consumes: 无（本任务不依赖其他包）

- [ ] **Step 1: 写失败的测试**

创建 `internal/config/config_test.go`：

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func TestLoadFromHierarchy(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	proj := filepath.Join(root, "proj")

	// global：provider + maxTokens
	mustWrite(t, filepath.Join(home, ".claw", "settings.json"),
		`{"provider":"openai","apiKey":"global-key","maxTokens":8192}`)
	// project：model 覆盖
	mustWrite(t, filepath.Join(proj, ".claw", "settings.json"),
		`{"model":"glm-project"}`)
	// local：apiKey 覆盖 + enableThinking 打开
	mustWrite(t, filepath.Join(proj, ".claw", "settings.local.json"),
		`{"apiKey":"local-key","enableThinking":true}`)

	s, err := loadFrom(filepath.Join(home, ".claw", "settings.json"), proj)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if s.Provider != "openai" {
		t.Errorf("Provider = %q, want openai", s.Provider)
	}
	if s.Model != "glm-project" {
		t.Errorf("Model = %q, want glm-project", s.Model)
	}
	if s.APIKey != "local-key" {
		t.Errorf("APIKey = %q, want local-key", s.APIKey)
	}
	if s.MaxTokens != 8192 {
		t.Errorf("MaxTokens = %d, want 8192", s.MaxTokens)
	}
	if !s.EnableThinking {
		t.Error("EnableThinking = false, want true")
	}
	if s.BaseURL != "https://open.bigmodel.cn/api/paas/v4/" {
		t.Errorf("BaseURL = %q, want default zhipu base", s.BaseURL)
	}
	if s.WorkDir != "." {
		t.Errorf("WorkDir = %q, want .", s.WorkDir)
	}
}

func TestExplicitFalseOverrides(t *testing.T) {
	proj := t.TempDir()
	mustWrite(t, filepath.Join(proj, ".claw", "settings.json"),
		`{"enableThinking":true}`)
	mustWrite(t, filepath.Join(proj, ".claw", "settings.local.json"),
		`{"enableThinking":false}`)

	s, err := loadFrom(filepath.Join(proj, "..", "nope.json"), proj)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if s.EnableThinking {
		t.Error("EnableThinking = true, want false (explicit false must override)")
	}
}

func TestDefaultsWhenNoFiles(t *testing.T) {
	root := t.TempDir()
	s, err := loadFrom(filepath.Join(root, "missing", "settings.json"), root)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if s.Provider != "openai" || s.BaseURL != "https://open.bigmodel.cn/api/paas/v4/" ||
		s.MaxTokens != 4096 || s.EnableThinking || s.WorkDir != "." {
		t.Errorf("defaults wrong: %+v", s)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/erik/GoCustome/tiny-claw && go test ./internal/config/... 2>&1`
Expected: FAIL（`undefined: loadFrom` 或 package 不存在）

- [ ] **Step 3: 实现最小代码**

创建 `internal/config/config.go`：

```go
// internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings 与 Claude Code 的 settings.json 同构的扁平配置结构。
type Settings struct {
	Provider       string `json:"provider"`       // "openai" | "claude"
	Model          string `json:"model"`          // 如 "glm-4.6"
	APIKey         string `json:"apiKey"`
	BaseURL        string `json:"baseURL"`        // 默认智谱底座兼容端点
	MaxTokens      int    `json:"maxTokens"`      // 默认 4096
	EnableThinking bool   `json:"enableThinking"` // 默认 false
	WorkDir        string `json:"workDir"`        // 默认 "."
}

const defaultBaseURL = "https://open.bigmodel.cn/api/paas/v4/"

func defaultSettings() Settings {
	return Settings{
		Provider:  "openai",
		BaseURL:   defaultBaseURL,
		MaxTokens: 4096,
		WorkDir:   ".",
	}
}

// applyLayer 按键存在性合并一层配置：data 中出现的键覆盖 s 中对应字段。
// 不能使用"非零值才覆盖"——否则显式 false / 0 / 空串无法覆盖上层。
func applyLayer(s *Settings, data []byte) error {
	var layer map[string]json.RawMessage
	if err := json.Unmarshal(data, &layer); err != nil {
		return err
	}
	if v, ok := layer["provider"]; ok {
		if err := json.Unmarshal(v, &s.Provider); err != nil {
			return err
		}
	}
	if v, ok := layer["model"]; ok {
		if err := json.Unmarshal(v, &s.Model); err != nil {
			return err
		}
	}
	if v, ok := layer["apiKey"]; ok {
		if err := json.Unmarshal(v, &s.APIKey); err != nil {
			return err
		}
	}
	if v, ok := layer["baseURL"]; ok {
		if err := json.Unmarshal(v, &s.BaseURL); err != nil {
			return err
		}
	}
	if v, ok := layer["maxTokens"]; ok {
		if err := json.Unmarshal(v, &s.MaxTokens); err != nil {
			return err
		}
	}
	if v, ok := layer["enableThinking"]; ok {
		if err := json.Unmarshal(v, &s.EnableThinking); err != nil {
			return err
		}
	}
	if v, ok := layer["workDir"]; ok {
		if err := json.Unmarshal(v, &s.WorkDir); err != nil {
			return err
		}
	}
	return nil
}

// loadFrom 按 global → project → local 顺序合并配置文件。
// 基准目录是 projectDir（通常为进程 cwd），不可依赖 settings.WorkDir。
func loadFrom(globalPath, projectDir string) (*Settings, error) {
	s := defaultSettings()
	paths := []string{
		globalPath,
		filepath.Join(projectDir, ".claw", "settings.json"),
		filepath.Join(projectDir, ".claw", "settings.local.json"),
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("读取配置 %s: %w", p, err)
		}
		if err := applyLayer(&s, data); err != nil {
			return nil, fmt.Errorf("解析配置 %s: %w", p, err)
		}
	}
	return &s, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/erik/GoCustome/tiny-claw && go test ./internal/config/... -v`
Expected: 3 个测试全部 PASS

- [ ] **Step 5: 验证**

Run: `cd /Users/erik/GoCustome/tiny-claw && gofmt -l internal/config/ && go build ./... && go test ./...`
Expected: `gofmt` 无输出、build 通过、`internal/tools` 21 个测试仍全绿

---

### Task 2: `internal/config` — 环境变量覆盖 + 校验 + 公开 `Load()`

**Files:**
- Modify: `internal/config/config.go`（追加函数）
- Test: `internal/config/config_test.go`（追加测试）

**Interfaces:**
- Consumes: Task 1 的 `Settings`、`defaultSettings()`、`loadFrom()`
- Produces: `func applyEnv(s *Settings)`（读 `CLAW_PROVIDER`/`CLAW_MODEL`/`CLAW_API_KEY`/`CLAW_BASE_URL`/`CLAW_MAX_TOKENS`/`CLAW_ENABLE_THINKING`/`CLAW_WORK_DIR`；`CLAW_MAX_TOKENS` 解析失败则忽略；布尔解析 `1/true/yes/on` 不区分大小写 → true，其余 → false）
- Produces: `func validate(s *Settings) error`（`Model` 为空 → error；`APIKey` 为空 → error）
- Produces: `func Load() (*Settings, error)`（公开入口：`os.UserHomeDir()` + `os.Getwd()` → `loadFrom` → `applyEnv` → `validate`）

- [ ] **Step 1: 写失败的测试**

追加到 `internal/config/config_test.go`：

```go
func TestApplyEnvOverrides(t *testing.T) {
	t.Setenv("CLAW_PROVIDER", "claude")
	t.Setenv("CLAW_MODEL", "env-model")
	t.Setenv("CLAW_API_KEY", "env-key")
	t.Setenv("CLAW_BASE_URL", "https://example.com/v1")
	t.Setenv("CLAW_MAX_TOKENS", "2048")
	t.Setenv("CLAW_ENABLE_THINKING", "yes")
	t.Setenv("CLAW_WORK_DIR", "/tmp/x")

	s := defaultSettings()
	applyEnv(&s)
	if s.Provider != "claude" || s.Model != "env-model" || s.APIKey != "env-key" ||
		s.BaseURL != "https://example.com/v1" || s.MaxTokens != 2048 ||
		!s.EnableThinking || s.WorkDir != "/tmp/x" {
		t.Errorf("applyEnv wrong: %+v", s)
	}
}

func TestApplyEnvInvalidMaxTokensIgnored(t *testing.T) {
	t.Setenv("CLAW_MAX_TOKENS", "abc")
	s := defaultSettings()
	applyEnv(&s)
	if s.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want 4096 (invalid env ignored)", s.MaxTokens)
	}
}

func TestParseBool(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "Yes", "yes", "on", "ON"} {
		if !parseBool(v) {
			t.Errorf("parseBool(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"0", "false", "no", "off", "", "2", "maybe"} {
		if parseBool(v) {
			t.Errorf("parseBool(%q) = true, want false", v)
		}
	}
}

func TestValidate(t *testing.T) {
	s := defaultSettings()
	if err := validate(&s); err == nil {
		t.Fatal("expected error when model and apiKey are empty")
	}
	s.Model = "glm-4.6"
	if err := validate(&s); err == nil {
		t.Fatal("expected error when apiKey is empty")
	}
	s.APIKey = "sk-xxx"
	if err := validate(&s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/erik/GoCustome/tiny-claw && go test ./internal/config/... -run "TestApplyEnv|TestParseBool|TestValidate" 2>&1`
Expected: FAIL（`undefined: applyEnv` / `undefined: parseBool` / `undefined: validate`）

- [ ] **Step 3: 实现**

追加到 `internal/config/config.go`（并给 import 块加 `strconv`、`strings`）：

```go
// applyEnv 应用环境变量覆盖（最高优先级，与 Claude Code 的 ANTHROPIC_* 模式同构）。
func applyEnv(s *Settings) {
	if v := os.Getenv("CLAW_PROVIDER"); v != "" {
		s.Provider = v
	}
	if v := os.Getenv("CLAW_MODEL"); v != "" {
		s.Model = v
	}
	if v := os.Getenv("CLAW_API_KEY"); v != "" {
		s.APIKey = v
	}
	if v := os.Getenv("CLAW_BASE_URL"); v != "" {
		s.BaseURL = v
	}
	if v := os.Getenv("CLAW_MAX_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.MaxTokens = n
		}
	}
	if v := os.Getenv("CLAW_ENABLE_THINKING"); v != "" {
		s.EnableThinking = parseBool(v)
	}
	if v := os.Getenv("CLAW_WORK_DIR"); v != "" {
		s.WorkDir = v
	}
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// validate 校验必填项。
func validate(s *Settings) error {
	if s.Model == "" {
		return fmt.Errorf("model 未配置：请设置 CLAW_MODEL 或配置文件中的 model 字段")
	}
	if s.APIKey == "" {
		return fmt.Errorf("apiKey 未配置：请设置 CLAW_API_KEY 或配置文件中的 apiKey 字段")
	}
	return nil
}

// Load 加载配置：默认值 → 全局 → 项目 → 本地 → 环境变量 → 校验。
func Load() (*Settings, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户主目录: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("获取工作目录: %w", err)
	}
	s, err := loadFrom(filepath.Join(home, ".claw", "settings.json"), cwd)
	if err != nil {
		return nil, err
	}
	applyEnv(s)
	if err := validate(s); err != nil {
		return nil, err
	}
	return s, nil
}
```

注意：import 块更新为

```go
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/erik/GoCustome/tiny-claw && go test ./internal/config/... -v`
Expected: 7 个测试全部 PASS（Task 1 的 3 个 + 本任务的 4 个）

- [ ] **Step 5: 验证**

Run: `cd /Users/erik/GoCustome/tiny-claw && gofmt -l internal/config/ && go build ./... && go test ./...`
Expected: `gofmt` 无输出、build 通过、全部测试绿

---

### Task 3: `internal/provider` — 构造器接收 Settings + 工厂函数 + 删 baseURL/panic

**Files:**
- Modify: `internal/provider/interface.go`（追加 `NewProvider` 工厂）
- Modify: `internal/provider/openai.go`（构造器改造、删 `var baseURL`、删 `os` import、`MaxTokens` 接线）
- Modify: `internal/provider/claude.go`（构造器改造、删 `os` import、`MaxTokens` 接线）
- Create: `internal/provider/provider_test.go`

**Interfaces:**
- Consumes: Task 1/2 的 `config.Settings`
- Produces: `func NewProvider(settings config.Settings) (LLMProvider, error)`（按 `settings.Provider` 路由；未知值 → error）
- Produces: `func NewOpenAIProvider(settings config.Settings) (*OpenAIProvider, error)`
- Produces: `func NewClaudeProvider(settings config.Settings) (*ClaudeProvider, error)`
- 删除：包级 `var baseURL`；`NewOpenAIProvider(model string)` / `NewClaudeProvider(model string)` 旧签名；`os` 依赖；`panic`

- [ ] **Step 1: 写失败的测试**

创建 `internal/provider/provider_test.go`：

```go
package provider

import (
	"testing"

	"github.com/aethelgards/tiny-claw/internal/config"
)

func TestNewProviderRouting(t *testing.T) {
	base := config.Settings{Model: "glm-4.6", APIKey: "sk-x"}

	p, err := NewProvider(base)
	if err != nil {
		t.Fatalf("NewProvider(openai default): %v", err)
	}
	if _, ok := p.(*OpenAIProvider); !ok {
		t.Errorf("default provider type = %T, want *OpenAIProvider", p)
	}

	base.Provider = "claude"
	p, err = NewProvider(base)
	if err != nil {
		t.Fatalf("NewProvider(claude): %v", err)
	}
	if _, ok := p.(*ClaudeProvider); !ok {
		t.Errorf("claude provider type = %T, want *ClaudeProvider", p)
	}
}

func TestNewProviderUnknown(t *testing.T) {
	_, err := NewProvider(config.Settings{Provider: "gemini", Model: "x", APIKey: "y"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNewOpenAIProviderMissingAPIKey(t *testing.T) {
	_, err := NewOpenAIProvider(config.Settings{Model: "glm-4.6"})
	if err == nil {
		t.Fatal("expected error when apiKey is empty")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/erik/GoCustome/tiny-claw && go test ./internal/provider/... -run TestNewProvider 2>&1`
Expected: FAIL（`undefined: NewProvider` / `undefined: NewOpenAIProvider` 签名不匹配）

- [ ] **Step 3: 实现**

**`internal/provider/interface.go`** 改为：

```go
package provider

import (
	"context"
	"fmt"

	"github.com/aethelgards/tiny-claw/internal/config"
	"github.com/aethelgards/tiny-claw/internal/schema"
)

type LLMProvider interface {
	Generate(ctx context.Context, messages []schema.Message, availableTools []schema.ToolDefinition) (*schema.Message, error)
}

// NewProvider 按 settings.Provider 路由到对应实现。
// 空 provider 视为 "openai"（与 spec 的缺省策略一致）。
func NewProvider(settings config.Settings) (LLMProvider, error) {
	switch settings.Provider {
	case "", "openai":
		return NewOpenAIProvider(settings)
	case "claude":
		return NewClaudeProvider(settings)
	default:
		return nil, fmt.Errorf("未知 provider %q（支持: openai, claude）", settings.Provider)
	}
}
```

**`internal/provider/openai.go`** 修改（import 去掉 `os`，删除第 15 行 `var baseURL`）：

```go
// internal/provider/openai.go
package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/aethelgards/tiny-claw/internal/config"
	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"
)

type OpenAIProvider struct {
	client    openai.Client
	model     string
	maxTokens int
}

func NewOpenAIProvider(settings config.Settings) (*OpenAIProvider, error) {
	if settings.APIKey == "" {
		return nil, errors.New("OpenAIProvider: apiKey 不能为空")
	}
	return &OpenAIProvider{
		client: openai.NewClient(
			option.WithAPIKey(settings.APIKey),
			option.WithBaseURL(settings.BaseURL),
		),
		model:     settings.Model,
		maxTokens: settings.MaxTokens,
	}, nil
}
```

`Generate` 中请求构建段（原第 100-103 行）改为：

```go
	params := openai.ChatCompletionNewParams{
		Model:    p.model,
		Messages: openaiMsgs,
	}
	if p.maxTokens > 0 {
		params.MaxTokens = param.NewOpt[int64](int64(p.maxTokens))
	}
```

其余 `Generate` 逻辑不变。

**`internal/provider/claude.go`** 修改（import 去掉 `os`）：

```go
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aethelgards/tiny-claw/internal/config"
	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type ClaudeProvider struct {
	client    anthropic.Client
	model     string
	maxTokens int64
}

func NewClaudeProvider(settings config.Settings) (*ClaudeProvider, error) {
	if settings.APIKey == "" {
		return nil, errors.New("ClaudeProvider: apiKey 不能为空")
	}
	return &ClaudeProvider{
		client: anthropic.NewClient(
			option.WithAPIKey(settings.APIKey),
			option.WithBaseURL(settings.BaseURL),
		),
		model:     settings.Model,
		maxTokens: int64(settings.MaxTokens),
	}, nil
}
```

`Generate` 中（原第 101 行）`MaxTokens: 4096` 改为 `MaxTokens: p.maxTokens`：

```go
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: p.maxTokens,
		Messages:  anthropicMsgs,
	}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/erik/GoCustome/tiny-claw && go test ./internal/provider/... -v`
Expected: 3 个测试全部 PASS

- [ ] **Step 5: 验证**

Run: `cd /Users/erik/GoCustome/tiny-claw && gofmt -l internal/provider/ && go build ./... && go test ./...`
Expected: `gofmt` 无输出、build 通过、全部测试绿

---

### Task 4: `internal/engine` — 构造器接收 Settings

**Files:**
- Modify: `internal/engine/loop.go`（构造器签名 + import）

**Interfaces:**
- Consumes: Task 1/2 的 `config.Settings`、Task 3 的 `provider.LLMProvider`
- Produces: `func NewAgentEngine(p provider.LLMProvider, r tools.Registry, settings config.Settings) *AgentEngine`（`WorkDir`/`EnableThink` 从 settings 读取）
- 删除：旧签名 `NewAgentEngine(p, r, workDir string, enableThinking bool)`

- [ ] **Step 1: 改造构造器**

`internal/engine/loop.go` 修改 import 与构造器：

```go
import (
	"context"
	"log/slog"

	"github.com/aethelgards/tiny-claw/internal/config"
	"github.com/aethelgards/tiny-claw/internal/provider"
	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/aethelgards/tiny-claw/internal/tools"
	"github.com/pkg/errors"
)

func NewAgentEngine(p provider.LLMProvider, r tools.Registry, settings config.Settings) *AgentEngine {
	return &AgentEngine{
		provider:    p,
		registry:    r,
		WorkDir:     settings.WorkDir,
		EnableThink: settings.EnableThinking,
	}
}
```

`Run` 方法体与 `AgentEngine` 结构体**不做任何改动**。

- [ ] **Step 2: 验证编译**

Run: `cd /Users/erik/GoCustome/tiny-claw && go build ./...`
Expected: build 通过（engine 无测试文件，`cmd/claw/main.go` 目前是空 main，不影响编译）

- [ ] **Step 3: 验证全量**

Run: `cd /Users/erik/GoCustome/tiny-claw && gofmt -l internal/engine/ && go vet ./... && go test ./...`
Expected: `gofmt` 无输出、vet 通过、全部测试绿

---

### Task 5: `cmd/claw/main.go` 装配 + `.gitignore`

**Files:**
- Modify: `cmd/claw/main.go`（从空 main 变为完整装配）
- Create: `.gitignore`

**Interfaces:**
- Consumes: `config.Load()`、`provider.NewProvider()`、`engine.NewAgentEngine()`、`tools.NewToolRegistry()`、`tools.NewReadFileTool(workDir)`、`tools.NewWriteFileTool(workDir)`、`tools.NewBashTool(workDir)`
- Produces: 可运行二进制 `claw <prompt>`；`.gitignore` 忽略 `.claw/settings.local.json`

- [ ] **Step 1: 写装配代码**

`cmd/claw/main.go` 全文替换为：

```go
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aethelgards/tiny-claw/internal/config"
	"github.com/aethelgards/tiny-claw/internal/engine"
	"github.com/aethelgards/tiny-claw/internal/provider"
	"github.com/aethelgards/tiny-claw/internal/tools"
)

func main() {
	settings, err := config.Load()
	if err != nil {
		slog.Error("配置加载失败", "err", err)
		os.Exit(1)
	}

	p, err := provider.NewProvider(*settings)
	if err != nil {
		slog.Error("provider 初始化失败", "err", err)
		os.Exit(1)
	}

	reg := tools.NewToolRegistry()
	for name, tool := range map[string]tools.BaseTool{
		"read_file":  tools.NewReadFileTool(settings.WorkDir),
		"write_file": tools.NewWriteFileTool(settings.WorkDir),
		"bash":       tools.NewBashTool(settings.WorkDir),
	} {
		if err := reg.Registry(tool); err != nil {
			slog.Error("工具注册失败", "tool", name, "err", err)
			os.Exit(1)
		}
	}

	agent := engine.NewAgentEngine(p, reg, *settings)

	if len(os.Args) < 2 {
		slog.Error("用法: claw <prompt>")
		os.Exit(1)
	}

	if err := agent.Run(context.Background(), os.Args[1]); err != nil {
		slog.Error("运行失败", "err", err)
		os.Exit(1)
	}
}
```

注意：`NewReadFileTool`/`NewWriteFileTool`/`NewBashTool` 第一参都是 `workDir string`，这里统一传 `settings.WorkDir`。

创建 `.gitignore`：

```
.claw/settings.local.json
```

- [ ] **Step 2: 验证编译 + 全量测试**

Run: `cd /Users/erik/GoCustome/tiny-claw && gofmt -l cmd/ && go build ./... && go vet ./... && go test ./...`
Expected: `gofmt` 无输出、build/vet 通过、全部测试绿

- [ ] **Step 3: 端到端冒烟（验证配置加载报错路径）**

Run: `cd /Users/erik/GoCustome/tiny-claw && go run ./cmd/claw "hi" 2>&1`
Expected: 退出码 1，日志显示 `配置加载失败`（apiKey/model 未配置的校验错误）——证明 Load→validate 链路生效

Run: `cd /Users/erik/GoCustome/tiny-claw && CLAW_API_KEY=sk-test CLAW_MODEL=glm-4.6 go run ./cmd/claw "hi" 2>&1`
Expected: 不再报配置校验错，进入引擎流程（可能因 API 调用失败报 provider 错误或网络错误，这属于正常——配置系统已生效）

---

## 完成后验收

- [ ] `gofmt -l internal/ cmd/` 无输出
- [ ] `go build ./...` 通过
- [ ] `go vet ./...` 通过
- [ ] `go test ./...` 全绿（config 7 个新测试 + tools 21 个既有测试 + provider 3 个新测试）
- [ ] 无任何 `panic("请设置 ZHIPU_API_KEY")` / `var baseURL` 残留（`grep -rn "ZHIPU_API_KEY\|var baseURL" internal/` 应为空）
- [ ] `.gitignore` 含 `.claw/settings.local.json`
