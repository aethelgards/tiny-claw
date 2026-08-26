// internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Settings 与 Claude Code 的 settings.json 同构的扁平配置结构。
type Settings struct {
	Provider       string `json:"provider"` // "openai" | "claude"
	Model          string `json:"model"`    // 如 "glm-4.6"
	APIKey         string `json:"apiKey"`
	BaseURL        string `json:"baseURL"`        // 默认智谱底座兼容端点
	MaxTokens      int    `json:"maxTokens"`      // 默认 4096
	EnableThinking bool   `json:"enableThinking"` // 默认 false
	WorkDir        string `json:"workDir"`        // 默认 "."
	PlanMode       bool   `json:"planMode"`

	// Lark 机器人模式配置（仅 cmd/larkbot 使用，CLI 模式可缺省）
	LarkAppID       string `json:"larkAppId"`
	LarkAppSecret   string `json:"larkAppSecret"`
	LarkChannelSize int    `json:"larkChannelSize"` // 消息队列容量，默认 64
	ApprovalTimeout string `json:"approvalTimeout"` // 审批超时（Go duration 字符串），默认 "5m"，<=0 视为默认

	// Memory 记忆系统配置
	Memory *MemoryConfig `json:"memory"`

	// ModelPricing 模型费用配置（单位：每百万 tokens）
	ModelPricing map[string]ModelPricing `json:"modelPricing,omitempty"`

	Log *LogConfig `json:"log"`
}

type ModelPricing struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

type MemoryConfig struct {
	MaxInjectTokens  int             `json:"maxInjectTokens"`  // 注入 token 预算，默认 400
	ExtractTimeout   string          `json:"extractTimeout"`   // 自动提取 LLM 超时，默认 "10s"
	CompactThreshold float64         `json:"compactThreshold"` // Compact 淘汰分数阈值，默认 1.0
	DecayLambda      float64         `json:"decayLambda"`      // 衰减系数，默认 0.05
	Embedding        *EmbeddingConfig `json:"embedding,omitempty"`
}

type EmbeddingConfig struct {
	Model    string  `json:"model,omitempty"`
	BaseURL  string  `json:"baseURL,omitempty"`
	APIKey   string  `json:"apiKey,omitempty"`
	Timeout  string  `json:"timeout,omitempty"`
	MinScore float64 `json:"minScore,omitempty"`
}

type LogConfig struct {
	Level     int    `json:"level"`
	Format    string `json:"format"`
	LogDir    string `json:"logDir"`
	AddSource bool   `json:"addSource"`
	Console   bool   `json:"console"`
}

func defaultSettings() Settings {
	return Settings{
		Provider:        "openai",
		BaseURL:         "https://open.bigmodel.cn/api/paas/v4/",
		MaxTokens:       4096,
		WorkDir:         ".",
		LarkChannelSize: 64,
		ApprovalTimeout: "5m",
		Memory: &MemoryConfig{
			MaxInjectTokens:  400,
			ExtractTimeout:   "10s",
			CompactThreshold: 1.0,
			DecayLambda:      0.05,
		},
		//Log: &LogConfig{
		//	Level:     0,
		//	Format:    "text",
		//	LogDir:    ".",
		//	AddSource: true,
		//	Console:   false,
		//},
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
	if v, ok := layer["larkAppId"]; ok {
		if err := json.Unmarshal(v, &s.LarkAppID); err != nil {
			return err
		}
	}
	if v, ok := layer["larkAppSecret"]; ok {
		if err := json.Unmarshal(v, &s.LarkAppSecret); err != nil {
			return err
		}
	}
	if v, ok := layer["larkChannelSize"]; ok {
		if err := json.Unmarshal(v, &s.LarkChannelSize); err != nil {
			return err
		}
	}
	if v, ok := layer["approvalTimeout"]; ok {
		if err := json.Unmarshal(v, &s.ApprovalTimeout); err != nil {
			return err
		}
	}
	if v, ok := layer["log"]; ok {
		if err := json.Unmarshal(v, &s.Log); err != nil {
		}
	}
	if v, ok := layer["planMode"]; ok {
		if err := json.Unmarshal(v, &s.PlanMode); err != nil {
			return err
		}
	}
	if v, ok := layer["modelPricing"]; ok {
		if s.ModelPricing == nil {
			s.ModelPricing = make(map[string]ModelPricing)
		}
		if err := json.Unmarshal(v, &s.ModelPricing); err != nil {
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
	if v := os.Getenv("CLAW_LARK_APP_ID"); v != "" {
		s.LarkAppID = v
	}
	if v := os.Getenv("CLAW_LARK_APP_SECRET"); v != "" {
		s.LarkAppSecret = v
	}
	if v := os.Getenv("CLAW_LARK_CHANNEL_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.LarkChannelSize = n
		}
	}
	if v := os.Getenv("CLAW_APPROVAL_TIMEOUT"); v != "" {
		s.ApprovalTimeout = v
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
	if err := InitLog(s.Log); err != nil {
		panic(err)
	}
	return s, nil
}
