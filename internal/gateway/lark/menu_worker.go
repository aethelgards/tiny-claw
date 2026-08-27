package lark

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/aethelgards/tiny-claw/internal/engine"
)

// MenuProcessor 消费端处理单元，由装配方注入（测试时可注入 fake）。
type MenuProcessor interface {
	ProcessMenu(ctx context.Context, event IncomingMenuEvent) error
}

// MenuWorker 单 goroutine 串行消费队列菜单事件。
// 同一时间只处理一条消息，天然保证处理顺序；
// ctx 取消时处理完当前消息后退出。
type MenuWorker struct {
	ch        chan IncomingMenuEvent
	processor MenuProcessor
}

// NewMenuWorker 创建 MenuWorker。
func NewMenuWorker(bufferSize int, p MenuProcessor) *MenuWorker {
	return &MenuWorker{
		ch:        make(chan IncomingMenuEvent, bufferSize),
		processor: p,
	}
}

// Enqueue 非阻塞入队。返回 false 表示队列已满。
func (w *MenuWorker) Enqueue(event IncomingMenuEvent) bool {
	select {
	case w.ch <- event:
		return true
	default:
		return false
	}
}

// Run 阻塞消费直到 ctx 取消。处理完当前事件才退出。
func (w *MenuWorker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "menu worker stopped by ctx cancel")
			return
		case evt, ok := <-w.ch:
			if !ok {
				return
			}
			w.safeProcess(ctx, evt)
		}
	}
}

// safeProcess 包裹 processor.ProcessMenu，捕获 panic 保证 worker 不因单个事件崩溃。
func (w *MenuWorker) safeProcess(ctx context.Context, event IncomingMenuEvent) {
	slog.InfoContext(ctx, "menu worker processing event",
		slog.String("menuKey", event.MenuKey),
		slog.String("openID", event.OpenID))

	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "menu worker recovered panic",
				slog.String("menuKey", event.MenuKey),
				slog.Any("panic", r))
		}
	}()

	if err := w.processor.ProcessMenu(ctx, event); err != nil {
		slog.ErrorContext(ctx, "process menu event failed",
			slog.String("menuKey", event.MenuKey),
			slog.String("openID", event.OpenID),
			slog.String("err", err.Error()))
	}
}

// OpenIDToChatIDMapping 维护 open_id -> chat_id 的映射（仅用于私聊场景）。
// 当用户在私聊中发送消息时，记录此映射；当用户点击菜单时，通过此映射找到 chat_id。
// 映射支持持久化到磁盘，避免服务重启后丢失（否则用户点击"新建会话"将找不到 chat_id）。
type OpenIDToChatIDMapping struct {
	mu       sync.RWMutex
	mappings map[string]string // open_id -> chat_id
	path     string            // 持久化文件路径，为空时不落盘
}

// NewOpenIDToChatIDMapping 创建纯内存映射管理器（不持久化）。
func NewOpenIDToChatIDMapping() *OpenIDToChatIDMapping {
	return &OpenIDToChatIDMapping{
		mappings: make(map[string]string),
	}
}

// NewOpenIDToChatIDMappingWithPath 创建映射管理器并启用持久化。
// 启动时应先调用 Load() 恢复既有映射。
func NewOpenIDToChatIDMappingWithPath(path string) *OpenIDToChatIDMapping {
	return &OpenIDToChatIDMapping{
		mappings: make(map[string]string),
		path:     path,
	}
}

// Set 记录 open_id -> chat_id 的映射；启用持久化时同步原子落盘。
// 落盘失败仅告警，不影响内存映射（下次 Set 会重试）。
func (m *OpenIDToChatIDMapping) Set(openID, chatID string) {
	if openID == "" || chatID == "" {
		return
	}
	m.mu.Lock()
	m.mappings[openID] = chatID
	m.mu.Unlock()

	if m.path != "" {
		if err := m.save(); err != nil {
			slog.Warn("openid chatid mapping save failed",
				slog.String("path", m.path), slog.String("err", err.Error()))
		}
	}
}

// Get 根据 open_id 获取 chat_id。
func (m *OpenIDToChatIDMapping) Get(openID string) (string, bool) {
	m.mu.RLock()
	chatID, ok := m.mappings[openID]
	m.mu.RUnlock()
	return chatID, ok
}

// Load 从磁盘读取既有映射；文件不存在时静默返回（视为首次启动）。
// 启用持久化（path 非空）时应在服务启动时调用。
func (m *OpenIDToChatIDMapping) Load() error {
	if m.path == "" {
		return nil
	}
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	restored := make(map[string]string)
	if len(data) > 0 {
		if err := json.Unmarshal(data, &restored); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.mappings = restored
	m.mu.Unlock()
	slog.Info("openid chatid mapping loaded",
		slog.String("path", m.path), slog.Int("count", len(restored)))
	return nil
}

// save 以 JSON 原子落盘（tmp + rename），避免写一半导致文件损坏。
// 调用方需保证 path 非空。
func (m *OpenIDToChatIDMapping) save() error {
	m.mu.RLock()
	data := make(map[string]string, len(m.mappings))
	for k, v := range m.mappings {
		data[k] = v
	}
	m.mu.RUnlock()

	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

// MenuEngineProcessor 处理菜单事件，清空会话并回复确认。
type MenuEngineProcessor struct {
	bot     *Bot
	mapping *OpenIDToChatIDMapping
}

// NewMenuEngineProcessor 创建 MenuEngineProcessor。
func NewMenuEngineProcessor(bot *Bot, mapping *OpenIDToChatIDMapping) *MenuEngineProcessor {
	return &MenuEngineProcessor{
		bot:     bot,
		mapping: mapping,
	}
}

// ProcessMenu 处理菜单事件。
func (p *MenuEngineProcessor) ProcessMenu(ctx context.Context, event IncomingMenuEvent) error {
	switch event.MenuKey {
	case "new_session":
		return p.handleNewSession(ctx, event)
	default:
		slog.WarnContext(ctx, "unknown menu key",
			slog.String("menuKey", event.MenuKey))
		return nil
	}
}

// handleNewSession 处理"新建会话"菜单事件。
func (p *MenuEngineProcessor) handleNewSession(ctx context.Context, event IncomingMenuEvent) error {
	// 通过 open_id 查找对应的 chat_id
	chatID, ok := p.mapping.Get(event.OpenID)
	if !ok {
		slog.WarnContext(ctx, "cannot find chat_id for open_id",
			slog.String("openID", event.OpenID))
		return nil
	}

	// 删除会话
	engine.GlobalSessionMessage.Delete(chatID)

	// 回复确认消息
	reply := "✅ 会话已清空，可以开始新的对话了"
	if err := p.bot.SendMessage(ctx, chatID, event.TenantKey, reply); err != nil {
		slog.ErrorContext(ctx, "send menu reply failed",
			slog.String("chatID", chatID),
			slog.String("err", err.Error()))
		return err
	}

	slog.InfoContext(ctx, "session cleared via menu",
		slog.String("chatID", chatID),
		slog.String("openID", event.OpenID))

	return nil
}
