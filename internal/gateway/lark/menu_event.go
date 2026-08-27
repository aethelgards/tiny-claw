package lark

import (
	larkapplication "github.com/larksuite/oapi-sdk-go/v3/service/application/v6"
)

// IncomingMenuEvent 是从 Lark Bot 菜单事件中解析出的消息载体，
// 通过 channel 在"接收"与"处理"之间传递。
type IncomingMenuEvent struct {
	MenuKey   string // 菜单标识，如 "new_session"
	OpenID    string // 点击者 OpenID
	TenantKey string // 租户 key
}

// ParseMenuEvent 从 Lark Bot 菜单事件中解析 IncomingMenuEvent。
// 返回 false 表示该事件不应进入管道（事件为空 / 字段缺失）。
func ParseMenuEvent(event *larkapplication.P2BotMenuV6) (IncomingMenuEvent, bool) {
	if event == nil || event.Event == nil {
		return IncomingMenuEvent{}, false
	}

	evt := event.Event

	// 解析 EventKey（菜单标识）
	if evt.EventKey == nil {
		return IncomingMenuEvent{}, false
	}

	in := IncomingMenuEvent{
		MenuKey:   *evt.EventKey,
		TenantKey: event.TenantKey(),
	}

	// 解析用户 OpenID
	if evt.Operator != nil && evt.Operator.OperatorId != nil && evt.Operator.OperatorId.OpenId != nil {
		in.OpenID = *evt.Operator.OperatorId.OpenId
	}

	return in, true
}
