package memory

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aethelgards/tiny-claw/internal/provider"
)

type MemoryStore struct {
	globalDir  string
	projectDir string
	mu         sync.RWMutex
	memories   map[Scope]map[MemoryType][]*Memory
	embedder   provider.Embedder
	minScore   float64
}

func NewMemoryStore(globalDir, projectDir string, opts ...StoreOption) (*MemoryStore, error) {
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return nil, err
	}
	s := &MemoryStore{
		globalDir:  globalDir,
		projectDir: projectDir,
		memories:   make(map[Scope]map[MemoryType][]*Memory),
		minScore:   0.35,
	}
	for _, opt := range opts {
		opt(s)
	}
	for _, scope := range []Scope{ScopeGlobal, ScopeProject} {
		s.memories[scope] = make(map[MemoryType][]*Memory)
		for _, t := range AllTypes {
			s.memories[scope][t] = nil
		}
	}
	for _, scope := range []Scope{ScopeGlobal, ScopeProject} {
		if err := s.loadScopeMemory(scope); err != nil {
			return nil, err
		}
	}
	return s, nil
}

type StoreOption func(*MemoryStore)

func WithEmbedder(e provider.Embedder) StoreOption {
	return func(s *MemoryStore) {
		s.embedder = e
	}
}

func WithMinScore(score float64) StoreOption {
	return func(s *MemoryStore) {
		s.minScore = score
	}
}

func (s *MemoryStore) dir(scope Scope) string {
	if scope == ScopeGlobal {
		return s.globalDir
	}
	return s.projectDir
}

func (s *MemoryStore) loadScopeMemory(scope Scope) error {
	dir := s.dir(scope)
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			slog.Warn("loadScopeLocked read memory failed", slog.String("file", f), slog.String("err", err.Error()))
			continue
		}
		lines := splitLines(string(data))
		if len(lines) == 0 {
			slog.Warn("loadScopeLocked read memory failed", slog.String("file", f))
			continue
		}
		fileVersion := parseVersion(lines[0])
		if fileVersion == 0 {
			slog.Warn("loadScopeLocked invalid memory file", slog.String("file", f))
			continue
		}
		for _, line := range lines[1:] {
			if line == "" {
				continue
			}
			var m Memory
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				slog.Warn("loadScopeLocked invalid memory JSON", slog.String("file", f), slog.String("err", err.Error()))
				continue
			}
			if m.Type == "" {
				slog.Warn("loadScopeLocked missing memory type", slog.String("file", f))
				continue
			}
			if _, ok := ValidType(string(m.Type)); !ok {
				slog.Warn("loadScopeLocked invalid memory type", slog.String("file", f), slog.String("type", string(m.Type)))
				continue
			}
			if len(m.Embedding) > 0 && s.embedder != nil {
				m.Embedding = nil
			}
			s.memories[scope][m.Type] = append(s.memories[scope][m.Type], &m)
		}
	}
	return nil
}

func parseVersion(line string) int {
	var probe struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal([]byte(line), &probe); err != nil {
		return 0
	}
	if probe.Version != 1 && probe.Version != 2 {
		return 0
	}
	return probe.Version
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func (s *MemoryStore) Save(m Memory, scope Scope) (string, error) {
	if m.Type == "" {
		return "", errors.New("memory type is required")
	}
	if m.Content == "" {
		return "", errors.New("memory content is required")
	}
	if scope != ScopeGlobal && scope != ScopeProject {
		return "", errors.New("invalid scope: must be global or project")
	}
	m.ID = MemoryID(m.Type, m.Content)
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	if s.embedder != nil && len(m.Embedding) == 0 {
		vecs, err := s.embedder.Embed(context.Background(), []string{m.Content})
		if err != nil {
			slog.Warn("embedding failed, falling back to keyword-only", "error", err)
		} else if len(vecs) > 0 && len(vecs[0]) > 0 {
			m.Embedding = vecs[0]
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	slice := s.memories[scope][m.Type]
	if slice == nil {
		slice = make([]*Memory, 0)
	}

	found := false
	for i, existing := range slice {
		if existing.ID == m.ID {
			slice[i] = &m
			found = true
			break
		}
	}
	if !found {
		slice = append(slice, &m)
	}
	s.memories[scope][m.Type] = slice

	if err := s.persistScopeLocked(scope, m.Type); err != nil {
		return "", err
	}
	return m.ID, nil
}

func (s *MemoryStore) persistScopeLocked(scope Scope, t MemoryType) error {
	slice := s.memories[scope][t]
	if slice == nil {
		slice = []*Memory{}
	}
	dir := s.dir(scope)
	path := filepath.Join(dir, string(t)+".jsonl")
	tmp := path + ".tmp"

	var buf []byte
	buf = append(buf, []byte("{\"version\":2}\n")...)
	for _, m := range slice {
		line, _ := json.Marshal(m)
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}

	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Recall 混合检索：关键词优先，不足时回退向量语义
func (s *MemoryStore) Recall(query string, scope Scope, limit int) []Memory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var candidates []*Memory
	scopes := []Scope{scope}
	if scope == "" {
		scopes = []Scope{ScopeProject, ScopeGlobal}
	}

	for _, sc := range scopes {
		for _, t := range AllTypes {
			for _, m := range s.memories[sc][t] {
				if m == nil {
					continue
				}
				score := keywordScore(query, m.Content)
				if query == "" || score > 0 {
					candidates = append(candidates, m)
				}
			}
		}
	}

	sortMemories(candidates, query)

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	if query != "" && len(candidates) < limit && s.embedder != nil {
		candidates = s.hybridRecallFallback(query, scopes, candidates, limit)
	}

	out := make([]Memory, len(candidates))
	for i, m := range candidates {
		out[i] = *m
	}
	return out
}

func (s *MemoryStore) hybridRecallFallback(query string, scopes []Scope, existing []*Memory, limit int) []*Memory {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	vecs, err := s.embedder.Embed(ctx, []string{query})
	if err != nil || len(vecs) == 0 {
		return existing
	}
	qVec := vecs[0]

	existingIDs := make(map[string]bool, len(existing))
	for _, m := range existing {
		existingIDs[m.ID] = true
	}

	type scored struct {
		m     *Memory
		score float64
	}
	var vectorHits []scored

	for _, sc := range scopes {
		for _, t := range AllTypes {
			for _, m := range s.memories[sc][t] {
				if m == nil || len(m.Embedding) == 0 || existingIDs[m.ID] {
					continue
				}
				sim := cosineSimilarity(qVec, m.Embedding)
				if sim >= s.minScore {
					vectorHits = append(vectorHits, scored{m, sim})
				}
			}
		}
	}

	sort.Slice(vectorHits, func(i, j int) bool {
		return vectorHits[i].score > vectorHits[j].score
	})

	for _, h := range vectorHits {
		if len(existing) >= limit {
			break
		}
		existing = append(existing, h.m)
	}

	return existing
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func keywordScore(query, content string) int {
	if query == "" {
		return 0
	}

	lowerQuery := strings.ToLower(query)
	lowerContent := strings.ToLower(content)

	if lowerQuery == lowerContent {
		return 100
	}

	score := 0
	for _, term := range splitTerms(query) {
		lowerTerm := strings.ToLower(term)

		if isASCII(lowerTerm) {
			score += countWordBoundary(lowerContent, lowerTerm) * 3
		} else {
			count := strings.Count(lowerContent, lowerTerm)
			if count > 0 {
				if isCJKWordMatch(lowerContent, lowerTerm) {
					score += count * 5
				} else {
					score += count * 1
				}
			}
		}
	}
	return score
}

// isCJKWordMatch 判断 CJK term 是否作为独立语义单元出现（前后无连续 CJK 字符）
func isCJKWordMatch(content, term string) bool {
	idx := strings.Index(content, term)
	if idx == -1 {
		return false
	}

	start := idx
	end := idx + len(term)

	if start > 0 && isCJK(rune(content[start-1])) {
		return false
	}
	if end < len(content) && isCJK(rune(content[end])) {
		return false
	}

	return true
}

// isCJK 判断字符是否为 CJK 统一表意文字
func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
		(r >= 0x20000 && r <= 0x2A6DF) // CJK Extension B
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

func countWordBoundary(content, term string) int {
	count := 0
	start := 0
	for {
		idx := strings.Index(content[start:], term)
		if idx == -1 {
			break
		}
		matchStart := start + idx
		matchEnd := matchStart + len(term)

		// 检查前边界：匹配位置在开头或前一个字符非字母数字
		prevOK := matchStart == 0 || !isAlNum(rune(content[matchStart-1]))
		// 检查后边界：匹配位置在末尾或后一个字符非字母数字
		nextOK := matchEnd >= len(content) || !isAlNum(rune(content[matchEnd]))

		if prevOK && nextOK {
			count++
		}
		start = matchEnd
	}
	return count
}

func isAlNum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func splitTerms(s string) []string {
	return strings.Fields(s)
}

func sortMemories(candidates []*Memory, query string) {
	// Pre-compute scores
	type scored struct {
		m     *Memory
		score int
	}
	scoredList := make([]scored, len(candidates))
	for i, m := range candidates {
		scoredList[i] = scored{m, keywordScore(query, m.Content)}
	}
	sort.Slice(scoredList, func(i, j int) bool {
		if scoredList[i].score != scoredList[j].score {
			return scoredList[i].score > scoredList[j].score
		}
		return scoredList[i].m.UpdatedAt.After(scoredList[j].m.UpdatedAt)
	})
	for i := range candidates {
		candidates[i] = scoredList[i].m
	}
}

// Forget 删除；scope 为空则先查项目再查全局
func (s *MemoryStore) Forget(id string, scope Scope) error {
	scopes := []Scope{scope}
	if scope == "" {
		scopes = []Scope{ScopeProject, ScopeGlobal}
	}

	for _, sc := range scopes {
		s.mu.Lock()
		found := false
		var foundType MemoryType
		for _, t := range AllTypes {
			slice := s.memories[sc][t]
			for i, m := range slice {
				if m.ID == id {
					s.memories[sc][t] = append(slice[:i], slice[i+1:]...)
					found = true
					foundType = t
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			_ = s.persistScopeLocked(sc, foundType)
			s.mu.Unlock()
			return nil
		}
		s.mu.Unlock()
	}
	return nil
}

// Touch 显式 recall 计数
func (s *MemoryStore) Touch(id string, scope Scope) {
	scopes := []Scope{scope}
	if scope == "" {
		scopes = []Scope{ScopeProject, ScopeGlobal}
	}

	for _, sc := range scopes {
		s.mu.Lock()
		found := false
		var foundType MemoryType
		for _, t := range AllTypes {
			for _, m := range s.memories[sc][t] {
				if m.ID == id {
					m.AccessCount++
					m.LastAccessAt = time.Now()
					m.UpdatedAt = time.Now()
					found = true
					foundType = t
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			_ = s.persistScopeLocked(sc, foundType)
		}
		s.mu.Unlock()
		if found {
			return
		}
	}
}

// Recent 返回最近活跃记忆（按 score 降序）
func (s *MemoryStore) Recent(scope Scope, limit int) []Memory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var candidates []*Memory
	scopes := []Scope{scope}
	if scope == "" {
		scopes = []Scope{ScopeProject, ScopeGlobal}
	}

	for _, sc := range scopes {
		for _, t := range AllTypes {
			for _, m := range s.memories[sc][t] {
				if m == nil {
					continue
				}
				candidates = append(candidates, m)
			}
		}
	}

	sortByScore(candidates)

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	out := make([]Memory, len(candidates))
	for i, m := range candidates {
		out[i] = *m
	}
	return out
}
