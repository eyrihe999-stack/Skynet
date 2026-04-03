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
type webhookRequest struct {
	RequestID   string              `json:"request_id"`
	Skill       string              `json:"skill"`
	Input       json.RawMessage     `json:"input"`
	Caller      protocol.CallerInfo `json:"caller"`
	TimeoutMs   int                 `json:"timeout_ms,omitempty"`
	CallbackURL string              `json:"callback_url,omitempty"` // Agent 异步完成后 POST 结果到此地址
}

// webhookResponse 是 Webhook Agent 返回的 HTTP 响应体。
type webhookResponse struct {
	Status string          `json:"status"` // "completed" | "failed"
	Output json.RawMessage `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// WebhookTransport 通过 HTTP POST 与 Agent 通信。
//
// 支持两种响应模式（对调用者透明）：
//   - 同步：Agent 返回 HTTP 200 + 结果 → 立即完成
//   - 异步：Agent 返回 HTTP 202 Accepted → 等待 Agent POST 回调
type WebhookTransport struct {
	agentID     string
	endpointURL string
	secret      string // agent_secret，用于 HMAC 签名
	callbackMgr *CallbackManager
	httpClient  *http.Client
	closeCh     chan struct{}
	closeOnce   sync.Once
}

// NewWebhookTransport 创建一个新的 Webhook 传输实例。
func NewWebhookTransport(agentID, endpointURL, secret string, callbackMgr *CallbackManager) *WebhookTransport {
	return &WebhookTransport{
		agentID:     agentID,
		endpointURL: endpointURL,
		secret:      secret,
		callbackMgr: callbackMgr,
		httpClient: &http.Client{
			Timeout: 0, // 由 context 控制超时
		},
		closeCh: make(chan struct{}),
	}
}

// SendInvoke 通过 HTTP POST 向 Agent 发送技能调用请求。
// Agent 可以同步返回结果（200）或接受后异步回调（202）。
func (w *WebhookTransport) SendInvoke(payload protocol.InvokePayload, timeout time.Duration) (string, *protocol.InvokeResult, error) {
	requestID := uuid.New().String()

	req := webhookRequest{
		RequestID:   requestID,
		Skill:       payload.Skill,
		Input:       payload.Input,
		Caller:      payload.Caller,
		TimeoutMs:   payload.TimeoutMs,
		CallbackURL: w.callbackMgr.CallbackURL(requestID),
	}

	result, err := w.doPost(requestID, req, timeout)
	if err != nil {
		return requestID, nil, err
	}
	return requestID, result, nil
}

// SendReply 通过 HTTP POST 发送多轮对话的回复。
func (w *WebhookTransport) SendReply(requestID string, payload protocol.ReplyPayload, timeout time.Duration) (*protocol.InvokeResult, error) {
	req := webhookRequest{
		RequestID:   requestID,
		Skill:       payload.Skill,
		Input:       payload.Input,
		Caller:      payload.Caller,
		CallbackURL: w.callbackMgr.CallbackURL(requestID),
	}

	return w.doPost(requestID, req, timeout)
}

// CloseCh 返回关闭信号通道。
func (w *WebhookTransport) CloseCh() <-chan struct{} {
	return w.closeCh
}

// Close 关闭此 transport。
func (w *WebhookTransport) Close() {
	w.closeOnce.Do(func() {
		close(w.closeCh)
	})
}

// doPost 执行 HTTP POST 并处理同步/异步两种响应模式。
func (w *WebhookTransport) doPost(requestID string, reqBody webhookRequest, timeout time.Duration) (*protocol.InvokeResult, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal webhook request: %w", err)
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// 先注册回调通道（无论同步异步都需要，异步时用于等待）
	callbackCh := w.callbackMgr.Register(requestID)
	defer w.callbackMgr.Remove(requestID)

	// 发送 HTTP 请求（用较短的连接超时，不用整个 invoke 超时）
	postTimeout := timeout
	if postTimeout > 60*time.Second {
		postTimeout = 60 * time.Second // HTTP POST 最多等 60s，剩余时间留给异步回调
	}

	ctx, cancel := context.WithTimeout(context.Background(), postTimeout)
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

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read webhook response: %w", err)
	}

	// ── 同步模式：Agent 直接返回结果 ──
	if resp.StatusCode == http.StatusOK {
		var webhookResp webhookResponse
		if err := json.Unmarshal(respBody, &webhookResp); err != nil {
			return nil, fmt.Errorf("parse webhook response: %w", err)
		}
		return &protocol.InvokeResult{
			Type: "result",
			Result: &protocol.ResultPayload{
				Status: webhookResp.Status,
				Output: webhookResp.Output,
				Error:  webhookResp.Error,
			},
		}, nil
	}

	// ── 异步模式：Agent 返回 202，等待回调 ──
	if resp.StatusCode == http.StatusAccepted {
		select {
		case result := <-callbackCh:
			return &protocol.InvokeResult{
				Type:   "result",
				Result: result,
			}, nil
		case <-time.After(timeout):
			return nil, fmt.Errorf("webhook async callback timed out after %s", timeout)
		case <-w.closeCh:
			return nil, fmt.Errorf("agent transport closed while waiting for callback")
		}
	}

	// ── 其他状态码：错误 ──
	return nil, fmt.Errorf("webhook returned HTTP %d: %s", resp.StatusCode, string(respBody))
}

// computeHMAC 计算 HMAC-SHA256 签名。
func computeHMAC(data []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}
