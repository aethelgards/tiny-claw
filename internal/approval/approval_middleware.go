package approval

import (
	"context"
	"errors"

	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/aethelgards/tiny-claw/internal/tools"
)

// ApprovalMiddleware 危险命令审批中间件：命中高危模式必须经发起人卡片审批。
// 非危险命令零开销放行；危险命令缺审批上下文 → 拒绝（fail-closed 兜底）。
func ApprovalMiddleware(mgr *ApprovalManager) tools.MiddlewareFunc {
	return func(ctx context.Context, call schema.ToolCall) (bool, error) {
		if !isDangerousCommand(call.Name, string(call.Arguments)) {
			return true, nil
		}
		ac, ok := approvalContextFrom(ctx)
		if !ok {
			return false, errors.New("缺少审批上下文，无法执行高危操作")
		}
		allowed, reason := mgr.WaitingForApproval(ctx, call.ID, call.Name, string(call.Arguments), ac.reporter, ac.approverID)
		if !allowed {
			return false, errors.New(reason)
		}
		return true, nil
	}
}
