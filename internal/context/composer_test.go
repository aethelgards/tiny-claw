package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

func TestBuild_NoSkills(t *testing.T) {
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "AGENT.md"), []byte("# Agent rules\n"), 0o644); err != nil {
		t.Fatalf("write AGENT.md: %v", err)
	}
	composer := NewPromptComposer(workDir, false)

	msg := composer.Build()

	if msg.Role != schema.RoleSystem {
		t.Errorf("Role = %q, want %q", msg.Role, schema.RoleSystem)
	}
	if !strings.HasPrefix(msg.Content, promptCore) {
		t.Errorf("Content does not start with promptCore")
	}
	if !strings.Contains(msg.Content, "# Agent rules") {
		t.Errorf("Content missing AGENT.md content")
	}
	if !strings.Contains(msg.Content, skillStrategy) {
		t.Errorf("Content missing skillStrategy")
	}
	if strings.Contains(msg.Content, "可用专业技能") {
		t.Errorf("Content should not contain skill header when no skills present")
	}
}

func TestBuild_WithSkills(t *testing.T) {
	workDir := t.TempDir()
	writeTestSkill(t, workDir, "demo",
		"---\nname: demo\ndescription: demo skill\n---\n\n# Demo body\n")
	composer := NewPromptComposer(workDir, false)

	msg := composer.Build()

	if !strings.HasPrefix(msg.Content, promptCore) {
		t.Errorf("Content does not start with promptCore")
	}
	if !strings.Contains(msg.Content, skillStrategy) {
		t.Errorf("Content missing skillStrategy:\n%s", msg.Content)
	}
	if strings.Contains(msg.Content, "# Demo body") {
		t.Errorf("Content should not contain skill body (v2: skill body loaded via tool call, not injected):\n%s", msg.Content)
	}
	if strings.Contains(msg.Content, "可用专业技能") {
		t.Errorf("Content should not contain v1 skill header:\n%s", msg.Content)
	}
}
