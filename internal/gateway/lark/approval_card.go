package lark

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aethelgards/tiny-claw/internal/approval"
)

const maxArgsLen = 512

// BuildApprovalCard 构建审批请求卡（card JSON v2，header red）。
func BuildApprovalCard(taskID, toolName, args string) string {
	card := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"template": "red",
			"title":    map[string]any{"tag": "plain_text", "content": "⚠️ 高危操作审批请求"},
		},
		"body": map[string]any{
			"elements": []any{
				map[string]any{
					"tag": "markdown",
					"content": fmt.Sprintf("**Agent 请求执行以下操作：**\n**工具：** `%s`\n**参数：**\n```\n%s\n```\n**任务 ID：** `%s`",
						escapeMarkdown(toolName), escapeMarkdown(truncateArgs(args)), taskID),
				},
				map[string]any{"tag": "hr"},
				map[string]any{
					"tag":  "form",
					"name": "approval_form",
					"elements": []any{
						map[string]any{
							"tag":         "input",
							"name":        "reject_reason",
							"required":    false,
							"label":       map[string]any{"tag": "plain_text", "content": "拒绝原因（选填）"},
							"placeholder": map[string]any{"tag": "plain_text", "content": "仅拒绝时填写"},
						},
						buildSubmitButton("approve_btn", "primary", "✅ 通过", "approve", taskID),
						buildSubmitButton("reject_btn", "danger", "❌ 拒绝", "reject", taskID),
					},
				},
			},
		},
	}
	return mustJSON(card)
}

// buildSubmitButton 构建 form 内 submit 按钮：name + form_action_type + behaviors callback。
func buildSubmitButton(name, btnType, text, action, taskID string) map[string]any {
	return map[string]any{
		"tag":              "button",
		"name":             name,
		"form_action_type": "submit",
		"type":             btnType,
		"text":             map[string]any{"tag": "plain_text", "content": text},
		"behaviors": []any{map[string]any{
			"type":  "callback",
			"value": map[string]any{"action": action, "task_id": taskID},
		}},
	}
}

// BuildApprovalResultCard 构建结果卡：允许 → green 通过；拒绝 → red 已拒绝（追加原因，未填省略）。
func BuildApprovalResultCard(task approval.Task, allowed bool, reason, operatorOpenID string) string {
	template, title := "green", "✅ 已通过"
	if !allowed {
		template, title = "red", "❌ 已拒绝"
	}
	lines := []string{
		fmt.Sprintf("**审批人：** `%s`", escapeMarkdown(operatorOpenID)),
		fmt.Sprintf("**任务 ID：** `%s`", escapeMarkdown(task.TaskID)),
		fmt.Sprintf("**工具：** `%s`", escapeMarkdown(task.ToolName)),
		fmt.Sprintf("**参数：**\n```\n%s\n```", escapeMarkdown(truncateArgs(task.Args))),
	}
	if !allowed && reason != "" {
		lines = append(lines, fmt.Sprintf("**拒绝原因：** %s", escapeMarkdown(reason)))
	}
	card := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"template": template,
			"title":    map[string]any{"tag": "plain_text", "content": title},
		},
		"body": map[string]any{
			"elements": []any{
				map[string]any{"tag": "markdown", "content": strings.Join(lines, "\n")},
			},
		},
	}
	return mustJSON(card)
}

// escapeMarkdown 反引号后接零宽空格：视觉不变，但破坏 ``` 围栏序列，防嵌入内容提前闭合代码块。
func escapeMarkdown(s string) string {
	return strings.ReplaceAll(s, "`", "`\u200b")
}

// truncateArgs 超长参数截断加省略号，避免卡片超限（消息 30KB 上限）。
func truncateArgs(args string) string {
	if len(args) <= maxArgsLen {
		return args
	}
	return args[:maxArgsLen] + "…"
}

// mustJSON 序列化卡片；结构固定，marshal 理论上不可能失败。失败返回空串（调用方 fail-closed）。
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
