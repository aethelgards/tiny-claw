package engine

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/aethelgards/tiny-claw/internal/context"
)

// SessionMessage 按 sessionID 管理进程内的 Session 注册表，并发安全。
// 会话由 Session 自身增量落盘（JSONL）；GetOrCreate 在内存未命中时经
// LoadSession 从磁盘恢复，进程重启后多轮会话可续聊。
type SessionMessage struct {
	sessions map[string]*context.Session
	mu       sync.Mutex
}

// NewSessionMessage 创建一个空的会话管理器实例（测试或独立使用）。
func NewSessionMessage() *SessionMessage {
	return &SessionMessage{sessions: make(map[string]*context.Session)}
}

// GlobalSessionMessage 进程级会话管理器单例。
var GlobalSessionMessage = NewSessionMessage()

// GetOrCreate 返回已注册的会话；未注册时从磁盘加载（文件不存在则新建）并注册。
// opts 透传给 LoadSession / NewSession，用于注入 WithSummarizer 等构造选项。
func (sm *SessionMessage) GetOrCreate(sessionID string, workDir string, opts ...context.Option) *context.Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if session, ok := sm.sessions[sessionID]; ok {
		return session
	}
	session, err := context.LoadSession(sessionID, workDir, opts...)
	if err != nil {
		// 磁盘读取失败（非文件不存在）：降级为空会话，不阻塞会话获取
		slog.Warn("session load failed, fallback to empty session",
			slog.String("sessionID", sessionID), slog.String("err", err.Error()))
		session = context.NewSession(sessionID, workDir, opts...)
	}
	sm.sessions[sessionID] = session
	return session
}

// Delete 注销会话并删除其磁盘文件，实现"新会话/重置"语义。
// 磁盘文件在会话锁内删除，避免与并发的 Append 落盘竞争产生撕裂文件；
// 文件不存在（从未落盘）时静默忽略。
// 注意：删除后调用方不应再持有该 Session 引用继续使用。
func (sm *SessionMessage) Delete(sessionID string) {
	sm.mu.Lock()
	session, ok := sm.sessions[sessionID]
	delete(sm.sessions, sessionID)
	sm.mu.Unlock()

	if !ok {
		return
	}
	session.Mu.Lock()
	defer session.Mu.Unlock()
	path := filepath.Join(session.WorkDir, "sessions", sessionID+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("session delete: remove file failed",
			slog.String("sessionID", sessionID), slog.String("err", err.Error()))
	}
}

// List 返回全部活跃会话 ID（排序，保证确定性输出）。
func (sm *SessionMessage) List() []string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	ids := make([]string, 0, len(sm.sessions))
	for id := range sm.sessions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Len 返回当前活跃会话数。
func (sm *SessionMessage) Len() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return len(sm.sessions)
}

// Flush 将所有活跃会话原子重写落盘（优雅停机前调用）。
// 即使会话从未 Append（无增量文件），也会被写出为空会话文件。
func (sm *SessionMessage) Flush() {
	sm.mu.Lock()
	sessions := make([]*context.Session, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		sessions = append(sessions, s)
	}
	sm.mu.Unlock()

	for _, s := range sessions {
		s.Mu.Lock()
		err := s.RewriteFile()
		s.Mu.Unlock()
		if err != nil {
			slog.Warn("session flush failed",
				slog.String("sessionID", s.ID), slog.String("err", err.Error()))
		}
	}
}

// Close 先落盘全部会话，再清空注册表（优雅停机）。
func (sm *SessionMessage) Close() {
	sm.Flush()
	sm.mu.Lock()
	sm.sessions = make(map[string]*context.Session)
	sm.mu.Unlock()
}
