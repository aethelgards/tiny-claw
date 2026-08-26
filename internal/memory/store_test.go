package memory

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

func TestMemoryStore_SaveAndRecall(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, err := NewMemoryStore(globalDir, projectDir)
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}

	m := Memory{
		Type:    TypeProject,
		Content: "本项目用 Go 1.26 + gin",
		Source:  "test",
	}
	if _, err := store.Save(m, ScopeProject); err != nil {
		t.Fatalf("Save: %v", err)
	}

	results := store.Recall("gin", ScopeProject, 10)
	if len(results) != 1 {
		t.Fatalf("Recall: expected 1 result, got %d", len(results))
	}
	if results[0].Content != m.Content {
		t.Errorf("Recall content mismatch: %s", results[0].Content)
	}
	if results[0].ID == "" {
		t.Errorf("Recall result should have ID set")
	}
}

func TestMemoryStore_SaveKindIdempotent(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	m := Memory{
		Type:    TypeProject,
		Content: "重复保存测试",
		Source:  "test",
	}

	if _, err := store.Save(m, ScopeProject); err != nil {
		t.Fatalf("Save 1: %v", err)
	}
	if _, err := store.Save(m, ScopeProject); err != nil {
		t.Fatalf("Save 2: %v", err)
	}

	results := store.Recall("测试", ScopeProject, 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 unique record, got %d", len(results))
	}
	if results[0].AccessCount != 0 {
		t.Errorf("AccessCount should be 0 on save, got %d", results[0].AccessCount)
	}
}

func TestMemoryStore_Recall_EmptyQueryReturnsRecent(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	for i := 1; i <= 3; i++ {
		m := Memory{Type: TypeProject, Content: "记忆 " + string(rune('A'+i-1)), Source: "test"}
		_, _ = store.Save(m, ScopeProject)
	}

	results := store.Recall("", ScopeProject, 2)
	if len(results) != 2 {
		t.Errorf("empty query with limit 2 should return 2, got %d", len(results))
	}
	for i := 1; i < len(results); i++ {
		if results[i-1].UpdatedAt.Before(results[i].UpdatedAt) {
			t.Errorf("results should be sorted by UpdatedAt desc")
		}
	}
}

func TestMemoryStore_Recall_ScopeIsolation(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	_, _ = store.Save(Memory{Type: TypeProject, Content: "项目知识", Source: "test"}, ScopeProject)
	_, _ = store.Save(Memory{Type: TypePreference, Content: "用户偏好", Source: "test"}, ScopeGlobal)

	proj := store.Recall("知识", ScopeProject, 10)
	if len(proj) != 1 || proj[0].Content != "项目知识" {
		t.Errorf("project scope isolation failed")
	}

	glob := store.Recall("偏好", ScopeGlobal, 10)
	if len(glob) != 1 || glob[0].Content != "用户偏好" {
		t.Errorf("global scope isolation failed")
	}
}

func TestMemoryStore_Forget(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	m := Memory{Type: TypeProject, Content: "待删除记忆", Source: "test"}
	id, _ := store.Save(m, ScopeProject)

	if err := store.Forget(id, ScopeProject); err != nil {
		t.Fatalf("Forget: %v", err)
	}

	results := store.Recall("删除", ScopeProject, 10)
	if len(results) != 0 {
		t.Errorf("Forget should remove memory")
	}
}

func TestMemoryStore_Forget_AutoScope(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	m := Memory{Type: TypeProject, Content: "自动scope删除", Source: "test"}
	id, _ := store.Save(m, ScopeProject)

	if err := store.Forget(id, ""); err != nil {
		t.Fatalf("Forget with empty scope: %v", err)
	}

	results := store.Recall("自动scope", ScopeProject, 10)
	if len(results) != 0 {
		t.Errorf("auto-scope Forget should have removed from project")
	}
}

func TestMemoryStore_Touch(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	m := Memory{Type: TypeProject, Content: "Touch测试", Source: "test"}
	id, _ := store.Save(m, ScopeProject)

	before := time.Now()
	store.Touch(id, ScopeProject)
	after := time.Now()

	results := store.Recall("Touch", ScopeProject, 10)
	if len(results) != 1 {
		t.Fatalf("recall after touch failed")
	}
	if results[0].AccessCount != 1 {
		t.Errorf("AccessCount should be 1 after Touch, got %d", results[0].AccessCount)
	}
	if results[0].LastAccessAt.Before(before) || results[0].LastAccessAt.After(after) {
		t.Errorf("LastAccessAt not updated correctly")
	}
}

func TestMemoryStore_PersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store1, _ := NewMemoryStore(globalDir, projectDir)
	_, _ = store1.Save(Memory{Type: TypeProject, Content: "持久化测试", Source: "test"}, ScopeProject)
	_, _ = store1.Save(Memory{Type: TypeError, Content: "错误记录", Source: "test"}, ScopeProject)
	_, _ = store1.Save(Memory{Type: TypePreference, Content: "全局偏好", Source: "test"}, ScopeGlobal)

	store2, _ := NewMemoryStore(globalDir, projectDir)

	proj := store2.Recall("持久化", ScopeProject, 10)
	if len(proj) != 1 {
		t.Errorf("project memories not loaded from disk")
	}
	errs := store2.Recall("错误", ScopeProject, 10)
	if len(errs) != 1 {
		t.Errorf("errors not loaded from disk")
	}
	globs := store2.Recall("偏好", ScopeGlobal, 10)
	if len(globs) != 1 {
		t.Errorf("global memories not loaded from disk")
	}

	for _, d := range []string{globalDir, projectDir} {
		files, _ := filepath.Glob(filepath.Join(d, "*.jsonl"))
		for _, f := range files {
			data, _ := os.ReadFile(f)
			if len(data) == 0 {
				t.Errorf("file %s should not be empty", f)
			}
			firstLine := string(data)
			if len(firstLine) < 14 || firstLine[:14] != "{\"version\":2}\n" {
				end := 50
				if len(firstLine) < end {
					end = len(firstLine)
				}
				t.Errorf("file %s missing version header: %s", f, firstLine[:end])
			}
		}
	}
}

func TestMemoryStore_ConcurrentSafety(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func(idx int) {
			m := Memory{Type: TypeProject, Content: "并发 " + fmt.Sprintf("%03d", idx), Source: "test"}
			if _, err := store.Save(m, ScopeProject); err != nil {
				t.Logf("Save error: %v", err)
			}
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}

	results := store.Recall("并发", ScopeProject, 200)
	if len(results) != 100 {
		t.Errorf("concurrent saves lost data: expected 100, got %d", len(results))
	}
}

func TestMemoryStore_RecallKeywordRelevance(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	_, _ = store.Save(Memory{Type: TypeProject, Content: "本项目用 Go 1.26 + gin 测试", Source: "test"}, ScopeProject)
	_, _ = store.Save(Memory{Type: TypeProject, Content: "Go 语言基础教程", Source: "test"}, ScopeProject)
	_, _ = store.Save(Memory{Type: TypeProject, Content: "Python 入门指南", Source: "test"}, ScopeProject)

	results := store.Recall("Go", ScopeProject, 10)
	if len(results) != 2 {
		t.Errorf("keyword Go should match 2 records, got %d", len(results))
	}
	for _, r := range results {
		if !containsIgnoreCase(r.Content, "Go") {
			t.Errorf("result %s should contain Go", r.Content)
		}
	}
}

func containsIgnoreCase(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestKeywordScore_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		content string
		want    int
	}{
		{"query lower content upper", "go", "Go 语言", 3},
		{"query upper content lower", "Go", "go build", 3},
		{"both upper", "GO", "GO lang", 3},
		{"mixed case", "Go", "gO lang", 3},
		{"no match different word", "Go", "Python", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keywordScore(tt.query, tt.content)
			if got != tt.want {
				t.Errorf("keywordScore(%q, %q) = %d, want %d", tt.query, tt.content, got, tt.want)
			}
		})
	}
}

func TestKeywordScore_WordBoundary(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		content string
		want    int
	}{
		{"gin not in begin", "gin", "begin", 0},
		{"gin not in margin", "gin", "margin", 0},
		{"gin standalone", "gin", "use gin framework", 3},
		{"gin with punctuation", "gin", "gin,redis,mysql", 3},
		{"go in golang", "go", "golang", 0},
		{"go standalone", "go", "go build", 3},
		{"test in testing", "test", "testing", 0},
		{"test standalone", "test", "run test", 3},
		{"CJK standalone word", "用户", "用户名偏好", 5},
		{"CJK multiple standalone", "用户", "用户A和用户B", 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keywordScore(tt.query, tt.content)
			if got != tt.want {
				t.Errorf("keywordScore(%q, %q) = %d, want %d", tt.query, tt.content, got, tt.want)
			}
		})
	}
}

func TestRecall_CaseInsensitiveIntegration(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewMemoryStore(filepath.Join(dir, "global"), filepath.Join(dir, "project"))

	_, _ = store.Save(Memory{Type: TypeProject, Content: "用户偏好中文回复", Source: "test"}, ScopeProject)

	results := store.Recall("偏好", ScopeProject, 10)
	if len(results) != 1 {
		t.Errorf("CJK recall should match, got %d", len(results))
	}

	results = store.Recall("偏好", ScopeProject, 10)
	if len(results) != 1 {
		t.Errorf("case-insensitive CJK should match, got %d", len(results))
	}
}

func TestRecall_WordBoundaryIntegration(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewMemoryStore(filepath.Join(dir, "global"), filepath.Join(dir, "project"))

	_, _ = store.Save(Memory{Type: TypeProject, Content: "使用 gin 框架", Source: "test"}, ScopeProject)
	_, _ = store.Save(Memory{Type: TypeProject, Content: "begin 开始", Source: "test"}, ScopeProject)

	results := store.Recall("gin", ScopeProject, 10)
	if len(results) != 1 {
		t.Fatalf("gin should match 1 record, got %d", len(results))
	}
	if results[0].Content != "使用 gin 框架" {
		t.Errorf("gin should match '使用 gin 框架', got %s", results[0].Content)
	}
}

type fakeEmbedder struct {
	dim    int
	calls  int
	vectors map[string][]float32
}

func newFakeEmbedder(dim int) *fakeEmbedder {
	return &fakeEmbedder{
		dim:     dim,
		vectors: make(map[string][]float32),
	}
}

func (f *fakeEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	f.calls++
	result := make([][]float32, len(texts))
	for i, t := range texts {
		if v, ok := f.vectors[t]; ok {
			result[i] = v
		} else {
			result[i] = f.makeDeterministic(t)
		}
	}
	return result, nil
}

func (f *fakeEmbedder) makeDeterministic(text string) []float32 {
	vec := make([]float32, f.dim)
	hash := 0
	for _, c := range text {
		hash = hash*31 + int(c)
	}
	for i := range vec {
		hash = hash*31 + i
		vec[i] = float32(hash%1000) / 1000.0
	}
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}

type failingEmbedder struct{}

func (f *failingEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, errors.New("embedding API unavailable")
}

func TestHybridRecall_VectorFallback(t *testing.T) {
	dir := t.TempDir()
	emb := newFakeEmbedder(8)

	emb.vectors["用户偏好中文回复"] = []float32{1, 0, 0, 0, 0, 0, 0, 0}
	emb.vectors["语言习惯"] = []float32{0.95, 0.31, 0, 0, 0, 0, 0, 0}

	store, _ := NewMemoryStore(
		filepath.Join(dir, "global"),
		filepath.Join(dir, "project"),
		WithEmbedder(emb),
		WithMinScore(0.35),
	)

	_, _ = store.Save(Memory{Type: TypeProject, Content: "用户偏好中文回复", Source: "test"}, ScopeProject)

	results := store.Recall("语言习惯", ScopeProject, 10)
	if len(results) != 1 {
		t.Fatalf("vector fallback should find 1 result, got %d", len(results))
	}
	if results[0].Content != "用户偏好中文回复" {
		t.Errorf("vector fallback should find '用户偏好中文回复', got %s", results[0].Content)
	}
}

func TestHybridRecall_KeywordPreferred(t *testing.T) {
	dir := t.TempDir()
	emb := newFakeEmbedder(8)

	emb.vectors["Go 编程"] = []float32{1, 0, 0, 0, 0, 0, 0, 0}
	emb.vectors["Python 编程"] = []float32{0.99, 0.14, 0, 0, 0, 0, 0, 0}

	store, _ := NewMemoryStore(
		filepath.Join(dir, "global"),
		filepath.Join(dir, "project"),
		WithEmbedder(emb),
		WithMinScore(0.35),
	)

	_, _ = store.Save(Memory{Type: TypeProject, Content: "Go 编程", Source: "test"}, ScopeProject)
	_, _ = store.Save(Memory{Type: TypeProject, Content: "Python 编程", Source: "test"}, ScopeProject)

	results := store.Recall("Go", ScopeProject, 1)
	if len(results) != 1 {
		t.Fatalf("keyword should match 1 result with limit=1, got %d", len(results))
	}
	if results[0].Content != "Go 编程" {
		t.Errorf("keyword should prefer exact match, got %s", results[0].Content)
	}
}

func TestHybridRecall_NoEmbedderFallback(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewMemoryStore(filepath.Join(dir, "global"), filepath.Join(dir, "project"))

	_, _ = store.Save(Memory{Type: TypeProject, Content: "用户偏好中文回复", Source: "test"}, ScopeProject)

	results := store.Recall("语言习惯", ScopeProject, 10)
	if len(results) != 0 {
		t.Errorf("without embedder, unrelated query should return 0, got %d", len(results))
	}
}

func TestHybridRecall_EmbedFailureGraceful(t *testing.T) {
	dir := t.TempDir()
	emb := &failingEmbedder{}

	store, _ := NewMemoryStore(
		filepath.Join(dir, "global"),
		filepath.Join(dir, "project"),
		WithEmbedder(emb),
	)

	_, _ = store.Save(Memory{Type: TypeProject, Content: "测试记忆", Source: "test"}, ScopeProject)

	results := store.Recall("测试", ScopeProject, 10)
	if len(results) != 1 {
		t.Errorf("embed failure should still return keyword results, got %d", len(results))
	}
}

func TestSave_EmbedFailureStillPersists(t *testing.T) {
	dir := t.TempDir()
	emb := &failingEmbedder{}

	store, _ := NewMemoryStore(
		filepath.Join(dir, "global"),
		filepath.Join(dir, "project"),
		WithEmbedder(emb),
	)

	id, err := store.Save(Memory{Type: TypeProject, Content: "embed失败测试", Source: "test"}, ScopeProject)
	if err != nil {
		t.Fatalf("Save should succeed even if embed fails: %v", err)
	}
	if id == "" {
		t.Error("Save should return ID")
	}

	results := store.Recall("embed", ScopeProject, 10)
	if len(results) != 1 {
		t.Errorf("memory should be persisted even without embedding")
	}
	if len(results[0].Embedding) != 0 {
		t.Error("embedding should be empty on embed failure")
	}
}

func TestSave_WithEmbedderStoresVector(t *testing.T) {
	dir := t.TempDir()
	emb := newFakeEmbedder(4)

	store, _ := NewMemoryStore(
		filepath.Join(dir, "global"),
		filepath.Join(dir, "project"),
		WithEmbedder(emb),
	)

	_, err := store.Save(Memory{Type: TypeProject, Content: "向量存储测试", Source: "test"}, ScopeProject)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	store2, _ := NewMemoryStore(filepath.Join(dir, "global"), filepath.Join(dir, "project"))
	results := store2.Recall("向量", ScopeProject, 10)
	if len(results) != 1 {
		t.Fatalf("Recall: expected 1, got %d", len(results))
	}
	if len(results[0].Embedding) != 4 {
		t.Errorf("embedding should be persisted, got len=%d", len(results[0].Embedding))
	}
}

func TestLoadV1File_BackwardCompatible(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")
	os.MkdirAll(globalDir, 0o755)
	os.MkdirAll(projectDir, 0o755)

	v1Data := []byte("{\"version\":1}\n{\"id\":\"abc123\",\"type\":\"project\",\"content\":\"v1记忆\",\"createdAt\":\"2026-01-01T00:00:00Z\",\"updatedAt\":\"2026-01-01T00:00:00Z\"}\n")
	os.WriteFile(filepath.Join(projectDir, "project.jsonl"), v1Data, 0o644)

	store, err := NewMemoryStore(globalDir, projectDir)
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}

	results := store.Recall("v1", ScopeProject, 10)
	if len(results) != 1 {
		t.Fatalf("v1 file should be readable, got %d results", len(results))
	}
	if results[0].Content != "v1记忆" {
		t.Errorf("v1 content mismatch: %s", results[0].Content)
	}
}

func TestLoadV2File_WithEmbedding(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")
	os.MkdirAll(globalDir, 0o755)
	os.MkdirAll(projectDir, 0o755)

	v2Data := []byte("{\"version\":2}\n{\"id\":\"def456\",\"type\":\"project\",\"content\":\"v2记忆\",\"embedding\":[0.1,0.2,0.3],\"createdAt\":\"2026-01-01T00:00:00Z\",\"updatedAt\":\"2026-01-01T00:00:00Z\"}\n")
	os.WriteFile(filepath.Join(projectDir, "project.jsonl"), v2Data, 0o644)

	store, err := NewMemoryStore(globalDir, projectDir)
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}

	results := store.Recall("v2", ScopeProject, 10)
	if len(results) != 1 {
		t.Fatalf("v2 file should be readable, got %d results", len(results))
	}
	if len(results[0].Embedding) != 3 {
		t.Errorf("embedding should be loaded, got len=%d", len(results[0].Embedding))
	}
}

func TestPersistWritesV2Header(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewMemoryStore(filepath.Join(dir, "global"), filepath.Join(dir, "project"))

	_, _ = store.Save(Memory{Type: TypeProject, Content: "v2写入测试", Source: "test"}, ScopeProject)

	data, _ := os.ReadFile(filepath.Join(dir, "project", "project.jsonl"))
	firstLine := string(data[:bytes.IndexByte(data, '\n')])
	if firstLine != "{\"version\":2}" {
		t.Errorf("persisted file should have version 2 header, got: %s", firstLine)
	}
}

func TestSessionHook_BatchEmbed(t *testing.T) {
	dir := t.TempDir()
	emb := newFakeEmbedder(4)
	emb.vectors["记忆A"] = []float32{1, 0, 0, 0}
	emb.vectors["记忆B"] = []float32{0, 1, 0, 0}

	store, _ := NewMemoryStore(
		filepath.Join(dir, "global"),
		filepath.Join(dir, "project"),
		WithEmbedder(emb),
	)

	extractor := &batchTestExtractor{memories: []Memory{
		{Type: TypeProject, Content: "记忆A"},
		{Type: TypeProject, Content: "记忆B"},
	}}

	hook := NewSessionHook(store, extractor, WithSessionEmbedder(emb))
	hook.Extract([]schema.Message{{Role: schema.RoleUser, Content: "test"}})

	if emb.calls != 1 {
		t.Errorf("batch embed should make 1 API call, got %d", emb.calls)
	}

	results := store.Recall("", ScopeProject, 10)
	if len(results) != 2 {
		t.Errorf("should save 2 memories, got %d", len(results))
	}
	for _, r := range results {
		if len(r.Embedding) != 4 {
			t.Errorf("embedding should be set, got len=%d for %s", len(r.Embedding), r.Content)
		}
	}
}

type batchTestExtractor struct {
	memories []Memory
	err      error
}

func (f *batchTestExtractor) Extract(ctx context.Context, messages []schema.Message) ([]Memory, error) {
	return f.memories, f.err
}