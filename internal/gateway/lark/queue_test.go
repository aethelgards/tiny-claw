package lark

import (
	"testing"
	"time"
)

func TestMessageQueueEnqueueDequeue(t *testing.T) {
	q := NewMessageQueue(2)
	msg := IncomingMessage{MessageID: "m1", ChatID: "c1", Text: "hi"}

	if !q.Enqueue(msg) {
		t.Fatal("入队应成功")
	}
	select {
	case got := <-q.Messages():
		if got.MessageID != "m1" {
			t.Errorf("MessageID = %q, want m1", got.MessageID)
		}
	case <-time.After(time.Second):
		t.Fatal("超时未取到消息")
	}
}

func TestMessageQueueFullDrops(t *testing.T) {
	q := NewMessageQueue(1)
	if !q.Enqueue(IncomingMessage{MessageID: "m1"}) {
		t.Fatal("第一条应入队成功")
	}
	// 队列已满：第二条应立即返回 false，且不阻塞
	start := time.Now()
	if q.Enqueue(IncomingMessage{MessageID: "m2"}) {
		t.Fatal("满队列时入队应返回 false")
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("非阻塞入队不应耗时过长: %v", elapsed)
	}
}

func TestMessageQueueDedup(t *testing.T) {
	q := NewMessageQueue(4)
	if !q.Enqueue(IncomingMessage{MessageID: "m1"}) {
		t.Fatal("第一条应入队成功")
	}
	if q.Enqueue(IncomingMessage{MessageID: "m1"}) {
		t.Fatal("重复 msg_id 应被去重丢弃")
	}
	// 不同 msg_id 不受影响
	if !q.Enqueue(IncomingMessage{MessageID: "m2"}) {
		t.Fatal("不同 msg_id 应可入队")
	}
}

func TestMessageQueueEmptyIDNeverDeduped(t *testing.T) {
	q := NewMessageQueue(2)
	if !q.Enqueue(IncomingMessage{MessageID: ""}) {
		t.Fatal("空 msg_id 不应被去重拦截")
	}
	if !q.Enqueue(IncomingMessage{MessageID: ""}) {
		t.Fatal("空 msg_id 应始终可入队（不去重）")
	}
}

func TestDeduperTTLExpiry(t *testing.T) {
	d := NewDeduper(50 * time.Millisecond)

	if d.Seen("id1") {
		t.Fatal("首次应返回 false")
	}
	if !d.Seen("id1") {
		t.Fatal("TTL 内再次应返回 true")
	}

	time.Sleep(80 * time.Millisecond)
	if d.Seen("id1") {
		t.Fatal("TTL 过期后应重新视为未见过")
	}
}
