package lark

import (
	"context"
	"encoding/json"
	"log/slog"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/pkg/errors"
)

// Bot 封装 Lark WebSocket 客户端与消息发送能力。
// 凭据来自配置层，不在代码中硬编码。
type Bot struct {
	appID     string
	appSecret string
	cli       *lark.Client
	wscli     *larkws.Client
}

// NewBot 创建 Bot；WebSocket 连接在 Start 时建立。
func NewBot(appID, appSecret string) *Bot {
	return &Bot{
		appID:     appID,
		appSecret: appSecret,
		cli:       lark.NewClient(appID, appSecret),
	}
}

// Start 注册事件处理并建立 WebSocket 长连接。
// register 回调用于装配事件处理器（如把消息入队）。
func (b *Bot) Start(ctx context.Context, register func(d *dispatcher.EventDispatcher)) error {
	d := dispatcher.NewEventDispatcher("", "")
	register(d)

	b.wscli = larkws.NewClient(b.appID, b.appSecret,
		larkws.WithEventHandler(d),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)
	if err := b.wscli.Start(ctx); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// Stop 关闭 WebSocket 长连接。
func (b *Bot) Stop() {
	if b.wscli != nil {
		b.wscli.Close()
	}
}

// SendMessage 向指定 chat_id 发送文本消息（群聊/私聊通用）。
// tenantKey 为空时使用默认租户。
func (b *Bot) SendMessage(ctx context.Context, chatID, tenantKey, content string) error {
	body, err := json.Marshal(map[string]string{"text": content})
	if err != nil {
		return errors.WithStack(err)
	}

	resp, err := b.cli.Im.Message.Create(
		ctx,
		larkim.NewCreateMessageReqBuilder().
			ReceiveIdType("chat_id").
			Body(larkim.NewCreateMessageReqBodyBuilder().
				MsgType(larkim.MsgTypeText).
				ReceiveId(chatID).
				Content(string(body)).
				Build()).
			Build(),
		larkcore.WithTenantKey(tenantKey),
	)
	if err != nil {
		return errors.WithStack(err)
	}
	if !resp.Success() {
		return errors.Errorf("send lark message failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	slog.InfoContext(ctx, "send lark message",
		slog.String("chatID", chatID),
		slog.String("content", content),
		slog.String("tenantKey", tenantKey))

	return nil
}

// SendCardMessage 发送互动卡片消息（interactive）；cardJSON 为 card JSON v2 字符串。
func (b *Bot) SendCardMessage(ctx context.Context, chatID, tenantKey, cardJSON string) error {
	resp, err := b.cli.Im.Message.Create(
		ctx,
		larkim.NewCreateMessageReqBuilder().
			ReceiveIdType("chat_id").
			Body(larkim.NewCreateMessageReqBodyBuilder().
				MsgType(larkim.MsgTypeInteractive).
				ReceiveId(chatID).
				Content(cardJSON).
				Build()).
			Build(),
		larkcore.WithTenantKey(tenantKey),
	)
	if err != nil {
		return errors.WithStack(err)
	}
	if !resp.Success() {
		return errors.Errorf("send card message failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	slog.InfoContext(ctx, "card message sent", slog.String("chatID", chatID))
	return nil
}
