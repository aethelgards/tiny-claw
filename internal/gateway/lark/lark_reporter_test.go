package lark

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

type fakeMessageSender struct {
	mu         sync.Mutex
	texts      []string
	cards      []string
	chatIDs    []string
	tenantKeys []string
	sendErr    error
}

func (f *fakeMessageSender) SendMessage(ctx context.Context, chatID, tenantKey, content string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.texts = append(f.texts, content)
	f.chatIDs = append(f.chatIDs, chatID)
	f.tenantKeys = append(f.tenantKeys, tenantKey)
	return f.sendErr
}

func (f *fakeMessageSender) SendCardMessage(ctx context.Context, chatID, tenantKey, cardJSON string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cards = append(f.cards, cardJSON)
	f.chatIDs = append(f.chatIDs, chatID)
	f.tenantKeys = append(f.tenantKeys, tenantKey)
	return f.sendErr
}

func TestSendApprovalMessageCard(t *testing.T) {
	fake := &fakeMessageSender{}
	rep := newLarkReporter(fake, "chat1", "tk1")
	if err := rep.SendApprovalMessage(context.Background(), "task-1", "bash", "rm -rf /"); err != nil {
		t.Fatalf("SendApprovalMessage 失败: %v", err)
	}
	if len(fake.cards) != 1 {
		t.Fatalf("应发送 1 张卡片，实际 %d", len(fake.cards))
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(fake.cards[0]), &root); err != nil {
		t.Fatalf("卡片 JSON 解析失败: %v", err)
	}
	if root["schema"] != "2.0" {
		t.Fatalf("卡片 schema = %v, want 2.0", root["schema"])
	}
	if !strings.Contains(fake.cards[0], `"tag":"form"`) {
		t.Fatal("卡片应含 form 容器")
	}
	if len(fake.chatIDs) != 1 || fake.chatIDs[0] != "chat1" {
		t.Fatalf("chatID 透传错误: %v", fake.chatIDs)
	}
	if len(fake.tenantKeys) != 1 || fake.tenantKeys[0] != "tk1" {
		t.Fatalf("tenantKey 透传错误: %v", fake.tenantKeys)
	}
	if len(fake.texts) != 0 {
		t.Fatalf("不应发送文本消息，实际 %v", fake.texts)
	}
}

func TestSendApprovalMessageError(t *testing.T) {
	fake := &fakeMessageSender{sendErr: errors.New("send failed")}
	rep := newLarkReporter(fake, "chat1", "tk1")
	err := rep.SendApprovalMessage(context.Background(), "task-1", "bash", "rm -rf /")
	if err == nil || err.Error() != "send failed" {
		t.Fatalf("期望透传 send failed，得到 %v", err)
	}
}

func TestReporterTextSendStillWorks(t *testing.T) {
	fake := &fakeMessageSender{}
	rep := newLarkReporter(fake, "chat1", "tk1")
	rep.OnMessage(context.Background(), "hi")
	if len(fake.texts) != 1 || fake.texts[0] != "hi" {
		t.Fatalf("OnMessage 应走 SendMessage 发送文本，得到 %v", fake.texts)
	}
	if len(fake.cards) != 0 {
		t.Fatalf("文本消息不应触发卡片发送，实际 %v", fake.cards)
	}
}
