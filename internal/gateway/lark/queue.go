package lark

import (
	"sync"
	"time"
)

// defaultDedupTTL 去重条目的存活时间，超过后允许相同 msg_id 重新入队。
const defaultDedupTTL = 10 * time.Minute

// Deduper 基于 msg_id 的短期去重器，防止 WS 断线重连导致的重复投递。
// 线程安全；仅记录 msg_id，不感知消息内容。
type Deduper struct {
	mu  sync.Mutex
	m   map[string]time.Time
	ttl time.Duration
}

func NewDeduper(ttl time.Duration) *Deduper {
	return &Deduper{
		m:   make(map[string]time.Time),
		ttl: ttl,
	}
}

// Seen 判断 id 是否已见过：首次返回 false 并记录；TTL 内再次出现返回 true。
// 顺带惰性清理过期条目，防止 map 无限增长。
func (d *Deduper) Seen(id string) bool {
	if id == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	if t, ok := d.m[id]; ok {
		if now.Sub(t) < d.ttl {
			return true
		}
		delete(d.m, id)
	}
	d.m[id] = now
	return false
}

// MessageQueue 有界消息队列：入队永不阻塞，满则丢弃（背压策略）。
// 同一 msg_id 只会被入队一次（去重）。
type MessageQueue struct {
	ch    chan IncomingMessage
	dedup *Deduper
}

func NewMessageQueue(size int) *MessageQueue {
	return &MessageQueue{
		ch:    make(chan IncomingMessage, size),
		dedup: NewDeduper(defaultDedupTTL),
	}
}

// Enqueue 非阻塞入队。返回 false 表示消息被丢弃：
// 可能是重复消息（msg_id 已处理过），也可能是队列已满。
func (q *MessageQueue) Enqueue(msg IncomingMessage) bool {
	if q.dedup.Seen(msg.MessageID) {
		return false
	}
	select {
	case q.ch <- msg:
		return true
	default:
		return false
	}
}

// Messages 返回只读消费通道。
func (q *MessageQueue) Messages() <-chan IncomingMessage {
	return q.ch
}
