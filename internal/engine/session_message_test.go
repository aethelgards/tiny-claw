package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

// ---- 测试辅助 ----

// appendMsg 追加一条 User 消息并落盘，保证磁盘存在会话文件。
func appendMsg(t *testing.T, s *Session, content string) {
	t.Helper()
	s.Append(context.Background(), schema.Message{Role: schema.RoleUser, Content: content})
}

func sessionFile(t *testing.T, workDir, sessionID string) string {
	t.Helper()
	return filepath.Join(workDir, "sessions", sessionID+".json")
}

// ---- GetOrCreate ----

func TestSessionMessageGetOrCreateCreatesAndReuses(t *testing.T) {
	sm := NewSessionMessage()
	workDir := t.TempDir()

	a1 := sm.GetOrCreate("chat-1", workDir)
	a2 := sm.GetOrCreate("chat-1", workDir)
	if a1 != a2 {
		t.Fatalf("GetOrCreate same ID returned different instances")
	}
	b := sm.GetOrCreate("chat-2", workDir)
	if a1 == b {
		t.Fatalf("GetOrCreate different IDs returned same instance")
	}
}

func TestSessionMessageGetOrCreateLoadsFromDisk(t *testing.T) {
	workDir := t.TempDir()

	first := NewSessionMessage()
	s := first.GetOrCreate("chat-1", workDir)
	appendMsg(t, s, "你好")

	// 新管理器（模拟进程重启）同 ID 应经 LoadSession 恢复历史
	second := NewSessionMessage()
	restored := second.GetOrCreate("chat-1", workDir)
	mem := restored.GetWorkingMemory(context.Background())
	if len(mem) != 1 || mem[0].Content != "你好" {
		t.Fatalf("restored history mismatch: %+v", mem)
	}
}

// ---- Delete ----

func TestSessionMessageDeleteRemovesMemoryAndFile(t *testing.T) {
	sm := NewSessionMessage()
	workDir := t.TempDir()

	s := sm.GetOrCreate("chat-1", workDir)
	appendMsg(t, s, "hello")
	file := sessionFile(t, workDir, "chat-1")
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("session file should exist before delete: %v", err)
	}

	sm.Delete("chat-1")

	if sm.Len() != 0 {
		t.Fatalf("session should be removed from registry after delete, Len=%d", sm.Len())
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("session file should be removed after delete, got err=%v", err)
	}

	// 删除后重新 GetOrCreate 得到全新会话（无历史）
	recreated := sm.GetOrCreate("chat-1", workDir)
	if mem := recreated.GetWorkingMemory(context.Background()); len(mem) != 0 {
		t.Fatalf("recreated session should start empty, got %+v", mem)
	}
}

func TestSessionMessageDeleteUnknownNoPanic(t *testing.T) {
	sm := NewSessionMessage()
	sm.Delete("never-existed")
}

func TestSessionMessageDeleteNoFileNoPanic(t *testing.T) {
	sm := NewSessionMessage()
	workDir := t.TempDir()
	sm.GetOrCreate("chat-1", workDir) // 从未 Append，无磁盘文件
	sm.Delete("chat-1")
}

// ---- List / Len ----

func TestSessionMessageListSorted(t *testing.T) {
	sm := NewSessionMessage()
	workDir := t.TempDir()
	for _, id := range []string{"chat-c", "chat-a", "chat-b"} {
		sm.GetOrCreate(id, workDir)
	}
	got := sm.List()
	want := []string{"chat-a", "chat-b", "chat-c"}
	if len(got) != len(want) {
		t.Fatalf("List length mismatch: got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("List not sorted: got %v, want %v", got, want)
		}
	}
}

func TestSessionMessageLen(t *testing.T) {
	sm := NewSessionMessage()
	workDir := t.TempDir()
	if sm.Len() != 0 {
		t.Fatalf("fresh manager should have 0 sessions")
	}
	sm.GetOrCreate("chat-1", workDir)
	sm.GetOrCreate("chat-2", workDir)
	sm.GetOrCreate("chat-1", workDir) // 复用不新增
	if sm.Len() != 2 {
		t.Fatalf("Len = %d, want 2", sm.Len())
	}
	sm.Delete("chat-1")
	if sm.Len() != 1 {
		t.Fatalf("Len = %d, want 1 after delete", sm.Len())
	}
}

// ---- Flush / Close ----

func TestSessionMessageFlushPersistsAll(t *testing.T) {
	workDir := t.TempDir()

	first := NewSessionMessage()
	s1 := first.GetOrCreate("chat-1", workDir)
	appendMsg(t, s1, "msg-1")
	first.GetOrCreate("chat-2", workDir) // 仅内存，未 Append
	first.Flush()

	for _, id := range []string{"chat-1", "chat-2"} {
		if _, err := os.Stat(sessionFile(t, workDir, id)); err != nil {
			t.Fatalf("Flush should persist session %s: %v", id, err)
		}
	}

	second := NewSessionMessage()
	r1 := second.GetOrCreate("chat-1", workDir)
	mem := r1.GetWorkingMemory(context.Background())
	if len(mem) != 1 || mem[0].Content != "msg-1" {
		t.Fatalf("flushed session history lost: %+v", mem)
	}
}

func TestSessionMessageClosePersistsAndClears(t *testing.T) {
	workDir := t.TempDir()

	sm := NewSessionMessage()
	s := sm.GetOrCreate("chat-1", workDir)
	appendMsg(t, s, "hello")
	sm.Close()

	if sm.Len() != 0 || len(sm.List()) != 0 {
		t.Fatalf("Close should clear registry: Len=%d List=%v", sm.Len(), sm.List())
	}
	if _, err := os.Stat(sessionFile(t, workDir, "chat-1")); err != nil {
		t.Fatalf("Close should persist before clearing: %v", err)
	}

	// 落盘文件仍在，可被新管理器恢复
	second := NewSessionMessage()
	restored := second.GetOrCreate("chat-1", workDir)
	if mem := restored.GetWorkingMemory(context.Background()); len(mem) != 1 {
		t.Fatalf("session lost after Close: %+v", mem)
	}
}

// ---- 构造选项透传 ----

func TestGetOrCreateWiresSummarizer(t *testing.T) {
	sm := NewSessionMessage()
	workDir := t.TempDir()

	var gotOld string
	var gotDropped []schema.Message
	sess := sm.GetOrCreate("chat-1", workDir,
		WithContextWindow(1000),
		WithSummarizer(fakeSummarizer(t, &gotOld, &gotDropped, "sum", nil)),
	)

	// 4 条 400 字符消息：estTokens = 4 × 204 = 816 > threshold(1000×80%=800)
	for i := 0; i < 4; i++ {
		sess.Append(context.Background(), contentMsg(schema.RoleUser, 400))
	}

	mem := sess.GetWorkingMemory(context.Background())
	if len(mem) == 0 || !strings.Contains(mem[0].Content, "sum") {
		t.Fatalf("summary should be generated and placed first, got %+v", mem)
	}
	if gotOld != "" {
		t.Fatalf("first compression old summary should be empty, got %q", gotOld)
	}
	if len(gotDropped) == 0 {
		t.Fatalf("summarizer should receive dropped messages")
	}
}

// ---- 并发 ----

// TestSessionMessageConcurrentGetOrCreate 并发 GetOrCreate 同一 ID 只产生一个实例。
func TestSessionMessageConcurrentGetOrCreate(t *testing.T) {
	sm := NewSessionMessage()
	workDir := t.TempDir()

	const n = 32
	var wg sync.WaitGroup
	instances := make([]*Session, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			instances[i] = sm.GetOrCreate("chat-1", workDir)
		}(i)
	}
	wg.Wait()
	for i := 1; i < n; i++ {
		if instances[i] != instances[0] {
			t.Fatalf("concurrent GetOrCreate produced distinct instances")
		}
	}
}
