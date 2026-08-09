package context

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/aethelgards/tiny-claw/internal/tools"
)

func TestSanitizeSkillName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "ascii with dash kept", in: "tdd-workflow", want: "tdd-workflow"},
		{name: "chinese chars become underscores", in: "中文技能名", want: "_____"},
		{name: "empty falls back to skill", in: "", want: "skill"},
		{name: "mixed invalid chars replaced", in: "my skill!", want: "my_skill_"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeSkillName(tt.in); got != tt.want {
				t.Errorf("sanitizeSkillName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRegisterSkills_NoSkillsDir(t *testing.T) {
	reg := tools.NewToolRegistry()

	if err := RegisterSkills(reg, t.TempDir()); err != nil {
		t.Fatalf("RegisterSkills returned error: %v", err)
	}
	if got := len(reg.GetAvailableTools()); got != 0 {
		t.Errorf("registered %d tools, want 0 when no skills dir exists", got)
	}
}

func TestRegisterSkills_OneValid(t *testing.T) {
	workDir := t.TempDir()
	writeTestSkill(t, workDir, "demo",
		"---\nname: demo\ndescription: demo skill\n---\n\n# Demo body\n")
	reg := tools.NewToolRegistry()

	if err := RegisterSkills(reg, workDir); err != nil {
		t.Fatalf("RegisterSkills returned error: %v", err)
	}

	defs := reg.GetAvailableTools()
	if len(defs) != 1 {
		t.Fatalf("registered %d tools, want 1", len(defs))
	}
	if defs[0].Name != "demo" {
		t.Errorf("tool name = %q, want %q", defs[0].Name, "demo")
	}

	res := reg.Execute(context.Background(), schema.ToolCall{Name: "demo", Arguments: json.RawMessage("{}")})
	if res.IsError {
		t.Fatalf("Execute returned error: %s", res.Output)
	}
	if !strings.Contains(res.Output, "必须严格遵循") {
		t.Errorf("Execute output missing obey prefix:\n%s", res.Output)
	}
	if !strings.Contains(res.Output, "# Demo body") {
		t.Errorf("Execute output missing skill body:\n%s", res.Output)
	}
}

func TestRegisterSkills_DuplicateName(t *testing.T) {
	workDir := t.TempDir()
	writeTestSkill(t, workDir, "one",
		"---\nname: demo\ndescription: first\n---\n\nfirst body\n")
	writeTestSkill(t, workDir, "two",
		"---\nname: demo\ndescription: second\n---\n\nsecond body\n")
	reg := tools.NewToolRegistry()

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	if err := RegisterSkills(reg, workDir); err != nil {
		t.Fatalf("RegisterSkills returned error: %v", err)
	}

	if got := len(reg.GetAvailableTools()); got != 1 {
		t.Errorf("registered %d tools, want 1 (duplicate name must be skipped)", got)
	}
	if !strings.Contains(buf.String(), "register skill failed") {
		t.Errorf("expected warn log for duplicate registration, got:\n%s", buf.String())
	}
}

func TestRegisterSkills_SkipsBadFormat(t *testing.T) {
	workDir := t.TempDir()
	writeTestSkill(t, workDir, "good",
		"---\nname: good\ndescription: ok\n---\n\n# Good body\n")
	// 缺闭合分隔符的坏技能：LoadAll 跳过，不影响其余注册。
	writeTestSkill(t, workDir, "bad",
		"---\nname: bad\ndescription: missing closing\n")
	reg := tools.NewToolRegistry()

	if err := RegisterSkills(reg, workDir); err != nil {
		t.Fatalf("RegisterSkills returned error: %v", err)
	}

	defs := reg.GetAvailableTools()
	if len(defs) != 1 {
		t.Fatalf("registered %d tools, want 1 (bad-format skill must be skipped)", len(defs))
	}
	if defs[0].Name != "good" {
		t.Errorf("tool name = %q, want %q", defs[0].Name, "good")
	}
	res := reg.Execute(context.Background(), schema.ToolCall{Name: "good", Arguments: json.RawMessage("{}")})
	if res.IsError {
		t.Fatalf("Execute returned error: %s", res.Output)
	}
	if !strings.Contains(res.Output, "# Good body") {
		t.Errorf("Execute output missing skill body:\n%s", res.Output)
	}
}
