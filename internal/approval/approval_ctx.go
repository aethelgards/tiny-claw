package approval

import (
	"context"

	"github.com/aethelgards/tiny-claw/internal/reporter"
)

type approvalCtxKey struct{}

type approvalContext struct {
	reporter   reporter.Reporter
	approverID string
}

// WithApprovalContext 把审批所需上下文（reporter + 发起人 open_id）注入 ctx。
func WithApprovalContext(ctx context.Context, reporter reporter.Reporter, approverID string) context.Context {
	return context.WithValue(ctx, approvalCtxKey{}, approvalContext{
		reporter:   reporter,
		approverID: approverID,
	})
}

func approvalContextFrom(ctx context.Context) (approvalContext, bool) {
	ac, ok := ctx.Value(approvalCtxKey{}).(approvalContext)
	return ac, ok
}
