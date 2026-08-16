package lark

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aethelgards/tiny-claw/internal/approval"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

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

func buildActionEvent(openID, taskID, name, action string, formValue map[string]any) *callback.CardActionTriggerEvent {
	return &callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Operator: &callback.Operator{OpenID: openID},
			Action: &callback.CallBackAction{
				Name:      name,
				Value:     map[string]any{"action": action, "task_id": taskID},
				FormValue: formValue,
			},
		},
	}
}

func TestHandlerApprove(t *testing.T) {
	mgr := approval.NewApprovalManager(time.Minute)
	rep := &fakeApprovalReporter{}
	handler := NewApprovalCardHandler(mgr)
	ctx := context.Background()

	type waitResult struct {
		allowed bool
		reason  string
	}
	got := make(chan waitResult, 1)
	go func() {
		allowed, reason := mgr.WaitingForApproval(ctx, "task-1", "bash", "rm -rf /", rep, "ou_approver")
		got <- waitResult{allowed, reason}
	}()
	waitFor(t, time.Second, func() bool {
		_, ok := mgr.GetTask("task-1")
		return ok
	})

	resp, err := handler(ctx, buildActionEvent("ou_approver", "task-1", "approve_btn", "approve", nil))
	if err != nil {
		t.Fatalf("handler 不应返回 error: %v", err)
	}
	if resp.Toast == nil || resp.Toast.Type != "success" || resp.Toast.Content != "已通过" {
		t.Fatalf("Toast 应为 success/已通过，得到 %+v", resp.Toast)
	}
	if resp.Card == nil || resp.Card.Type != "raw" {
		t.Fatalf("Card 应为 raw 类型，得到 %+v", resp.Card)
	}
	data, ok := resp.Card.Data.(string)
	if !ok || !strings.Contains(data, "✅ 已通过") {
		t.Fatalf("结果卡应含 ✅ 已通过，得到 %v", resp.Card.Data)
	}
	r := <-got
	if !r.allowed || r.reason != "" {
		t.Fatalf("WaitingForApproval 应返回 (true, \"\")，得到 (%v, %q)", r.allowed, r.reason)
	}
	if _, ok := mgr.GetTask("task-1"); ok {
		t.Fatal("任务应已清理")
	}
}

func TestHandlerRejectWithReason(t *testing.T) {
	mgr := approval.NewApprovalManager(time.Minute)
	rep := &fakeApprovalReporter{}
	handler := NewApprovalCardHandler(mgr)
	ctx := context.Background()

	got := make(chan struct {
		allowed bool
		reason  string
	}, 1)
	go func() {
		allowed, reason := mgr.WaitingForApproval(ctx, "task-1", "bash", "rm -rf /", rep, "ou_approver")
		got <- struct {
			allowed bool
			reason  string
		}{allowed, reason}
	}()
	waitFor(t, time.Second, func() bool {
		_, ok := mgr.GetTask("task-1")
		return ok
	})

	resp, err := handler(ctx, buildActionEvent("ou_approver", "task-1", "reject_btn", "reject", map[string]any{"reject_reason": "不想放"}))
	if err != nil {
		t.Fatalf("handler 不应返回 error: %v", err)
	}
	if resp.Toast == nil || resp.Toast.Content != "已拒绝" {
		t.Fatalf("Toast 应为 已拒绝，得到 %+v", resp.Toast)
	}
	data := resp.Card.Data.(string)
	if !strings.Contains(data, "❌ 已拒绝") || !strings.Contains(data, "不想放") {
		t.Fatalf("结果卡应含 ❌ 已拒绝 与拒绝原因，得到 %s", data)
	}
	r := <-got
	if r.allowed || r.reason != "不想放" {
		t.Fatalf("WaitingForApproval 应返回 (false, 不想放)，得到 (%v, %q)", r.allowed, r.reason)
	}
}

func TestHandlerRejectNoReason(t *testing.T) {
	mgr := approval.NewApprovalManager(time.Minute)
	rep := &fakeApprovalReporter{}
	handler := NewApprovalCardHandler(mgr)
	ctx := context.Background()

	got := make(chan struct {
		allowed bool
		reason  string
	}, 1)
	go func() {
		allowed, reason := mgr.WaitingForApproval(ctx, "task-1", "bash", "rm -rf /", rep, "ou_approver")
		got <- struct {
			allowed bool
			reason  string
		}{allowed, reason}
	}()
	waitFor(t, time.Second, func() bool {
		_, ok := mgr.GetTask("task-1")
		return ok
	})

	resp, err := handler(ctx, buildActionEvent("ou_approver", "task-1", "reject_btn", "reject", nil))
	if err != nil {
		t.Fatalf("handler 不应返回 error: %v", err)
	}
	if data := resp.Card.Data.(string); strings.Contains(data, "拒绝原因") {
		t.Fatalf("未填拒绝原因时结果卡不应含 拒绝原因: %s", data)
	}
	r := <-got
	if r.allowed || r.reason != "" {
		t.Fatalf("WaitingForApproval 应返回 (false, \"\")，得到 (%v, %q)", r.allowed, r.reason)
	}
}

func TestHandlerNotApprover(t *testing.T) {
	mgr := approval.NewApprovalManager(time.Minute)
	rep := &fakeApprovalReporter{}
	handler := NewApprovalCardHandler(mgr)
	ctx := context.Background()

	released := make(chan struct{}, 1)
	go func() {
		mgr.WaitingForApproval(ctx, "task-1", "bash", "rm -rf /", rep, "ou_approver")
		released <- struct{}{}
	}()
	waitFor(t, time.Second, func() bool {
		_, ok := mgr.GetTask("task-1")
		return ok
	})

	resp, err := handler(ctx, buildActionEvent("ou_other", "task-1", "approve_btn", "approve", nil))
	if err != nil {
		t.Fatalf("handler 不应返回 error: %v", err)
	}
	if resp.Toast == nil || resp.Toast.Type != "error" || !strings.Contains(resp.Toast.Content, "只有发起请求的用户才能审批") {
		t.Fatalf("Toast 应为 error 且含身份校验文案，得到 %+v", resp.Toast)
	}
	if resp.Card != nil {
		t.Fatal("身份校验失败时不应返回结果卡")
	}
	if !mgr.ResolveApproval(ctx, "task-1", false, "非发起人") {
		t.Fatal("任务仍在等待中，ResolveApproval 应返回 true")
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("WaitingForApproval 未被释放")
	}
}

func TestHandlerTaskNotFound(t *testing.T) {
	mgr := approval.NewApprovalManager(time.Minute)
	handler := NewApprovalCardHandler(mgr)
	resp, err := handler(context.Background(), buildActionEvent("ou_approver", "nope", "approve_btn", "approve", nil))
	if err != nil {
		t.Fatalf("handler 不应返回 error: %v", err)
	}
	if resp.Toast == nil || resp.Toast.Type != "warning" || !strings.Contains(resp.Toast.Content, "该请求已处理或不存在") {
		t.Fatalf("Toast 应为 warning 且含任务不存在文案，得到 %+v", resp.Toast)
	}
	if resp.Card != nil {
		t.Fatal("任务不存在时不应返回结果卡")
	}
}

func TestHandlerActionMismatch(t *testing.T) {
	mgr := approval.NewApprovalManager(time.Minute)
	handler := NewApprovalCardHandler(mgr)
	resp, err := handler(context.Background(), buildActionEvent("ou_approver", "task-1", "approve_btn", "reject", nil))
	if err != nil {
		t.Fatalf("handler 不应返回 error: %v", err)
	}
	if resp.Toast == nil || resp.Toast.Type != "error" || !strings.Contains(resp.Toast.Content, "未知操作") {
		t.Fatalf("Toast 应为 error 且含未知操作文案，得到 %+v", resp.Toast)
	}
	if resp.Card != nil {
		t.Fatal("操作不匹配时不应返回结果卡")
	}
}
