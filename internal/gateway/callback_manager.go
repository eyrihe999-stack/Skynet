package gateway

import (
	"sync"

	"github.com/eyrihe999-stack/Skynet-sdk/protocol"
)

// CallbackManager 管理 Webhook Agent 异步回调的待处理请求。
//
// 当 Webhook Agent 返回 HTTP 202（接受但未完成）时，WebhookTransport 会在此处
// 注册一个等待通道。Agent 处理完成后 POST 到回调端点，CallbackHandler 将结果
// 投递到对应通道，从而唤醒等待中的 SendInvoke 调用。
type CallbackManager struct {
	externalURL string
	mu          sync.Mutex
	pending     map[string]chan *protocol.ResultPayload
}

// NewCallbackManager 创建回调管理器。
// externalURL 是 Platform 对外可达的地址（如 "http://host:9090"），
// 用于拼接回调 URL 告知 Agent。
func NewCallbackManager(externalURL string) *CallbackManager {
	return &CallbackManager{
		externalURL: externalURL,
		pending:     make(map[string]chan *protocol.ResultPayload),
	}
}

// CallbackURL 返回指定 requestID 的回调地址。
func (m *CallbackManager) CallbackURL(requestID string) string {
	return m.externalURL + "/api/v1/callbacks/" + requestID
}

// Register 为一个请求注册等待通道，返回该通道以供调用方阻塞等待。
func (m *CallbackManager) Register(requestID string) chan *protocol.ResultPayload {
	ch := make(chan *protocol.ResultPayload, 1)
	m.mu.Lock()
	m.pending[requestID] = ch
	m.mu.Unlock()
	return ch
}

// Deliver 将 Agent 回调的结果投递到对应的等待通道。
// 如果 requestID 不存在（已超时被清理），返回 false。
func (m *CallbackManager) Deliver(requestID string, result *protocol.ResultPayload) bool {
	m.mu.Lock()
	ch, ok := m.pending[requestID]
	if ok {
		delete(m.pending, requestID)
	}
	m.mu.Unlock()

	if !ok {
		return false
	}

	ch <- result
	return true
}

// Remove 移除指定请求的等待通道（用于超时后清理）。
func (m *CallbackManager) Remove(requestID string) {
	m.mu.Lock()
	delete(m.pending, requestID)
	m.mu.Unlock()
}
