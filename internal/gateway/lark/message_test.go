package lark

import (
	"testing"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// buildEvent 构造一个最小可用的 P2MessageReceiveV1 事件。
func buildEvent(msgType, content, senderType string) *larkim.P2MessageReceiveV1 {
	return &larkim.P2MessageReceiveV1{
		EventV2Base: &larkevent.EventV2Base{
			Header: &larkevent.EventHeader{TenantKey: "tenant-1"},
		},
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId:   &larkim.UserId{OpenId: new("ou_123")},
				SenderType: new(senderType),
			},
			Message: &larkim.EventMessage{
				MessageId:   new("om_abc"),
				ChatId:      new("oc_xyz"),
				MessageType: new(msgType),
				Content:     new(content),
			},
		},
	}
}

func TestParseMessageEventText(t *testing.T) {
	event := buildEvent("text", `{"text":"你好"}`, "user")

	msg, ok := ParseMessageEvent(event)
	if !ok {
		t.Fatal("text 消息应被解析")
	}
	if msg.MessageID != "om_abc" {
		t.Errorf("MessageID = %q, want om_abc", msg.MessageID)
	}
	if msg.ChatID != "oc_xyz" {
		t.Errorf("ChatID = %q, want oc_xyz", msg.ChatID)
	}
	if msg.OpenID != "ou_123" {
		t.Errorf("OpenID = %q, want ou_123", msg.OpenID)
	}
	if msg.TenantKey != "tenant-1" {
		t.Errorf("TenantKey = %q, want tenant-1", msg.TenantKey)
	}
	if msg.Text != "你好" {
		t.Errorf("Text = %q, want 你好", msg.Text)
	}
}

func TestParseMessageEventBotSender(t *testing.T) {
	event := buildEvent("text", `{"text":"hi"}`, "app")

	if _, ok := ParseMessageEvent(event); ok {
		t.Fatal("机器人自身消息应被过滤")
	}
}

func TestParseMessageEventNonText(t *testing.T) {
	event := buildEvent("image", `{"image_key":"img_v2_xxx"}`, "user")

	if _, ok := ParseMessageEvent(event); ok {
		t.Fatal("非 text 消息应被过滤")
	}
}

func TestParseMessageEventMalformedContent(t *testing.T) {
	event := buildEvent("text", `not-json`, "user")

	if _, ok := ParseMessageEvent(event); ok {
		t.Fatal("畸形 content 应解析失败")
	}
}

func TestParseMessageEventNil(t *testing.T) {
	if _, ok := ParseMessageEvent(nil); ok {
		t.Fatal("nil 事件应返回 false")
	}
}

func TestParseMessageEventMissingFields(t *testing.T) {
	event := buildEvent("text", `{"text":"hi"}`, "user")
	event.Event.Message.MessageId = nil

	if _, ok := ParseMessageEvent(event); ok {
		t.Fatal("缺少 messageId 应返回 false")
	}
}
