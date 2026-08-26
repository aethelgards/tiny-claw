package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

func TestSessionHook_Extract_SavesMemoriesWithScope(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	projectDir := filepath.Join(dir, "project")

	store, _ := NewMemoryStore(globalDir, projectDir)

	fakeProvider := &fakeMemoryProvider{
		response: `[
			{"type":"project","content":"本项目用 Go 1.26 + gin"},
			{"type":"preferences","content":"用户偏好中文回复"},
			{"type":"errors","content":"错误：bash 工具输出被截断到 8KB，解决方法是用 read_file"},
			{"type":"tools","content":"修改文件后用 go build ./... 验证编译"}
		]`,
	}

	extractor := NewLLMExtractor(fakeProvider)
	hook := NewSessionHook(store, extractor)

	dropped := []schema.Message{
		{Role: schema.RoleUser, Content: "给这个项目添加单元测试"},
		{Role: schema.RoleAssistant, Content: "好的，我来帮你添加测试"},
		{Role: schema.RoleUser, Content: "bash 工具输出被截断"},
		{Role: schema.RoleAssistant, Content: "用 read_file 读取大文件"},
	}

	hook.Extract(dropped)

	proj := store.Recall("项目", ScopeProject, 10)
	if len(proj) == 0 {
		t.Errorf("project memories not saved")
	}

	prefs := store.Recall("偏好", ScopeGlobal, 10)
	if len(prefs) == 0 {
		t.Errorf("global preferences not saved")
	}

	errs := store.Recall("错误", ScopeProject, 10)
	if len(errs) == 0 {
		t.Errorf("errors not saved")
	}

	tools := store.Recall("工具", ScopeProject, 10)
	if len(tools) == 0 {
		t.Errorf("tools not saved")
	}
}

func TestMemoryExtractor_Extract_EmptyResponse(t *testing.T) {
	fakeProvider := &fakeMemoryProvider{response: "[]"}
	extractor := NewLLMExtractor(fakeProvider)

	dropped := []schema.Message{{Role: schema.RoleUser, Content: "无关消息"}}
	memories, err := extractor.Extract(context.Background(), dropped)
	if err != nil {
		t.Fatalf("Extract should not error on empty response: %v", err)
	}
	if len(memories) != 0 {
		t.Errorf("should have no memories from empty response, got %d", len(memories))
	}
}

func TestMemoryExtractor_Extract_InvalidJSON(t *testing.T) {
	fakeProvider := &fakeMemoryProvider{response: "not json"}
	extractor := NewLLMExtractor(fakeProvider)

	dropped := []schema.Message{{Role: schema.RoleUser, Content: "test"}}
	memories, err := extractor.Extract(context.Background(), dropped)
	if err == nil {
		t.Errorf("Extract should return error on invalid JSON")
	}
	if memories != nil {
		t.Errorf("memories should be nil on parse failure")
	}
}

func TestMemoryExtractor_Extract_FiltersInvalidItems(t *testing.T) {
	fakeProvider := &fakeMemoryProvider{
		response: `[
			{"type":"unknown","content":"非法类型应被过滤"},
			{"type":"project","content":""},
			{"type":"project","content":"有效记忆"}
		]`,
	}
	extractor := NewLLMExtractor(fakeProvider)

	dropped := []schema.Message{{Role: schema.RoleUser, Content: "test"}}
	memories, err := extractor.Extract(context.Background(), dropped)
	if err != nil {
		t.Fatalf("Extract should not error: %v", err)
	}
	if len(memories) != 1 || memories[0].Type != TypeProject || memories[0].Content != "有效记忆" {
		t.Errorf("expected single valid memory, got %+v", memories)
	}
}

func TestMemoryExtractor_Timeout(t *testing.T) {
	fakeProvider := &fakeMemoryProvider{
		response: `[]`,
		delay:    200 * time.Millisecond,
	}
	extractor := NewLLMExtractor(fakeProvider)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	dropped := []schema.Message{{Role: schema.RoleUser, Content: "test"}}
	memories, err := extractor.Extract(ctx, dropped)
	if err == nil {
		t.Errorf("expected timeout error")
	}
	if memories != nil {
		t.Errorf("expected nil memories on timeout")
	}
}

type fakeMemoryProvider struct {
	response string
	delay    time.Duration
}

func (f *fakeMemoryProvider) Generate(ctx context.Context, messages []schema.Message, availableTools []schema.ToolDefinition) (*schema.Message, error) {
	if f.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(f.delay):
		}
	}
	return &schema.Message{
		Role:    schema.RoleAssistant,
		Content: f.response,
	}, nil
}
