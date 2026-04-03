package gateway

import (
	"encoding/json"
	"sync"
)

// Event 表示一个平台事件。
type Event struct {
	Type string `json:"type"` // agent_online, agent_offline, invoke_completed
	Data any    `json:"data"`
}

// EventBus 是一个简单的发布-订阅事件总线。
// 订阅者通过 Subscribe 获取事件通道，通过 Unsubscribe 取消。
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
}

// NewEventBus 创建并返回一个新的 EventBus 实例。
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[chan Event]struct{}),
	}
}

// Subscribe 注册一个新的事件订阅者，返回事件接收通道。
func (b *EventBus) Subscribe() chan Event {
	ch := make(chan Event, 16) // 带缓冲，防止慢消费者阻塞
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe 取消订阅并关闭通道。
func (b *EventBus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subscribers, ch)
	close(ch)
}

// Publish 向所有订阅者广播事件。
// 非阻塞发送：如果某个订阅者的缓冲满了，跳过该订阅者（避免慢消费者阻塞全局）。
func (b *EventBus) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// 跳过慢消费者
		}
	}
}

// PublishJSON 序列化 data 后发布事件（便捷方法）。
func (b *EventBus) PublishJSON(eventType string, data any) {
	b.Publish(Event{Type: eventType, Data: data})
}

// FormatSSE 将事件格式化为 SSE 消息格式。
func FormatSSE(event Event) ([]byte, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	// SSE 格式: "event: {type}\ndata: {json}\n\n"
	msg := "event: " + event.Type + "\ndata: " + string(data) + "\n\n"
	return []byte(msg), nil
}
