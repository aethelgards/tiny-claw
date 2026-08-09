package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestSkill 在 workDir/.claw/skills/<name>/SKILL.md 写入技能文件（frontmatter 参考 .claw/skills/tdd-workflow/SKILL.md）。
func writeTestSkill(t *testing.T, workDir, name, content string) {
	t.Helper()
	dir := filepath.Join(workDir, ".claw", "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestParseSkillMD_StandardFrontmatter(t *testing.T) {
	content := "---\n" +
		"name: tdd-workflow\n" +
		"description: Use this skill when writing tests.\n" +
		"origin: ECC\n" +
		"---\n" +
		"\n" +
		"# TDD Workflow\n" +
		"Some body content\n"

	skill, err := parseSkillMD(content)
	if err != nil {
		t.Fatalf("parseSkillMD returned error: %v", err)
	}
	if skill.Name != "tdd-workflow" {
		t.Errorf("Name = %q, want %q", skill.Name, "tdd-workflow")
	}
	if skill.Description != "Use this skill when writing tests." {
		t.Errorf("Description = %q, want %q", skill.Description, "Use this skill when writing tests.")
	}
	if !strings.Contains(skill.Body, "# TDD Workflow") {
		t.Errorf("Body = %q, want it to contain %q", skill.Body, "# TDD Workflow")
	}
	if !strings.Contains(skill.Body, "Some body content") {
		t.Errorf("Body = %q, want it to contain %q", skill.Body, "Some body content")
	}
}

func TestParseSkillMD_DescriptionWithColon(t *testing.T) {
	content := "---\n" +
		"name: tdd-workflow\n" +
		"description: Use this when: writing tests, and: more\n" +
		"---\n" +
		"\n" +
		"body\n"

	skill, err := parseSkillMD(content)
	if err != nil {
		t.Fatalf("parseSkillMD returned error: %v", err)
	}
	want := "Use this when: writing tests, and: more"
	if skill.Description != want {
		t.Errorf("Description = %q, want %q (must not be truncated at first colon)", skill.Description, want)
	}
}

func TestParseSkillMD_QuotedName(t *testing.T) {
	content := "---\n" +
		"name: \"tdd-workflow\"\n" +
		"description: 'quoted description'\n" +
		"---\n" +
		"\n" +
		"body\n"

	skill, err := parseSkillMD(content)
	if err != nil {
		t.Fatalf("parseSkillMD returned error: %v", err)
	}
	if skill.Name != "tdd-workflow" {
		t.Errorf("Name = %q, want %q (quotes should be stripped)", skill.Name, "tdd-workflow")
	}
	if skill.Description != "quoted description" {
		t.Errorf("Description = %q, want %q (single quotes should be stripped)", skill.Description, "quoted description")
	}
}

func TestParseSkillMD_MissingClosingDelimiter(t *testing.T) {
	content := "---\n" +
		"name: broken\n" +
		"description: no closing delimiter\n"

	skill, err := parseSkillMD(content)
	if err == nil {
		t.Fatalf("parseSkillMD = (%v, nil), want error for missing closing '---'", skill)
	}
	if skill != nil {
		t.Errorf("skill = %v, want nil on error (caller must not dereference)", skill)
	}
}

func TestParseSkillMD_NoFrontmatter(t *testing.T) {
	content := "# Just a body\nno frontmatter at all\n"

	skill, err := parseSkillMD(content)
	if err != nil {
		t.Fatalf("parseSkillMD returned error: %v", err)
	}
	if skill.Name != "Unknown Skill" {
		t.Errorf("Name = %q, want default %q", skill.Name, "Unknown Skill")
	}
	if skill.Description != "No description provided" {
		t.Errorf("Description = %q, want default %q", skill.Description, "No description provided")
	}
	if skill.Body != strings.TrimSpace(content) {
		t.Errorf("Body = %q, want %q", skill.Body, strings.TrimSpace(content))
	}
}

func TestLoadAll_DirNotExist(t *testing.T) {
	loader := NewSkillLoader(t.TempDir())

	skills := loader.LoadAll()
	if skills != nil {
		t.Errorf("LoadAll = %v, want nil for missing skills directory", skills)
	}
}

func TestLoadAll_ValidDir(t *testing.T) {
	workDir := t.TempDir()
	writeTestSkill(t, workDir, "tdd-workflow",
		"---\n"+
			"name: tdd-workflow\n"+
			"description: Use this skill when writing tests.\n"+
			"origin: ECC\n"+
			"---\n"+
			"\n"+
			"# TDD Workflow\n"+
			"body\n")
	loader := NewSkillLoader(workDir)

	skills := loader.LoadAll()
	if len(skills) != 1 {
		t.Fatalf("LoadAll returned %d skills, want 1", len(skills))
	}
	if skills[0].Name != "tdd-workflow" {
		t.Errorf("skills[0].Name = %q, want %q", skills[0].Name, "tdd-workflow")
	}
	if skills[0].Description != "Use this skill when writing tests." {
		t.Errorf("skills[0].Description = %q, want %q", skills[0].Description, "Use this skill when writing tests.")
	}
}

func TestLoadAll_SkipsBadFile(t *testing.T) {
	workDir := t.TempDir()
	writeTestSkill(t, workDir, "good",
		"---\nname: good\ndescription: ok\n---\n\nbody\n")
	// 缺闭合分隔符的坏文件：应被跳过，不影响 good 加载，且不 panic。
	writeTestSkill(t, workDir, "bad",
		"---\nname: bad\ndescription: missing closing\n")
	loader := NewSkillLoader(workDir)

	skills := loader.LoadAll()
	if len(skills) != 1 {
		t.Fatalf("LoadAll returned %d skills, want 1 (bad file must be skipped)", len(skills))
	}
	if skills[0].Name != "good" {
		t.Errorf("skills[0].Name = %q, want %q", skills[0].Name, "good")
	}
}
