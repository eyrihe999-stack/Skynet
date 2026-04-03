package gateway

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/eyrihe999-stack/Skynet-sdk/protocol"
)

// webhookRequest 是 Platform 向 Webhook Agent 发送的 HTTP 请求体。
// 它是 InvokePayload 的超集，额外携带 request_id 以便 Agent 追踪。
type webhookRequest struct {
	RequestID string          `json:"request_id"`
	Skill     string          `json:"skill"`
	Input     json.RawMessage `json:"input"`
	Caller    protocol.CallerInfo `json:"caller"`
	TimeoutMs int             `json:"timeout_ms,omitempty"`
}

// webhookResponse 是 Webhook Agent 返回的 HTTP 响应体。
type webhookResponse struct {
	Status string          `json:"status"` // "completed" | "failed"
	Output json.RawMessage `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// WebhookTransport 通过 HTTP POST 与 Agent 通信。
//
// 当 Agent 以 direct（webhook）模式注册时，Platform 持有该 transport，
// 在需要调用 Skill 时向 Agent 的 EndpointURL 发送 HTTP 请求。
//
// 安全性：使用 HMAC-SHA256 对请求体签名，Agent 可通过
// X-Skynet-Signature 头验证请求来源。
type WebhookTransport struct {
	agentID     string
	endpointURL string
	secret      string // agent_secret，用于 HMAC 签名
	httpClient  *http.Client
	closeCh     chan struct{}
	closeOnce   sync.Once
}

// NewWebhookTransport 创建一个新的 Webhook 传输实例。
func NewWebhookTransport(agentID, endpointURL, secret string) *WebhookTransport {
	return &WebhookTransport{
		agentID:     agentID,
		endpointURL: endpointURL,
		secret:      secret,
		httpClient: &http.Client{
			Timeout: 0, // 由 context 控制超时
		},
		closeCh: make(chan struct{}),
	}
}

// SendInvoke 通过 HTTP POST 向 Agent 发送技能调用请求。
func (w *WebhookTransport) SendInvoke(payload protocol.InvokePayload, timeout time.Duration) (string, *protocol.InvokeResult, error) {
	requestID := uuid.New().String()

	req := webhookRequest{
		RequestID: requestID,
		Skill:     payload.Skill,
		Input:     payload.Input,
		Caller:    payload.Caller,
		TimeoutMs: payload.TimeoutMs,
	}

	result, err := w.doPost(requestID, req, timeout)
	if err != nil {
		return requestID, nil, err
	}
	return requestID, result, nil
}

// SendReply 通过 HTTP POST 发送多轮对话的回复。
// Webhook Agent 是无状态的，每次都发送完整的合并后输入，Agent 当作新调用处理。
func (w *WebhookTransport) SendReply(requestID string, payload protocol.ReplyPayload, timeout time.Duration) (*protocol.InvokeResult, error) {
	req := webhookRequest{
		RequestID: requestID,
		Skill:     payload.Skill,
		Input:     payload.Input, // 已合并的完整输入
		Caller:    payload.Caller,
	}

	return w.doPost(requestID, req, timeout)
}

// CloseCh 返回关闭信号通道。
func (w *WebhookTransport) CloseCh() <-chan struct{} {
	return w.closeCh
}

// Close 关闭此 transport，标记为不可用。
func (w *WebhookTransport) Close() {
	w.closeOnce.Do(func() {
		close(w.closeCh)
	})
}

// doPost 执行 HTTP POST 请求并解析响应。
func (w *WebhookTransport) doPost(requestID string, reqBody webhookRequest, timeout time.Duration) (*protocol.InvokeResult, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal webhook request: %w", err)
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, w.endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create webhook request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Skynet-Request-ID", requestID)
	httpReq.Header.Set("X-Skynet-Agent-ID", w.agentID)
	if w.secret != "" {
		httpReq.Header.Set("X-Skynet-Signature", "sha256="+computeHMAC(body, w.secret))
	}

	resp, err := w.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("webhook POST failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return nil, fmt.Errorf("read webhook response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("webhook returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var webhookResp webhookResponse
	if err := json.Unmarshal(respBody, &webhookResp); err != nil {
		return nil, fmt.Errorf("parse webhook response: %w", err)
	}

	result := &protocol.ResultPayload{
		Status: webhookResp.Status,
		Output: webhookResp.Output,
		Error:  webhookResp.Error,
	}

	return &protocol.InvokeResult{
		Type:   "result",
		Result: result,
	}, nil
}

// computeHMAC 计算 HMAC-SHA256 签名。
func computeHMAC(data []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}
