package context

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/aethelgards/tiny-claw/internal/helper"
	"github.com/aethelgards/tiny-claw/internal/schema"
)

const (
	// DefaultContextWindow 未显式指定窗口时的默认上下文窗口（token）。
	DefaultContextWindow = 128_000
	// DefaultCompressRatio 默认触发压缩的窗口占用百分比。
	DefaultCompressRatio = 80
	// summaryPrefix 摘要消息的内容前缀，用于标识与日志。
	summaryPrefix = "【历史会话摘要】\n"
)

// Summarizer 把被丢弃的旧消息（连同已有摘要）压缩成一条新摘要。
// 返回错误时调用方回退为纯截断。
type Summarizer func(ctx context.Context, existingSummary string, msgs []schema.Message) (string, error)

// Option 为 newSession / LoadSession 的构造选项。
type Option func(*Session)

// Session 管理一次 LLM 会话的对话历史与窗口压缩。
// 只存对话消息（User/Assistant/Tool 结果），system 提示词由使用方自行组装。
// 当历史估算 token 超过 contextWindow × compressRatio / 100 时，
// Append 内主动把最旧的超窗消息压缩为一条摘要（混合式：摘要 + 保留最近原始消息）。
type Session struct {
	ID        string
	WorkDir   string
	CreatedAt time.Time
	UpdatedAt time.Time

	history []schema.Message // 仅原始对话消息，不含摘要
	summary string           // 压缩摘要（可为空）

	contextWindow int  // 模型上下文窗口总 token 数
	compressRatio int  // 触发百分比 1~100
	windowSet     bool // 是否显式设置过窗口（WithContextWindow 优先于 WithModel）

	summarizer Summarizer // 摘要生成函数，nil 时压缩退化为纯截断

	TotalCompletionTokens int64
	TotalPromptTokens     int64
	TotalCostCNY          float64

	Mu sync.Mutex
}

func NewSession(sessionID, workDir string, opts ...Option) *Session {
	s := &Session{
		ID:            sessionID,
		WorkDir:       workDir,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		history:       []schema.Message{},
		contextWindow: DefaultContextWindow,
		compressRatio: DefaultCompressRatio,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Session) RecordUsage(prompt int64, completion int64, cost float64) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.TotalPromptTokens += prompt
	s.TotalCostCNY += cost
	s.TotalCompletionTokens += completion
}

// LoadSession 从磁盘加载既有会话；文件不存在时返回空会话。
func LoadSession(sessionID, workDir string, opts ...Option) (*Session, error) {
	s := NewSession(sessionID, workDir, opts...)
	data, err := os.ReadFile(filepath.Join(workDir, ".claw", "sessions", sessionID+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var probe struct {
			Summary string `json:"summary"`
			Role    string `json:"role"`
		}
		if err := json.Unmarshal([]byte(line), &probe); err != nil {
			slog.Warn("load session: skip invalid line", slog.String("err", err.Error()))
			continue
		}
		if probe.Summary != "" {
			s.summary = probe.Summary
			continue
		}
		var msg schema.Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			slog.Warn("load session: skip invalid message line", slog.String("err", err.Error()))
			continue
		}
		if msg.Role == schema.RoleSystem {
			slog.Warn("load session: skip system message line", slog.String("sessionID", s.ID))
			continue
		}
		s.history = append(s.history, msg)
	}
	return s, nil
}

// LookupContextWindow 按模型名返回近似上下文窗口（token）。
// 子串匹配、大小写不敏感、首个命中生效；未知模型返回默认窗口。
func LookupContextWindow(model string) int {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "claude"):
		return 200_000
	case strings.Contains(m, "glm-4"):
		return 128_000
	case strings.Contains(m, "gpt-4o"), strings.Contains(m, "gpt-4.1"):
		return 128_000
	case strings.Contains(m, "deepseek"):
		return 128_000
	case strings.Contains(m, "qwen"):
		return 128_000
	default:
		return DefaultContextWindow
	}
}

// WithContextWindow 显式指定模型上下文窗口（token），优先级高于 WithModel。
func WithContextWindow(tokens int) Option {
	return func(s *Session) {
		if tokens > 0 {
			s.contextWindow = tokens
			s.windowSet = true
		}
	}
}

// WithCompressRatio 设置触发压缩的窗口占用百分比（钳制到 1~100）。
func WithCompressRatio(pct int) Option {
	return func(s *Session) {
		if pct < 1 {
			pct = 1
		}
		if pct > 100 {
			pct = 100
		}
		s.compressRatio = pct
	}
}

// WithModel 按模型名查表设定上下文窗口；仅当未显式 WithContextWindow 时生效。
func WithModel(model string) Option {
	return func(s *Session) {
		if !s.windowSet {
			s.contextWindow = LookupContextWindow(model)
		}
	}
}

// WithSummarizer 注入摘要生成函数；nil 时压缩退化为纯截断。
func WithSummarizer(fn Summarizer) Option {
	return func(s *Session) { s.summarizer = fn }
}

// Append 追加消息并落盘；累计估算 token 超过阈值时触发窗口压缩。
// system 提示词不属于会话历史（由使用方单独组装），误入时丢弃并告警。
func (s *Session) Append(ctx context.Context, msgs ...schema.Message) {
	msgs = s.dropSystem(ctx, msgs)
	if len(msgs) == 0 {
		return
	}
	s.Mu.Lock()
	s.history = append(s.history, msgs...)
	s.UpdatedAt = time.Now()
	s.saveToDisk(ctx, msgs)
	s.Mu.Unlock()

	if s.totalTokens() > s.threshold() {
		s.compress(ctx)
	}
}

// dropSystem 过滤掉误入的 system 提示词，保持 history 只含对话消息。
func (s *Session) dropSystem(ctx context.Context, msgs []schema.Message) []schema.Message {
	hasSystem := false
	for _, m := range msgs {
		if m.Role == schema.RoleSystem {
			hasSystem = true
			break
		}
	}
	if !hasSystem {
		return msgs
	}
	out := make([]schema.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == schema.RoleSystem {
			slog.WarnContext(ctx, "session: drop system message, system prompt must be assembled outside session",
				slog.String("sessionID", s.ID))
			continue
		}
		out = append(out, m)
	}
	return out
}

// GetWorkingMemory 返回可直接喂给 LLM 的会话上下文：
// 摘要置于最前（RoleUser）；history 以 User 开头时摘要前置合并进该消息，
// 保证输出序列不出现两个连续 User。
func (s *Session) GetWorkingMemory(ctx context.Context) []schema.Message {
	_ = ctx
	s.Mu.Lock()
	defer s.Mu.Unlock()

	out := cloneMsgs(s.history)
	if s.summary == "" {
		return out
	}
	summaryMsg := schema.Message{Role: schema.RoleUser, Content: summaryPrefix + s.summary}
	if len(out) == 0 {
		return []schema.Message{summaryMsg}
	}
	if out[0].Role == schema.RoleUser {
		out[0].Content = summaryMsg.Content + "\n\n" + out[0].Content
		return out
	}
	return append([]schema.Message{summaryMsg}, out...)
}

// totalTokens 估算整个历史的 token 数。
func (s *Session) totalTokens() int {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	n := 0
	for _, m := range s.history {
		n += estTokens(m)
	}
	return n
}

// threshold 触发压缩的阈值：contextWindow × compressRatio / 100。
func (s *Session) threshold() int {
	return s.contextWindow * s.compressRatio / 100
}

// Threshold 返回触发压缩的阈值（contextWindow × compressRatio / 100）。
func (s *Session) Threshold() int { return s.threshold() }

// Summary 返回当前压缩摘要。
func (s *Session) Summary() string { return s.summary }

// SetSummary 设置压缩摘要（供测试使用）。
func (s *Session) SetSummary(v string) { s.summary = v }

// History 返回当前会话历史的副本。
func (s *Session) History() []schema.Message { return cloneMsgs(s.history) }

// TotalTokens 估算整个历史的 token 数（导出供测试使用）。
func (s *Session) TotalTokens() int { return s.totalTokens() }

// estTokens 单条消息的保守 token 估算：固定开销 + 字符数/2。
func estTokens(msg schema.Message) int {
	n := 4
	n += utf8.RuneCountInString(msg.Content) / 2
	for _, tc := range msg.ToolCalls {
		n += utf8.RuneCountInString(string(tc.Arguments)) / 2
	}
	return n
}

// compress 把最旧的超窗前缀压缩为摘要，保留最近原始消息。
// 触发条件由调用方保证（totalTokens > threshold）。
func (s *Session) compress(ctx context.Context) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	// 摘要预留 threshold/8，其余留给最近原始消息
	budget := s.threshold() - s.threshold()/8

	// 从末尾向前累计，找到最靠前的、满足预算的后缀起点 cutoff
	suffix := 0
	cutoff := len(s.history)
	for i := len(s.history) - 1; i >= 0; i-- {
		if suffix+estTokens(s.history[i]) > budget {
			break
		}
		suffix += estTokens(s.history[i])
		cutoff = i
	}

	// 整个历史都不超预算：无需压缩
	if cutoff == 0 {
		return
	}
	// 单个回合本身就超预算（连最后一条消息都放不进预算）：接受超限，避免死循环
	if cutoff == len(s.history) {
		slog.WarnContext(ctx, "session compress skipped: single turn exceeds budget",
			slog.String("sessionID", s.ID))
		return
	}

	// 边界修正：后缀起点若是孤儿工具结果（其 assistant 已在前缀被丢弃），
	// 前移到下一个非工具结果消息；若因此清空后缀则放弃压缩（保住当前回合）。
	for cutoff < len(s.history) && s.history[cutoff].ToolCallID != "" {
		cutoff++
	}
	if cutoff >= len(s.history) {
		slog.WarnContext(ctx, "session compress skipped: boundary fix would drop current turn",
			slog.String("sessionID", s.ID))
		return
	}

	dropped := s.history[:cutoff]
	s.history = append([]schema.Message{}, s.history[cutoff:]...)

	if s.summarizer != nil {
		newSummary, err := s.summarizer(ctx, s.summary, dropped)
		if err != nil {
			slog.WarnContext(ctx, "session summarizer failed, fallback to truncation",
				slog.String("err", err.Error()))
			newSummary = ""
		}
		s.summary = newSummary
	} else {
		s.summary = ""
	}

	if err := s.RewriteFile(); err != nil {
		slog.WarnContext(ctx, "session rewrite failed", slog.String("err", err.Error()))
	}
}

// saveToDisk 以 JSONL 追加方式增量落盘。
func (s *Session) saveToDisk(ctx context.Context, msgs []schema.Message) {
	sessionFile := filepath.Join(s.WorkDir, ".claw", "sessions", s.ID+".json")
	for _, msg := range msgs {
		line := helper.Any2Json(msg)
		if err := helper.AppendLine(sessionFile, line); err != nil {
			slog.WarnContext(ctx, "save to disk failed", slog.String("err", err.Error()))
		}
	}
}

// RewriteFile 原子重写整个会话文件：摘要记录 + 原始消息。
func (s *Session) RewriteFile() error {
	file := filepath.Join(s.WorkDir, ".claw", "sessions", s.ID+".json")
	var sb strings.Builder
	if s.summary != "" {
		sb.WriteString(helper.Any2Json(map[string]string{"summary": s.summary}))
		sb.WriteByte('\n')
	}
	for _, msg := range s.history {
		sb.WriteString(helper.Any2Json(msg))
		sb.WriteByte('\n')
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, []byte(sb.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

func cloneMsgs(msgs []schema.Message) []schema.Message {
	out := make([]schema.Message, len(msgs))
	copy(out, msgs)
	return out
}
