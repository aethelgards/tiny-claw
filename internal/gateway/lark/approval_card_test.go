package lark

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aethelgards/tiny-claw/internal/approval"
)

func TestBuildApprovalCardStructure(t *testing.T) {
	cardJSON := BuildApprovalCard("task-t-1", "bash", "rm -rf /")
	var root map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &root); err != nil {
		t.Fatalf("卡片 JSON 解析失败: %v", err)
	}
	if root["schema"] != "2.0" {
		t.Fatalf("schema = %v, want 2.0", root["schema"])
	}
	header := root["header"].(map[string]any)
	if header["template"] != "red" {
		t.Fatalf("header.template = %v, want red", header["template"])
	}
	body := root["body"].(map[string]any)
	elements := body["elements"].([]any)
	if len(elements) != 3 {
		t.Fatalf("elements 数量 = %d, want 3", len(elements))
	}
	md := elements[0].(map[string]any)
	if md["tag"] != "markdown" {
		t.Fatalf("elements[0].tag = %v, want markdown", md["tag"])
	}
	content := md["content"].(string)
	for _, want := range []string{"task-t-1", "bash", "rm -rf /"} {
		if !strings.Contains(content, want) {
			t.Fatalf("markdown content 缺少 %q: %s", want, content)
		}
	}
	if elements[1].(map[string]any)["tag"] != "hr" {
		t.Fatal("elements[1].tag 应为 hr")
	}
	form := elements[2].(map[string]any)
	if form["tag"] != "form" || form["name"] != "approval_form" {
		t.Fatalf("elements[2] 应为 form approval_form，得到 tag=%v name=%v", form["tag"], form["name"])
	}
	formElements := form["elements"].([]any)
	if len(formElements) != 3 {
		t.Fatalf("form elements 数量 = %d, want 3", len(formElements))
	}
	input := formElements[0].(map[string]any)
	if input["tag"] != "input" || input["name"] != "reject_reason" || input["required"] != false {
		t.Fatalf("form elements[0] 应为 input reject_reason(非必填)，得到 %v", input)
	}
	buttons := []struct {
		name   string
		action string
	}{{"approve_btn", "approve"}, {"reject_btn", "reject"}}
	for i, want := range buttons {
		b := formElements[i+1].(map[string]any)
		if b["tag"] != "button" || b["name"] != want.name || b["form_action_type"] != "submit" {
			t.Fatalf("button %d 应为 name=%s form_action_type=submit，得到 %v", i, want.name, b)
		}
		behaviors := b["behaviors"].([]any)
		value := behaviors[0].(map[string]any)["value"].(map[string]any)
		if value["action"] != want.action || value["task_id"] != "task-t-1" {
			t.Fatalf("button %s 的 callback value = %v", want.name, value)
		}
	}
}

func TestBuildApprovalCardNoActionModule(t *testing.T) {
	cardJSON := BuildApprovalCard("task-t-1", "bash", "rm -rf /")
	if strings.Contains(cardJSON, `"tag":"action"`) {
		t.Fatal("卡片不应包含 v2 不支持的 action 模块")
	}
}

func TestBuildApprovalCardTruncate(t *testing.T) {
	long := strings.Repeat("a", 600)
	if !strings.Contains(BuildApprovalCard("task-t-1", "bash", long), strings.Repeat("a", maxArgsLen)+"…") {
		t.Fatal("超长参数应截断到 maxArgsLen 并加省略号")
	}
	short := "ls -la"
	if !strings.Contains(BuildApprovalCard("task-t-1", "bash", short), short) {
		t.Fatal("短参数应原样包含")
	}
	if strings.Contains(BuildApprovalCard("task-t-1", "bash", short), "…") {
		t.Fatal("短参数不应截断")
	}
}

func TestBuildApprovalCardEscape(t *testing.T) {
	cardJSON := BuildApprovalCard("task-t-1", "bash", "```dangerous```")
	var root map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &root); err != nil {
		t.Fatalf("卡片 JSON 解析失败: %v", err)
	}
	content := root["body"].(map[string]any)["elements"].([]any)[0].(map[string]any)["content"].(string)
	if n := strings.Count(content, "```"); n != 2 {
		t.Fatalf("markdown content 应只含 2 处围栏，实际 %d 处: %s", n, content)
	}
}

func TestBuildApprovalResultCard(t *testing.T) {
	task := approval.Task{
		TaskID:   "task-t-1",
		ToolName: "bash",
		Args:     "rm -rf /",
	}

	var root map[string]any
	allowedJSON := BuildApprovalResultCard(task, true, "", "ou_approver")
	if err := json.Unmarshal([]byte(allowedJSON), &root); err != nil {
		t.Fatalf("结果卡 JSON 解析失败: %v", err)
	}
	header := root["header"].(map[string]any)
	if header["template"] != "green" {
		t.Fatalf("通过卡 template = %v, want green", header["template"])
	}
	if !strings.Contains(header["title"].(map[string]any)["content"].(string), "✅ 已通过") {
		t.Fatal("通过卡标题应含 ✅ 已通过")
	}
	content := root["body"].(map[string]any)["elements"].([]any)[0].(map[string]any)["content"].(string)
	for _, want := range []string{"ou_approver", "task-t-1", "bash", "rm -rf /"} {
		if !strings.Contains(content, want) {
			t.Fatalf("通过卡内容缺少 %q: %s", want, content)
		}
	}
	if strings.Contains(content, "拒绝原因") {
		t.Fatal("通过卡不应含拒绝原因")
	}

	rejectedJSON := BuildApprovalResultCard(task, false, "风险过高", "ou_approver")
	root = nil
	if err := json.Unmarshal([]byte(rejectedJSON), &root); err != nil {
		t.Fatalf("拒绝卡 JSON 解析失败: %v", err)
	}
	if root["header"].(map[string]any)["template"] != "red" {
		t.Fatal("拒绝卡 template 应为 red")
	}
	if !strings.Contains(root["header"].(map[string]any)["title"].(map[string]any)["content"].(string), "❌ 已拒绝") {
		t.Fatal("拒绝卡标题应含 ❌ 已拒绝")
	}
	rejectedContent := root["body"].(map[string]any)["elements"].([]any)[0].(map[string]any)["content"].(string)
	if !strings.Contains(rejectedContent, "拒绝原因") || !strings.Contains(rejectedContent, "风险过高") {
		t.Fatalf("拒绝卡应含拒绝原因与原因文本: %s", rejectedContent)
	}

	noReasonJSON := BuildApprovalResultCard(task, false, "", "ou_approver")
	if strings.Contains(noReasonJSON, "拒绝原因") {
		t.Fatal("未填拒绝原因时结果卡不应含拒绝原因")
	}
}
