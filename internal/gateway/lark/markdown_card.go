package lark

// BuildMarkdownCard 构建渲染 markdown 内容的互动卡片（card JSON v2）。
// 飞书 text 消息不支持 markdown 渲染，仅 interactive 卡片的 markdown 元素可渲染
// 加粗、代码块、列表、标题等格式，与审批卡片保持同一套卡片体系。
func BuildMarkdownCard(content string) string {
	card := map[string]any{
		"schema": "2.0",
		"body": map[string]any{
			"elements": []any{
				map[string]any{"tag": "markdown", "content": content},
			},
		},
	}
	return mustJSON(card)
}
