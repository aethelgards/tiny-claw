package memory

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

// 非 *LLMExtractor 的 MemoryExtractor 实现（模拟 mock / 其它实现）
type fakeExtractor struct{}

func (f *fakeExtractor) Extract(ctx context.Context, messages []schema.Message) ([]Memory, error) {
	return nil, nil
}

// recordingExtractor 记录自身是否被调用，用于验证跳过逻辑
type recordingExtractor struct{ called bool }

func (f *recordingExtractor) Extract(ctx context.Context, messages []schema.Message) ([]Memory, error) {
	f.called = true
	return nil, nil
}

func assertNoPanic(t *testing.T) func() {
	t.Helper()
	return func() {
		if r := recover(); r != nil {
			t.Errorf("SessionHook.Extract panicked: %v", r)
		}
	}
}

// 回归：非 *LLMExtractor 实现不得 panic 击穿进程（该调用位于 goroutine 内）。
func TestSessionHook_Extract_NoPanicOnNonLLMExtractor(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")
	store, _ := NewMemoryStore(globalDir, projectDir)

	hook := NewSessionHook(store, &fakeExtractor{})

	defer assertNoPanic(t)()
	hook.Extract([]schema.Message{{Role: schema.RoleUser, Content: "hello"}})
}

// 回归：store 未装配（SetMemoryStore 未调用 / NewSessionHook(nil,...)）时
// 不得 panic，且应直接跳过而不发起提取调用。
func TestSessionHook_Extract_NilStoreSkipsExtraction(t *testing.T) {
	ex := &recordingExtractor{}
	hook := NewSessionHook(nil, ex)

	defer assertNoPanic(t)()
	hook.Extract([]schema.Message{{Role: schema.RoleUser, Content: "hello"}})

	if ex.called {
		t.Errorf("extractor must not be invoked when store is nil")
	}
}

// 回归：Save 对非法 scope 应返回错误而非 nil map 写入 panic。
func TestMemoryStore_Save_InvalidScopeReturnsError(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")
	store, _ := NewMemoryStore(globalDir, projectDir)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Save panicked on invalid scope: %v", r)
		}
	}()
	if _, err := store.Save(Memory{Type: TypeProject, Content: "测试"}, Scope("invalid")); err == nil {
		t.Errorf("expected error for invalid scope, got nil")
	}
}
