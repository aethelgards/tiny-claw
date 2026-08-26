package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Compact 衰减整理
func (s *MemoryStore) Compact(threshold float64) (int, error) {
	evicted := 0
	for _, scope := range []Scope{ScopeProject, ScopeGlobal} {
		s.mu.Lock()
		for _, t := range AllTypes {
			slice := s.memories[scope][t]
			if slice == nil {
				continue
			}
			var keep []*Memory
			for _, m := range slice {
				if m.AccessCount == 0 && time.Since(m.CreatedAt) > 30*24*time.Hour {
					_ = s.archiveLocked(scope, t, m)
					evicted++
				} else if score(m) < threshold && time.Since(m.LastAccessAt) > 30*24*time.Hour {
					_ = s.archiveLocked(scope, t, m)
					evicted++
				} else {
					keep = append(keep, m)
				}
			}
			s.memories[scope][t] = keep
			_ = s.persistScopeLocked(scope, t)
		}
		s.mu.Unlock()
	}
	return evicted, nil
}

func (s *MemoryStore) archiveLocked(scope Scope, t MemoryType, m *Memory) error {
	dir := s.dir(scope)
	archiveDir := filepath.Join(dir, "archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return err
	}
	archivePath := filepath.Join(archiveDir, string(t)+".jsonl")
	line, _ := json.Marshal(m)
	// 追加写：多次归档不得互相覆盖
	f, err := os.OpenFile(archivePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

func score(m *Memory) float64 {
	if m.AccessCount == 0 {
		return 0
	}
	days := time.Since(m.LastAccessAt).Hours() / 24
	if days < 0 {
		days = 0
	}
	return float64(m.AccessCount) * (1 / (1 + 0.05*days))
}

// sortByScore 按分数降序，分数相同按 UpdatedAt 降序（稳定排序）
func sortByScore(candidates []*Memory) {
	sort.SliceStable(candidates, func(i, j int) bool {
		si := score(candidates[i])
		sj := score(candidates[j])
		if si != sj {
			return si > sj
		}
		return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt)
	})
}