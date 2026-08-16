package approval

import (
	"context"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/aethelgards/tiny-claw/internal/reporter"
)

type ApprovalResult struct {
	Allowed      bool
	RejectReason string
}

var dangerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+-r`),
	regexp.MustCompile(`sudo`),
	regexp.MustCompile(`\bdrop\b`),
	regexp.MustCompile(`>.*\.go`),
}

// isDangerousCommand 判断工具调用是否命中高危模式；仅 bash 参与判断。
func isDangerousCommand(toolName string, args string) bool {
	if toolName != "bash" {
		return false
	}
	for _, re := range dangerPatterns {
		if re.MatchString(args) {
			return true
		}
	}
	return false
}

// Task 单个待审批任务：结果 channel + 元数据（结果卡展示/身份校验用）。
type Task struct {
	ch         chan ApprovalResult
	TaskID     string
	ApproverID string // 谁发的请求，只有 TA 能批
	ToolName   string
	Args       string
}

type ApprovalManager struct {
	mu           sync.RWMutex
	pendingTasks map[string]Task
	timeout      time.Duration
}

// NewApprovalManager 创建审批管理器；timeout <= 0 视为默认 5 分钟。
func NewApprovalManager(timeout time.Duration) *ApprovalManager {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &ApprovalManager{
		pendingTasks: make(map[string]Task),
		timeout:      timeout,
	}
}

// WaitingForApproval 注册审批任务并阻塞等待结论。
// 返回 (allowed, reason)；reason 直接作为中间件拒绝文案。
// 所有出口（成功/发送失败/超时/ctx 取消）都会清理任务。
func (m *ApprovalManager) WaitingForApproval(ctx context.Context, taskID, toolName, args string, reporter reporter.Reporter, approverID string) (bool, string) {
	task := Task{
		ch:         make(chan ApprovalResult, 1),
		TaskID:     taskID,
		ApproverID: approverID,
		ToolName:   toolName,
		Args:       args,
	}
	m.mu.Lock()
	m.pendingTasks[taskID] = task
	m.mu.Unlock()

	// 通知发送失败（含 reporter 缺失）→ fail-closed：清理任务并拒绝执行
	if reporter == nil {
		slog.ErrorContext(ctx, "approval reporter is nil", slog.String("taskId", taskID))
		m.deleteTask(taskID)
		return false, "审批通知发送失败，已拒绝执行"
	}
	if err := reporter.SendApprovalMessage(ctx, taskID, toolName, args); err != nil {
		slog.ErrorContext(ctx, "sendApprovalMessage",
			slog.String("taskId", taskID), slog.String("toolName", toolName),
			slog.String("args", args), slog.String("err", err.Error()))
		m.deleteTask(taskID)
		return false, "审批通知发送失败，已拒绝执行"
	}

	select {
	case res := <-task.ch:
		m.deleteTask(taskID)
		return res.Allowed, res.RejectReason
	case <-time.After(m.timeout):
		slog.WarnContext(ctx, "approval timeout",
			slog.String("taskId", taskID), slog.Duration("timeout", m.timeout))
		m.deleteTask(taskID)
		return false, "审批超时，已自动拒绝"
	case <-ctx.Done():
		slog.WarnContext(ctx, "approval ctx canceled",
			slog.String("taskId", taskID), slog.String("err", ctx.Err().Error()))
		m.deleteTask(taskID)
		return false, "审批上下文已取消"
	}
}

func (m *ApprovalManager) deleteTask(taskID string) {
	m.mu.Lock()
	delete(m.pendingTasks, taskID)
	m.mu.Unlock()
}

// ResolveApproval 投递审批结果；返回是否成功投递（任务不存在/已处理 → false）。
func (m *ApprovalManager) ResolveApproval(ctx context.Context, taskID string, allowed bool, reason string) bool {
	m.mu.RLock()
	task, ok := m.pendingTasks[taskID]
	m.mu.RUnlock()
	if !ok {
		slog.WarnContext(ctx, "resolveApproval: task not exist, maybe timeout or already handled",
			slog.String("taskId", taskID))
		return false
	}
	slog.InfoContext(ctx, "resolveApproval",
		slog.String("taskId", taskID), slog.Bool("allowed", allowed), slog.String("reason", reason))
	select {
	case task.ch <- ApprovalResult{Allowed: allowed, RejectReason: reason}:
		return true
	default:
		return false // 缓冲已满（已被并发 resolve），不阻塞
	}
}

// GetTask 供卡片回调处理读取任务元数据（身份校验 + 结果卡展示）。
func (m *ApprovalManager) GetTask(taskID string) (Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.pendingTasks[taskID]
	return t, ok
}

// ParseApprovalTimeout 解析审批超时配置；非法值/<=0 回退默认 5 分钟并告警。
func ParseApprovalTimeout(raw string) time.Duration {
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		slog.Warn("approvalTimeout 非法，回退默认 5m", slog.String("raw", raw))
		return 5 * time.Minute
	}
	return d
}
