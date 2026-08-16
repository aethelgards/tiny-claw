package approval

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// waitFor 轮询等待条件成立，超时则失败；供审批测试等待任务注册/清理。
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("等待条件超时")
}

// fakeApprovalReporter 记录发送的审批通知，可按配置触发发送失败。
type fakeApprovalReporter struct {
	sendErr error
	calls   int
	last    struct {
		taskID   string
		toolName string
		args     string
	}
}

func (f *fakeApprovalReporter) OnThinking(ctx context.Context)                     {}
func (f *fakeApprovalReporter) OnToolCall(ctx context.Context, n string, a string) {}
func (f *fakeApprovalReporter) OnToolResult(ctx context.Context, n string, r string, isErr bool) {
}
func (f *fakeApprovalReporter) OnMessage(ctx context.Context, content string) {}

func (f *fakeApprovalReporter) SendApprovalMessage(ctx context.Context, taskID string, toolName string, args string) error {
	f.calls++
	f.last.taskID = taskID
	f.last.toolName = toolName
	f.last.args = args
	return f.sendErr
}

func TestApprovalResolve(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := &fakeApprovalReporter{}
	ctx := context.Background()

	type result struct {
		allowed bool
		reason  string
	}
	got := make(chan result, 1)
	go func() {
		allowed, reason := mgr.WaitingForApproval(ctx, "call-1", "bash", "rm -rf /", rep, "ou_approver")
		got <- result{allowed, reason}
	}()

	waitFor(t, time.Second, func() bool {
		_, ok := mgr.GetTask("call-1")
		return ok
	})
	if !mgr.ResolveApproval(ctx, "call-1", true, "") {
		t.Fatal("ResolveApproval 应返回 true")
	}
	select {
	case r := <-got:
		if !r.allowed || r.reason != "" {
			t.Fatalf("期望 (true, \"\")，得到 (%v, %q)", r.allowed, r.reason)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitingForApproval 未解除阻塞")
	}
	if _, ok := mgr.GetTask("call-1"); ok {
		t.Fatal("任务应已清理")
	}
	if rep.calls != 1 {
		t.Fatalf("SendApprovalMessage 应调用 1 次，得到 %d", rep.calls)
	}
	if rep.last.taskID != "call-1" || rep.last.toolName != "bash" || rep.last.args != "rm -rf /" {
		t.Fatalf("审批通知内容不符: %+v", rep.last)
	}
}

func TestApprovalResolveRejectReason(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := &fakeApprovalReporter{}
	ctx := context.Background()

	got := make(chan string, 1)
	go func() {
		_, reason := mgr.WaitingForApproval(ctx, "call-1", "bash", "rm -rf /", rep, "ou_approver")
		got <- reason
	}()

	waitFor(t, time.Second, func() bool {
		_, ok := mgr.GetTask("call-1")
		return ok
	})
	if !mgr.ResolveApproval(ctx, "call-1", false, "风险过高") {
		t.Fatal("ResolveApproval 应返回 true")
	}
	select {
	case r := <-got:
		if r != "风险过高" {
			t.Fatalf("期望拒绝原因 %q，得到 %q", "风险过高", r)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitingForApproval 未解除阻塞")
	}
}

func TestApprovalTimeoutDeny(t *testing.T) {
	mgr := NewApprovalManager(50 * time.Millisecond)
	rep := &fakeApprovalReporter{}
	allowed, reason := mgr.WaitingForApproval(context.Background(), "call-1", "bash", "rm -rf /", rep, "ou_approver")
	if allowed {
		t.Fatal("超时应拒绝")
	}
	if !strings.Contains(reason, "审批超时") {
		t.Fatalf("期望超时文案，得到 %q", reason)
	}
	if _, ok := mgr.GetTask("call-1"); ok {
		t.Fatal("任务应已清理")
	}
}

func TestApprovalCtxCancelDeny(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := &fakeApprovalReporter{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	allowed, reason := mgr.WaitingForApproval(ctx, "call-1", "bash", "rm -rf /", rep, "ou_approver")
	if allowed {
		t.Fatal("ctx 取消应拒绝")
	}
	if !strings.Contains(reason, "审批上下文已取消") {
		t.Fatalf("期望取消文案，得到 %q", reason)
	}
	if _, ok := mgr.GetTask("call-1"); ok {
		t.Fatal("任务应已清理")
	}
}

func TestApprovalSendFailClosed(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := &fakeApprovalReporter{sendErr: errors.New("send failed")}
	allowed, reason := mgr.WaitingForApproval(context.Background(), "call-1", "bash", "rm -rf /", rep, "ou_approver")
	if allowed {
		t.Fatal("发送失败应 fail-closed 拒绝")
	}
	if reason != "审批通知发送失败，已拒绝执行" {
		t.Fatalf("期望 fail-closed 文案，得到 %q", reason)
	}
	if _, ok := mgr.GetTask("call-1"); ok {
		t.Fatal("任务应已清理")
	}
	if rep.calls != 1 {
		t.Fatalf("SendApprovalMessage 应调用 1 次，得到 %d", rep.calls)
	}
}

func TestApprovalNilReporterFailClosed(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	allowed, reason := mgr.WaitingForApproval(context.Background(), "call-1", "bash", "rm -rf /", nil, "ou_approver")
	if allowed {
		t.Fatal("nil reporter 应 fail-closed 拒绝")
	}
	if reason != "审批通知发送失败，已拒绝执行" {
		t.Fatalf("期望 fail-closed 文案，得到 %q", reason)
	}
	if _, ok := mgr.GetTask("call-1"); ok {
		t.Fatal("任务应已清理")
	}
}

func TestResolveUnknownTask(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	if mgr.ResolveApproval(context.Background(), "nope", true, "") {
		t.Fatal("未知任务 ResolveApproval 应返回 false")
	}
}

func TestResolveAfterTimeout(t *testing.T) {
	mgr := NewApprovalManager(50 * time.Millisecond)
	rep := &fakeApprovalReporter{}
	allowed, _ := mgr.WaitingForApproval(context.Background(), "call-1", "bash", "rm -rf /", rep, "ou_approver")
	if allowed {
		t.Fatal("超时应拒绝")
	}
	if mgr.ResolveApproval(context.Background(), "call-1", true, "") {
		t.Fatal("超时后任务已清理，ResolveApproval 应返回 false")
	}
}

type blockingReporter struct {
	fakeApprovalReporter
	release chan struct{}
}

func (b *blockingReporter) SendApprovalMessage(ctx context.Context, taskID string, toolName string, args string) error {
	<-b.release
	return nil
}

func TestResolveTwiceSecondFalse(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := &fakeApprovalReporter{}
	ctx := context.Background()
	done := make(chan bool, 1)
	go func() {
		allowed, _ := mgr.WaitingForApproval(ctx, "call-1", "bash", "rm -rf /", rep, "ou_approver")
		done <- allowed
	}()
	waitFor(t, time.Second, func() bool {
		_, ok := mgr.GetTask("call-1")
		return ok
	})
	if !mgr.ResolveApproval(ctx, "call-1", true, "") {
		t.Fatal("第一次投递应返回 true")
	}
	if !<-done {
		t.Fatal("等待结果应为 true")
	}
	if mgr.ResolveApproval(ctx, "call-1", true, "") {
		t.Fatal("任务已处理完成，第二次投递应返回 false")
	}
}

func TestResolveTwiceWhilePendingSecondFalse(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := &blockingReporter{release: make(chan struct{})}
	ctx := context.Background()
	done := make(chan bool, 1)
	go func() {
		allowed, _ := mgr.WaitingForApproval(ctx, "call-1", "bash", "rm -rf /", rep, "ou_approver")
		done <- allowed
	}()
	waitFor(t, time.Second, func() bool {
		_, ok := mgr.GetTask("call-1")
		return ok
	})
	if !mgr.ResolveApproval(ctx, "call-1", true, "") {
		t.Fatal("第一次投递应返回 true")
	}
	if mgr.ResolveApproval(ctx, "call-1", true, "") {
		t.Fatal("任务仍在等待中且通道已满，第二次投递应返回 false")
	}
	close(rep.release)
	if !<-done {
		t.Fatal("等待结果应为 true")
	}
}

func TestParseApprovalTimeout(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Duration
	}{
		{"5m", 5 * time.Minute},
		{"abc", 5 * time.Minute},
		{"0s", 5 * time.Minute},
		{"-1s", 5 * time.Minute},
	}
	for _, c := range cases {
		if got := ParseApprovalTimeout(c.raw); got != c.want {
			t.Errorf("ParseApprovalTimeout(%q) = %v, want %v", c.raw, got, c.want)
		}
	}
}

func TestIsDangerousCommand(t *testing.T) {
	cases := []struct {
		name     string
		toolName string
		args     string
		want     bool
	}{
		{"bash rm -rf", "bash", "rm -rf /", true},
		{"bash sudo", "bash", "sudo apt update", true},
		{"bash drop", "bash", "drop table users", true},
		{"bash overwrite go", "bash", "cat > x.go", true},
		{"bash benign", "bash", "ls -la", false},
		{"write_file rm", "write_file", "rm -r", false},
		{"edit_file anything", "edit_file", "anything", false},
		{"read_file anything", "read_file", "anything", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isDangerousCommand(c.toolName, c.args); got != c.want {
				t.Errorf("isDangerousCommand(%q, %q) = %v, want %v", c.toolName, c.args, got, c.want)
			}
		})
	}
}
