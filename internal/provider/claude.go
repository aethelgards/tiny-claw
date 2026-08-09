package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aethelgards/tiny-claw/internal/config"
	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ClaudeProvider 是基于 Anthropic Claude 协议（messages API，含兼容端点）的大模型提供者。
// 它实现 LLMProvider 接口，Generate 的核心职责：
//   - 将内部 schema.Message 翻译为 Anthropic MessageParam 格式（system 提示单独提取）；
//   - 将工具定义翻译为 Claude tool_use 格式；
//   - 发送请求并把响应中的 text / tool_use 块反向翻译回内部 schema.Message。
type ClaudeProvider struct {
	client    anthropic.Client // Anthropic SDK 客户端，负责与兼容端点通信
	model     string           // 使用的模型名
	maxTokens int64            // 单次生成的最大 token 数，必填（Claude API 要求）
}

func NewClaudeProvider(settings config.Settings) (*ClaudeProvider, error) {
	if settings.APIKey == "" {
		return nil, errors.New("ClaudeProvider: apiKey 不能为空")
	}
	return &ClaudeProvider{
		client: anthropic.NewClient(
			option.WithAPIKey(settings.APIKey),
			option.WithBaseURL(settings.BaseURL),
		),
		model:     settings.Model,
		maxTokens: int64(settings.MaxTokens),
	}, nil
}

func (p *ClaudeProvider) Generate(ctx context.Context, msgs []schema.Message, availableTools []schema.ToolDefinition) (*schema.Message, error) {
	var anthropicMsgs []anthropic.MessageParam
	var systemPrompt string

	// 1. 消息翻译
	for _, msg := range msgs {
		switch msg.Role {
		case schema.RoleSystem:
			systemPrompt = msg.Content
		case schema.RoleUser:
			if msg.ToolCallID != "" {
				anthropicMsgs = append(anthropicMsgs, anthropic.NewUserMessage(
					anthropic.NewToolResultBlock(msg.ToolCallID, msg.Content, false),
				))
			} else {
				anthropicMsgs = append(anthropicMsgs, anthropic.NewUserMessage(
					anthropic.NewTextBlock(msg.Content),
				))
			}
		case schema.RoleAssistant:
			var blocks []anthropic.ContentBlockParamUnion
			if msg.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
			}

			// 将历史工具调用转回 Claude 特有的 ToolUseBlockParam
			for _, tc := range msg.ToolCalls {
				var inputMap map[string]interface{}
				_ = json.Unmarshal(tc.Arguments, &inputMap)
				blocks = append(blocks, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    tc.ID,
						Name:  tc.Name,
						Input: inputMap,
					},
				})
			}
			if len(blocks) > 0 {
				anthropicMsgs = append(anthropicMsgs, anthropic.NewAssistantMessage(blocks...))
			}
		}
	}

	// 2. 工具 Schema 翻译
	var anthropicTools []anthropic.ToolUnionParam
	for _, toolDef := range availableTools {
		// ToolInputSchemaParam 是结构体，需要通过 Properties 字段精准填充
		var properties map[string]any
		var required []string

		m := toolDef.InputSchema
		if p, ok := m["properties"].(map[string]interface{}); ok {
			properties = p
		}
		if r, ok := m["required"].([]string); ok {
			required = r
		}
		tp := anthropic.ToolParam{
			Name:        toolDef.Name,
			Description: anthropic.String(toolDef.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: properties,
				Required:   required,
			},
		}
		anthropicTools = append(anthropicTools, anthropic.ToolUnionParam{OfTool: &tp})
	}

	// 3. 构建请求并发送
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: p.maxTokens,
		Messages:  anthropicMsgs,
	}

	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	if len(anthropicTools) > 0 {
		params.Tools = anthropicTools
	}

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("Claude/Zhipu API 请求失败: %w", err)
	}

	// 4. 反向解析
	resultMsg := &schema.Message{
		Role: schema.RoleAssistant,
	}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			resultMsg.Content += block.Text
		case "tool_use":
			argsBytes, _ := json.Marshal(block.Input)
			resultMsg.ToolCalls = append(resultMsg.ToolCalls, schema.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: argsBytes,
			})
		}
	}

	return resultMsg, nil
}
