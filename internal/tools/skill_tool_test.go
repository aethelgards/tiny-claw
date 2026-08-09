package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSkillTool_Name(t *testing.T) {
	tool := NewSkillTool("tdd-workflow", "desc", "body")
	if got := tool.Name(); got != "tdd-workflow" {
		t.Errorf("Name() = %q, want %q", got, "tdd-workflow")
	}
}

func TestSkillTool_Definition(t *testing.T) {
	tool := NewSkillTool("tdd-workflow", "Use this skill when writing tests.", "body")

	def := tool.Definition()

	if def.Name != "tdd-workflow" {
		t.Errorf("Definition.Name = %q, want %q", def.Name, "tdd-workflow")
	}
	if def.Description != "Use this skill when writing tests." {
		t.Errorf("Definition.Description = %q, want %q", def.Description, "Use this skill when writing tests.")
	}
	input, ok := def.InputSchema["type"].(string)
	if !ok || input != "object" {
		t.Errorf("Definition.InputSchema[\"type\"] = %v, want \"object\"", def.InputSchema["type"])
	}
	props, ok := def.InputSchema["properties"].(map[string]any)
	if !ok || len(props) != 0 {
		t.Errorf("Definition.InputSchema[\"properties\"] = %v, want empty map (no-arg skill)", def.InputSchema["properties"])
	}
}

func TestSkillTool_Execute(t *testing.T) {
	shortBody := "# TDD Workflow\nWrite tests first."
	longBody := strings.Repeat("a", maxSkillBodyLen) + "TAILMARKER"

	tests := []struct {
		name       string
		body       string
		wantSuffix string
		wantMarker bool
	}{
		{name: "short body returns prefix + full body", body: shortBody, wantSuffix: shortBody, wantMarker: false},
		{name: "body over 32KB is truncated with marker", body: longBody, wantSuffix: strings.Repeat("a", maxSkillBodyLen), wantMarker: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := NewSkillTool("demo", "desc", tt.body)

			out, err := tool.Execute(context.Background(), json.RawMessage("{}"))
			if err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			wantPrefix := "以下是你加载的技能 <demo> 的完整执行指南，必须严格遵循：\n\n"
			if !strings.HasPrefix(out, wantPrefix) {
				t.Errorf("Execute output missing obey prefix %q:\n%s", wantPrefix, out)
			}
			if !strings.Contains(out, tt.wantSuffix) {
				t.Errorf("Execute output missing expected body content")
			}
			if tt.wantMarker {
				if !strings.Contains(out, "[技能正文超出 32KB 已截断]") {
					t.Errorf("Execute output missing truncation marker")
				}
				if strings.Contains(out, "TAILMARKER") {
					t.Errorf("Execute output contains content beyond 32KB limit")
				}
			} else if strings.Contains(out, "[技能正文超出 32KB 已截断]") {
				t.Errorf("Execute output should not contain truncation marker for short body")
			}
		})
	}
}

// ensureSkillToolConforms 编译期断言 SkillTool 实现 BaseTool。
var _ BaseTool = (*SkillTool)(nil)
