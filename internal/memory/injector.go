package memory

import (
	"context"
)

// MemoryInjector 注入器：从 MemoryStore 获取最近活跃记忆并按 token 预算裁剪
type MemoryInjector struct {
	store     *MemoryStore
	maxTokens int // 注入预算，默认 400 tokens（estTokens 同口径估算）
}

func NewMemoryInjector(store *MemoryStore, maxTokens int) *MemoryInjector {
	if maxTokens <= 0 {
		maxTokens = 400
	}
	return &MemoryInjector{
		store:     store,
		maxTokens: maxTokens,
	}
}

// Recent 返回最近活跃记忆（项目 + 全局合并，项目优先），按预算裁剪
// 注入命中不调用 Touch()，避免富者愈富的 recency 正反馈（R10）
func (i *MemoryInjector) Recent(_ context.Context) []Memory {
	// 分别获取项目和全局，按 score 降序
	projectMemories := i.store.Recent(ScopeProject, 50)
	globalMemories := i.store.Recent(ScopeGlobal, 50)

	// 合并：项目优先，然后全局
	candidates := append(projectMemories, globalMemories...)

	if len(candidates) == 0 {
		return nil
	}

	// 按预算裁剪：estTokens 估算，超预算从低分开始裁剪
	var selected []Memory
	used := 0
	for _, m := range candidates {
		tokens := estTokens(m)
		if used+tokens > i.maxTokens {
			break
		}
		selected = append(selected, m)
		used += tokens
	}

	return selected
}

// estTokens 单条消息的保守 token 估算（与 session.go 同口径：4 + 字符数/2）
func estTokens(m Memory) int {
	n := 4
	n += len(m.Content) / 2
	return n
}
