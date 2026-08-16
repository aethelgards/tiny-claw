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

func TestLarkDefaults(t *testing.T) {
	root := t.TempDir()
	s, err := loadFrom(filepath.Join(root, "missing", "settings.json"), root)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if s.LarkAppID != "" || s.LarkAppSecret != "" {
		t.Errorf("lark 凭据应默认为空: %+v", s)
	}
	if s.LarkChannelSize != 64 {
		t.Errorf("LarkChannelSize = %d, want 64", s.LarkChannelSize)
	}
}

func TestLarkConfigFileLayer(t *testing.T) {
	proj := t.TempDir()
	mustWrite(t, filepath.Join(proj, ".claw", "settings.json"),
		`{"larkAppId":"cli_x","larkAppSecret":"secret_x","larkChannelSize":128}`)

	s, err := loadFrom(filepath.Join(proj, "..", "nope.json"), proj)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if s.LarkAppID != "cli_x" || s.LarkAppSecret != "secret_x" {
		t.Errorf("lark 凭据解析错误: %+v", s)
	}
	if s.LarkChannelSize != 128 {
		t.Errorf("LarkChannelSize = %d, want 128", s.LarkChannelSize)
	}
}

func TestLarkEnvOverrides(t *testing.T) {
	t.Setenv("CLAW_LARK_APP_ID", "cli_env")
	t.Setenv("CLAW_LARK_APP_SECRET", "secret_env")
	t.Setenv("CLAW_LARK_CHANNEL_SIZE", "256")

	s := defaultSettings()
	applyEnv(&s)
	if s.LarkAppID != "cli_env" || s.LarkAppSecret != "secret_env" {
		t.Errorf("lark env overrides wrong: %+v", s)
	}
	if s.LarkChannelSize != 256 {
		t.Errorf("LarkChannelSize = %d, want 256", s.LarkChannelSize)
	}
}

func TestLarkEnvInvalidChannelSizeIgnored(t *testing.T) {
	t.Setenv("CLAW_LARK_CHANNEL_SIZE", "abc")
	s := defaultSettings()
	applyEnv(&s)
	if s.LarkChannelSize != 64 {
		t.Errorf("LarkChannelSize = %d, want 64 (invalid env ignored)", s.LarkChannelSize)
	}
}

func TestDefaultApprovalTimeout(t *testing.T) {
	if got := defaultSettings().ApprovalTimeout; got != "5m" {
		t.Errorf("ApprovalTimeout = %q, want 5m", got)
	}
}

func TestApplyLayerApprovalTimeout(t *testing.T) {
	s := defaultSettings()
	if err := applyLayer(&s, []byte(`{"approvalTimeout":"10m"}`)); err != nil {
		t.Fatalf("applyLayer: %v", err)
	}
	if s.ApprovalTimeout != "10m" {
		t.Errorf("ApprovalTimeout = %q, want 10m", s.ApprovalTimeout)
	}
}

func TestApplyEnvApprovalTimeout(t *testing.T) {
	t.Setenv("CLAW_APPROVAL_TIMEOUT", "2m")
	s := defaultSettings()
	applyEnv(&s)
	if s.ApprovalTimeout != "2m" {
		t.Errorf("ApprovalTimeout = %q, want 2m", s.ApprovalTimeout)
	}
}

func TestLoadKeepsApprovalTimeout(t *testing.T) {
	proj := t.TempDir()
	mustWrite(t, filepath.Join(proj, ".claw", "settings.json"),
		`{"approvalTimeout":"10m"}`)

	s, err := loadFrom(filepath.Join(proj, "..", "nope.json"), proj)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if s.ApprovalTimeout != "10m" {
		t.Errorf("ApprovalTimeout = %q, want 10m", s.ApprovalTimeout)
	}
}
