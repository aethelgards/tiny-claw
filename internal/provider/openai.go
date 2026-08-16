// internal/provider/openai.go
package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/aethelgards/tiny-claw/internal/config"
	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"
)

// OpenAIProvider 是基于 OpenAI 兼容协议（含智谱 GLM 等兼容端点）的大模型提供者。
// 它实现 LLMProvider 接口，Generate 的核心职责：
//   - 将内部 schema.Message 翻译为 OpenAI ChatCompletion 消息格式（含 ToolCalls 还原）；
//   - 将工具定义翻译为 OpenAI function calling 格式；
//   - 发送请求并把 API 响应反向翻译回内部 schema.Message。
type OpenAIProvider struct {
	client    openai.Client // OpenAI SDK 客户端，负责与兼容端点通信
	model     string        // 使用的模型名，如 "glm-4.6"
	maxTokens int           // 单次生成的最大 token 数，<=0 时不传给 API（由服务端默认）
}

func NewOpenAIProvider(settings config.Settings) (*OpenAIProvider, error) {
	if settings.APIKey == "" {
		return nil, errors.New("OpenAIProvider: apiKey 不能为空")
	}
	return &OpenAIProvider{
		client: openai.NewClient(
			option.WithAPIKey(settings.APIKey),
			option.WithBaseURL(settings.BaseURL),
		),
		model:     settings.Model,
		maxTokens: settings.MaxTokens,
	}, nil
}

func (p *OpenAIProvider) Generate(ctx context.Context, msgs []schema.Message, availableTools []schema.ToolDefinition) (*schema.Message, error) {
	var openaiMsgs []openai.ChatCompletionMessageParamUnion

	// 1. 翻译上下文消息
	for _, msg := range msgs {
		switch msg.Role {
		case schema.RoleSystem:
			openaiMsgs = append(openaiMsgs, openai.SystemMessage(msg.Content))

		case schema.RoleUser:
			if msg.ToolCallID != "" {
				// 注意：v3 新版参数顺序是 (content, toolCallID)
				openaiMsgs = append(openaiMsgs, openai.ToolMessage(msg.Content, msg.ToolCallID))
			} else {
				openaiMsgs = append(openaiMsgs, openai.UserMessage(msg.Content))
			}

		case schema.RoleAssistant:
			astParam := openai.ChatCompletionAssistantMessageParam{}

			if msg.Content != "" {
				astParam.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: openai.String(msg.Content),
				}
			}

			// 【重要】如果历史包含 ToolCalls，必须原样放回，以维系大模型的逻辑链
			if len(msg.ToolCalls) > 0 {
				var toolCalls []openai.ChatCompletionMessageToolCallUnionParam
				for _, tc := range msg.ToolCalls {
					// OfFunction 对应 GetFunction()，字段类型严格要求为指针
					toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID:   tc.ID,
							Type: "function",
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Name,
								Arguments: string(tc.Arguments),
							},
						},
					})
				}
				astParam.ToolCalls = toolCalls
			}

			openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &astParam,
			})
		}
	}

	// 2. 翻译工具定义 (v3 新 API 特性适配)
	var openaiTools []openai.ChatCompletionToolUnionParam
	for _, toolDef := range availableTools {
		params := shared.FunctionParameters(toolDef.InputSchema)

		openaiTools = append(openaiTools, openai.ChatCompletionFunctionTool(
			shared.FunctionDefinitionParam{
				Name:        toolDef.Name,
				Description: openai.String(toolDef.Description),
				Parameters:  params,
			},
		))
	}

	// 3. 构建请求并发送
	params := openai.ChatCompletionNewParams{
		Model:    p.model,
		Messages: openaiMsgs,
	}
	if p.maxTokens > 0 {
		params.MaxTokens = param.NewOpt[int64](int64(p.maxTokens))
	}

	// 【慢思考机制支撑】仅当 availableTools 存在时才挂载 Tools
	if len(openaiTools) > 0 {
		params.Tools = openaiTools
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("OpenAI/Zhipu API 请求失败: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("API 返回了空的 Choices")
	}

	// 4. 将 API Response 反向翻译为内部 schema.Message
	choice := resp.Choices[0].Message
	resultMsg := &schema.Message{
		Role:    schema.RoleAssistant,
		Content: choice.Content,
		Usage: &schema.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
		},
	}

	for _, tc := range choice.ToolCalls {
		if tc.Type == "function" {
			resultMsg.ToolCalls = append(resultMsg.ToolCalls, schema.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: []byte(tc.Function.Arguments), // 提取 JSON 字符串字节
			})
		}
	}

	return resultMsg, nil
}
