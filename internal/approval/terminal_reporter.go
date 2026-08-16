package approval

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aethelgards/tiny-claw/internal/engine"
	"github.com/aethelgards/tiny-claw/internal/reporter"
)

// TerminalReporter 终端版审批 reporter：包装 engine.TerminalReporter，
// SendApprovalMessage 改为交互式 y/N 提示 + 直接 ResolveApproval。
type TerminalReporter struct {
	base        reporter.Reporter // 转发 OnThinking/OnToolCall/OnToolResult/OnMessage
	mgr         *ApprovalManager
	reader      *bufio.Reader // 持久 reader：构造时创建一次，避免过度缓冲吞掉后续输入
	interactive bool          // 构造时检测 stdin 是否为字符设备
}

// NewTerminalReporter 创建终端审批 reporter；stdin 非字符设备（CI/管道）时审批直接拒绝（fail-closed）。
func NewTerminalReporter(mgr *ApprovalManager) reporter.Reporter {
	fi, _ := os.Stdin.Stat()
	interactive := fi != nil && fi.Mode()&os.ModeCharDevice != 0
	return &TerminalReporter{
		base:        engine.NewTerminalReporter(),
		mgr:         mgr,
		reader:      bufio.NewReader(os.Stdin),
		interactive: interactive,
	}
}

// newTerminalReporterForTest 供测试注入 reader 与 interactive 标志。
func newTerminalReporterForTest(mgr *ApprovalManager, in io.Reader, interactive bool) reporter.Reporter {
	return &TerminalReporter{
		base:        engine.NewTerminalReporter(),
		mgr:         mgr,
		reader:      bufio.NewReader(in),
		interactive: interactive,
	}
}

// 编译期断言：实现 engine.Reporter。
var _ reporter.Reporter = (*TerminalReporter)(nil)

func (t *TerminalReporter) OnThinking(ctx context.Context) {
	t.base.OnThinking(ctx)
}

func (t *TerminalReporter) OnToolCall(ctx context.Context, toolName string, args string) {
	t.base.OnToolCall(ctx, toolName, args)
}

func (t *TerminalReporter) OnToolResult(ctx context.Context, toolName string, result string, isError bool) {
	t.base.OnToolResult(ctx, toolName, result, isError)
}

func (t *TerminalReporter) OnMessage(ctx context.Context, content string) {
	t.base.OnMessage(ctx, content)
}

// SendApprovalMessage 打印审批请求并交互式读取一行 y/N：
//   - y/Y/yes → 放行
//   - 其余（含 n、空行）→ 拒绝（"终端用户拒绝"）
//   - 读失败/EOF → fail-closed 拒绝（"终端输入读取失败"）
//   - 非交互 stdin → 立即拒绝不阻塞（"非交互终端，自动拒绝"）
//   - 超时/ctx 取消 → 拒绝（文案与 WaitingForApproval 一致）
//
// 读操作放 goroutine + select 兜底：用户不输入时由 mgr.timeout / ctx 取消解阻塞（不挂死）。
// 超时后遗留的阻塞读 goroutine 属可接受（任务已被清理，后续 ResolveApproval 返回 false 无副作用）。
func (t *TerminalReporter) SendApprovalMessage(ctx context.Context, taskID, toolName, args string) error {
	fmt.Printf("⚠️ 高危操作审批请求\n工具: %s\n参数: %s\n任务 ID: %s\n允许执行? (y/N): ", toolName, args, taskID)
	if !t.interactive {
		t.mgr.ResolveApproval(ctx, taskID, false, "非交互终端，自动拒绝")
		return nil
	}

	lineCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		line, err := t.reader.ReadString('\n')
		if err != nil {
			errCh <- err
			return
		}
		lineCh <- line
	}()

	select {
	case line := <-lineCh:
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			t.mgr.ResolveApproval(ctx, taskID, true, "")
		default:
			t.mgr.ResolveApproval(ctx, taskID, false, "终端用户拒绝")
		}
	case <-errCh:
		t.mgr.ResolveApproval(ctx, taskID, false, "终端输入读取失败")
	case <-time.After(t.mgr.timeout):
		t.mgr.ResolveApproval(ctx, taskID, false, "审批超时，已自动拒绝")
	case <-ctx.Done():
		t.mgr.ResolveApproval(ctx, taskID, false, "审批上下文已取消")
	}
	return nil
}
