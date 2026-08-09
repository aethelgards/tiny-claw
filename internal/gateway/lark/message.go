package lark

import (
	"encoding/json"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// IncomingMessage 是从 Lark 事件中解析出的最小消息载体，
// 通过 channel 在"接收"与"处理"之间传递。
type IncomingMessage struct {
	MessageID string // 去重键（Lark 消息全局唯一）
	ChatID    string // 回复目标 chat_id（群聊/私聊通用）
	OpenID    string // 发送者 open_id（备用字段）
	TenantKey string // 租户 key，发送回复时透传
	Text      string // 解析出的纯文本
}

// textContent 对应 text 消息 content 的 JSON 结构。
type textContent struct {
	Text string `json:"text"`
}

// ParseMessageEvent 从 Lark 事件中解析 IncomingMessage。
// 返回 false 表示该事件不应进入管道（机器人自身消息 / 非 text 消息 / 字段缺失）。
func ParseMessageEvent(event *larkim.P2MessageReceiveV1) (IncomingMessage, bool) {
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Sender == nil {
		return IncomingMessage{}, false
	}

	// 过滤机器人自身消息，防止回环
	if senderType := event.Event.Sender.SenderType; senderType != nil && *senderType == "app" {
		return IncomingMessage{}, false
	}

	msg := event.Event.Message
	// v1 仅支持文本消息
	if msg.MessageType == nil || *msg.MessageType != "text" {
		return IncomingMessage{}, false
	}
	if msg.MessageId == nil || msg.ChatId == nil || msg.Content == nil {
		return IncomingMessage{}, false
	}

	var tc textContent
	if err := json.Unmarshal([]byte(*msg.Content), &tc); err != nil {
		return IncomingMessage{}, false
	}

	in := IncomingMessage{
		MessageID: *msg.MessageId,
		ChatID:    *msg.ChatId,
		TenantKey: event.TenantKey(),
		Text:      tc.Text,
	}
	if senderID := event.Event.Sender.SenderId; senderID != nil && senderID.OpenId != nil {
		in.OpenID = *senderID.OpenId
	}
	return in, true
}
