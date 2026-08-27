package lark

import (
	"testing"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkapplication "github.com/larksuite/oapi-sdk-go/v3/service/application/v6"
)

func buildMenuEvent(eventKey string) *larkapplication.P2BotMenuV6 {
	return &larkapplication.P2BotMenuV6{
		EventV2Base: &larkevent.EventV2Base{
			Header: &larkevent.EventHeader{TenantKey: "tenant-1"},
		},
		Event: &larkapplication.P2BotMenuV6Data{
			EventKey: &eventKey,
			Operator: &larkapplication.Operator{
				OperatorId: &larkapplication.UserId{
					OpenId: new("ou_456"),
				},
			},
		},
	}
}

func TestParseMenuEventValid(t *testing.T) {
	event := buildMenuEvent("new_session")

	evt, ok := ParseMenuEvent(event)
	if !ok {
		t.Fatal("有效菜单事件应解析成功")
	}
	if evt.MenuKey != "new_session" {
		t.Errorf("MenuKey = %q, want new_session", evt.MenuKey)
	}
	if evt.OpenID != "ou_456" {
		t.Errorf("OpenID = %q, want ou_456", evt.OpenID)
	}
	if evt.TenantKey != "tenant-1" {
		t.Errorf("TenantKey = %q, want tenant-1", evt.TenantKey)
	}
}

func TestParseMenuEventNilEvent(t *testing.T) {
	if _, ok := ParseMenuEvent(nil); ok {
		t.Fatal("nil 事件应返回 false")
	}
}

func TestParseMenuEventNilData(t *testing.T) {
	event := &larkapplication.P2BotMenuV6{
		EventV2Base: &larkevent.EventV2Base{
			Header: &larkevent.EventHeader{TenantKey: "tenant-1"},
		},
	}
	if _, ok := ParseMenuEvent(event); ok {
		t.Fatal("nil Event data 应返回 false")
	}
}

func TestParseMenuEventNilEventKey(t *testing.T) {
	event := &larkapplication.P2BotMenuV6{
		EventV2Base: &larkevent.EventV2Base{
			Header: &larkevent.EventHeader{TenantKey: "tenant-1"},
		},
		Event: &larkapplication.P2BotMenuV6Data{
			Operator: &larkapplication.Operator{
				OperatorId: &larkapplication.UserId{
					OpenId: new("ou_456"),
				},
			},
		},
	}
	if _, ok := ParseMenuEvent(event); ok {
		t.Fatal("nil EventKey 应返回 false")
	}
}

func TestParseMenuEventNilOperator(t *testing.T) {
	eventKey := "new_session"
	event := &larkapplication.P2BotMenuV6{
		EventV2Base: &larkevent.EventV2Base{
			Header: &larkevent.EventHeader{TenantKey: "tenant-1"},
		},
		Event: &larkapplication.P2BotMenuV6Data{
			EventKey: &eventKey,
		},
	}
	evt, ok := ParseMenuEvent(event)
	if !ok {
		t.Fatal("nil Operator 应仍解析成功，只是 OpenID 为空")
	}
	if evt.OpenID != "" {
		t.Errorf("OpenID = %q, want empty", evt.OpenID)
	}
}
