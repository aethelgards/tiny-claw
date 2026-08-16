package approval

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/aethelgards/tiny-claw/internal/tools"
)

type fakeBashTool struct{}

func (fakeBashTool) Name() string { return "bash" }
func (fakeBashTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{Name: "bash", Description: "fake bash"}
}
func (fakeBashTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return "ok", nil
}

func TestMiddlewarePassNonDangerous(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	mw := ApprovalMiddleware(mgr)
	allowed, err := mw(context.Background(), schema.ToolCall{ID: "call-1", Name: "read_file", Arguments: json.RawMessage(`{"path":"a.txt"}`)})
	if err != nil || !allowed {
		t.Fatalf("非危险命令应直接放行，得到 (%v, %v)", allowed, err)
	}
}

func TestMiddlewareDenyNoCtx(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	mw := ApprovalMiddleware(mgr)
	allowed, err := mw(context.Background(), schema.ToolCall{ID: "call-1", Name: "bash", Arguments: json.RawMessage(`{"command":"rm -rf /"}`)})
	if allowed {
		t.Fatal("缺审批上下文时危险命令应拒绝")
	}
	if err == nil || !strings.Contains(err.Error(), "缺少审批上下文") {
		t.Fatalf("拒绝原因应含 缺少审批上下文，得到 %v", err)
	}
}

func TestMiddlewareApprove(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := &fakeApprovalReporter{}
	ctx := WithApprovalContext(context.Background(), rep, "ou_approver")
	mw := ApprovalMiddleware(mgr)

	type result struct {
		allowed bool
		err     error
	}
	got := make(chan result, 1)
	go func() {
		allowed, err := mw(ctx, schema.ToolCall{ID: "call-1", Name: "bash", Arguments: json.RawMessage(`{"command":"rm -rf /"}`)})
		got <- result{allowed, err}
	}()

	waitFor(t, time.Second, func() bool {
		_, ok := mgr.GetTask("call-1")
		return ok
	})
	if !mgr.ResolveApproval(ctx, "call-1", true, "") {
		t.Fatal("ResolveApproval 应返回 true")
	}
	r := <-got
	if !r.allowed || r.err != nil {
		t.Fatalf("审批通过后应 (true, nil)，得到 (%v, %v)", r.allowed, r.err)
	}
	if _, ok := mgr.GetTask("call-1"); ok {
		t.Fatal("任务应已清理")
	}
}

func TestMiddlewareReject(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := &fakeApprovalReporter{}
	ctx := WithApprovalContext(context.Background(), rep, "ou_approver")
	mw := ApprovalMiddleware(mgr)

	got := make(chan error, 1)
	go func() {
		_, err := mw(ctx, schema.ToolCall{ID: "call-1", Name: "bash", Arguments: json.RawMessage(`{"command":"rm -rf /"}`)})
		got <- err
	}()

	waitFor(t, time.Second, func() bool {
		_, ok := mgr.GetTask("call-1")
		return ok
	})
	if !mgr.ResolveApproval(ctx, "call-1", false, "原因Y") {
		t.Fatal("ResolveApproval 应返回 true")
	}
	err := <-got
	if err == nil || !strings.Contains(err.Error(), "原因Y") {
		t.Fatalf("拒绝原因应含 原因Y，得到 %v", err)
	}
}

func TestRegistryIntegrationIntercepted(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := &fakeApprovalReporter{}
	ctx := WithApprovalContext(context.Background(), rep, "ou_approver")
	reg := tools.NewToolRegistry()
	if err := reg.Registry(fakeBashTool{}); err != nil {
		t.Fatalf("注册工具失败: %v", err)
	}
	reg.Use(ApprovalMiddleware(mgr))

	got := make(chan schema.ToolResult, 1)
	go func() {
		got <- reg.Execute(ctx, schema.ToolCall{ID: "call-r1", Name: "bash", Arguments: json.RawMessage(`{"command":"rm -rf /tmp"}`)})
	}()

	waitFor(t, time.Second, func() bool {
		_, ok := mgr.GetTask("call-r1")
		return ok
	})
	if !mgr.ResolveApproval(ctx, "call-r1", false, "") {
		t.Fatal("ResolveApproval 应返回 true")
	}
	result := <-got
	if !result.IsError {
		t.Fatal("中间件拒绝后结果应标记为错误")
	}
	if !strings.Contains(result.Output, "执行被系统拦截") {
		t.Fatalf("registry 拦截文案应含 执行被系统拦截，得到 %q", result.Output)
	}
}
