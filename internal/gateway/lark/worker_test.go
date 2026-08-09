package lark

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeProcessor 记录调用序列，可按配置触发 panic 或返回错误。
type fakeProcessor struct {
	mu       sync.Mutex
	order    []string
	panicIdx int32 // 第 N 次调用触发 panic，0 表示永不
	failIdx  int32 // 第 N 次调用返回错误，0 表示永不
	calls    atomic.Int32
}

func (f *fakeProcessor) Process(ctx context.Context, msg IncomingMessage) error {
	idx := f.calls.Add(1)
	if f.panicIdx == idx {
		panic("boom")
	}
	if f.failIdx == idx {
		return errors.New("process failed")
	}
	f.mu.Lock()
	f.order = append(f.order, msg.MessageID)
	f.mu.Unlock()
	return nil
}

func (f *fakeProcessor) processed() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.order))
	copy(out, f.order)
	return out
}

// waitFor 轮询直到 cond 为 true 或超时。
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("等待条件超时")
}

func TestWorkerSerialOrder(t *testing.T) {
	q := NewMessageQueue(8)
	fp := &fakeProcessor{}
	w := NewWorker(q, fp)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	for _, id := range []string{"m1", "m2", "m3"} {
		if !q.Enqueue(IncomingMessage{MessageID: id}) {
			t.Fatalf("入队 %s 失败", id)
		}
	}

	want := []string{"m1", "m2", "m3"}
	waitFor(t, time.Second, func() bool {
		got := fp.processed()
		if len(got) != len(want) {
			return false
		}
		for i := range want {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	})
}

func TestWorkerPanicRecovery(t *testing.T) {
	q := NewMessageQueue(4)
	fp := &fakeProcessor{panicIdx: 1}
	notified := make(chan struct{}, 4)
	w := NewWorker(q, fp, func(ctx context.Context, msg IncomingMessage, err error) {
		notified <- struct{}{}
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go w.Run(ctx)

	if !q.Enqueue(IncomingMessage{MessageID: "m1"}) {
		t.Fatal("入队 m1 失败")
	}
	if !q.Enqueue(IncomingMessage{MessageID: "m2"}) {
		t.Fatal("入队 m2 失败")
	}

	// m1 触发 panic → onError 被调用；worker 存活继续处理 m2
	select {
	case <-notified:
	case <-time.After(time.Second):
		t.Fatal("panic 后未触发 onError")
	}
	waitFor(t, time.Second, func() bool {
		return len(fp.processed()) == 1 && fp.processed()[0] == "m2"
	})
}

func TestWorkerErrorNotifies(t *testing.T) {
	q := NewMessageQueue(2)
	fp := &fakeProcessor{failIdx: 1}
	var notifiedErr atomic.Value
	w := NewWorker(q, fp, func(ctx context.Context, msg IncomingMessage, err error) {
		notifiedErr.Store(err.Error())
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go w.Run(ctx)

	if !q.Enqueue(IncomingMessage{MessageID: "m1"}) {
		t.Fatal("入队失败")
	}

	waitFor(t, time.Second, func() bool {
		v := notifiedErr.Load()
		return v != nil && v.(string) == "process failed"
	})
}

func TestWorkerCtxCancelStops(t *testing.T) {
	q := NewMessageQueue(2)
	fp := &fakeProcessor{}
	w := NewWorker(q, fp)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ctx 取消后 worker 未退出")
	}
}
