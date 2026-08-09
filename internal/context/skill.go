package context

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type Skill struct {
	Name        string
	Description string
	Body        string
}

type SkillLoader struct {
	workDir string
}

func NewSkillLoader(workDir string) *SkillLoader {
	return &SkillLoader{workDir: workDir}
}

func (s *SkillLoader) LoadAll() []*Skill {
	skillPath := filepath.Join(s.workDir, ".claw", "skills")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		return nil
	}

	var skills []*Skill
	err := filepath.WalkDir(skillPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("walk skill path failed", slog.String("path", path), slog.String("err", err.Error()))
			return nil // 跳过单个坏条目，不中断整个遍历
		}
		if d.IsDir() || d.Name() != "SKILL.md" {
			return nil // 修复：文件才可能是 SKILL.md；跳过目录与其他文件
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("read skill failed", slog.String("path", path), slog.String("err", err.Error()))
			return nil
		}
		skill, perr := parseSkillMD(string(raw))
		if perr != nil {
			slog.Warn("parse skill failed, skip", slog.String("path", path), slog.String("err", perr.Error()))
			return nil // 决策3：跳过坏技能，不影响其他
		}
		skills = append(skills, skill)
		return nil
	})
	if err != nil {
		slog.Warn("load all skills failed", slog.String("path", skillPath), slog.String("err", err.Error()))
		return nil
	}
	return skills
}

func parseSkillMD(content string) (*Skill, error) {
	skill := &Skill{
		Name:        "Unknown Skill",
		Description: "No description provided",
		Body:        "",
	}

	if strings.HasPrefix(content, "---\n") || strings.HasPrefix(content, "---\r\n") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("skill frontmatter missing closing '---' delimiter")
		}
		skill.Body = strings.TrimSpace(parts[2])

		frontmatter := parts[1]
		for line := range strings.SplitSeq(frontmatter, "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "name:"):
				skill.Name = parseKV(line)
			case strings.HasPrefix(line, "description:"):
				skill.Description = parseKV(line)
			}
		}
	} else {
		// 无 frontmatter：整个文件即正文（兼容纯 markdown 技能文件）。
		skill.Body = strings.TrimSpace(content)
	}
	return skill, nil
}

// parseKV 提取 "key: value" 中的 value，容忍值内含冒号与引号。
func parseKV(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(parts[1]), `"'`)
}
