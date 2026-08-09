package context

import (
	"log/slog"
	"strings"

	"github.com/aethelgards/tiny-claw/internal/tools"
)

// RegisterSkills 将 workDir 下所有技能注册进 registry。
// 单个技能注册失败仅记 warn 并跳过，不中断其他技能；空技能列表返回 nil 不报错。
func RegisterSkills(reg tools.Registry, workDir string) error {
	loader := NewSkillLoader(workDir)
	for _, s := range loader.LoadAll() {
		name := sanitizeSkillName(s.Name)
		tool := tools.NewSkillTool(name, s.Description, s.Body)
		if err := reg.Registry(tool); err != nil {
			slog.Warn("register skill failed", slog.String("skill", s.Name), slog.String("err", err.Error()))
			continue
		}
		slog.Info("registered skill", slog.String("skill", name))
	}
	return nil
}

// sanitizeSkillName 保证 tool name 合法（仅 [a-zA-Z0-9_-]）。
func sanitizeSkillName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "skill"
	}
	return b.String()
}
