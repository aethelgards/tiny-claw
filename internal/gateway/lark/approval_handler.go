package lark

import (
	"context"

	"github.com/aethelgards/tiny-claw/internal/approval"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

// NewApprovalCardHandler 处理卡片审批回调（card.action.trigger）：
// 校验任务存在与发起人身份 → ResolveApproval 投递 → 原位更新结果卡（原子更新）。
func NewApprovalCardHandler(mgr *approval.ApprovalManager) func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	return func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
		req := event.Event
		if event == nil || req == nil || req.Action == nil || req.Operator == nil {
			return errorToast("回调数据缺失"), nil
		}

		// 按钮身份：Action.Name 为主，Value["action"] 交叉校验
		actionName, _ := req.Action.Value["action"].(string)
		var approve bool
		switch req.Action.Name {
		case "approve_btn":
			if actionName != "" && actionName != "approve" {
				return errorToast("未知操作"), nil
			}
			approve = true
		case "reject_btn":
			if actionName != "" && actionName != "reject" {
				return errorToast("未知操作"), nil
			}
			approve = false
		default:
			return errorToast("未知操作"), nil
		}

		taskID, _ := req.Action.Value["task_id"].(string)
		if taskID == "" {
			return errorToast("任务 ID 缺失"), nil
		}

		// 任务存在性
		task, ok := mgr.GetTask(taskID)
		if !ok {
			return warningToast("该请求已处理或不存在"), nil
		}

		// 身份校验：只有发起请求的用户能审批（open_id 为空时任何非空 operator 都过不了 → 超时拒绝，fail-safe）
		if req.Operator.OpenID != task.ApproverID {
			return errorToast("只有发起请求的用户才能审批"), nil
		}

		// 拒绝原因（可选；approve 恒空）
		reason := ""
		if !approve {
			if v, ok := req.Action.FormValue["reject_reason"].(string); ok {
				reason = v
			}
		}

		// 投递（竞态/已处理 → false → warning）
		if !mgr.ResolveApproval(ctx, taskID, approve, reason) {
			return warningToast("该请求已处理或不存在"), nil
		}

		toastContent := "已通过"
		if !approve {
			toastContent = "已拒绝"
		}
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{Type: "success", Content: toastContent},
			Card: &callback.Card{
				Type: "raw",
				Data: BuildApprovalResultCard(task, approve, reason, req.Operator.OpenID),
			},
		}, nil
	}
}

func errorToast(content string) *callback.CardActionTriggerResponse {
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{Type: "error", Content: content},
	}
}

func warningToast(content string) *callback.CardActionTriggerResponse {
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{Type: "warning", Content: content},
	}
}
