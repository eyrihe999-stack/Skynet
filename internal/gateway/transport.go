package gateway

import (
	"time"

	"github.com/eyrihe999-stack/Skynet-sdk/protocol"
)

// AgentTransport 抽象了与 Agent 之间的通信通道。
//
// 两种实现：
//   - AgentConn（tunnel 模式）：通过 WebSocket 长连接通信
//   - WebhookTransport（direct 模式）：通过 HTTP POST 回调通信
type AgentTransport interface {
	// SendInvoke 发送技能调用请求并等待结果。
	// 返回 requestID（用于多轮对话关联）、调用结果和错误。
	SendInvoke(payload protocol.InvokePayload, timeout time.Duration) (requestID string, result *protocol.InvokeResult, err error)

	// SendReply 发送多轮对话的回复并等待下一轮响应。
	SendReply(requestID string, payload protocol.ReplyPayload, timeout time.Duration) (*protocol.InvokeResult, error)

	// CloseCh 返回连接关闭信号通道。
	CloseCh() <-chan struct{}

	// Close 关闭连接并释放资源。
	Close()
}
