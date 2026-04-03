package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/eyrihe999-stack/Skynet/internal/gateway"
	"github.com/eyrihe999-stack/Skynet/pkg/response"
	"github.com/eyrihe999-stack/Skynet-sdk/protocol"
)

// CallbackHandler 处理 Webhook Agent 异步回调的 HTTP 端点。
//
// 当 Agent 无法在一次 HTTP 请求内完成处理时（返回 202 Accepted），
// 它会在完成后将结果 POST 到 Platform 的回调地址。
// 此 Handler 接收结果并投递到对应的等待通道。
type CallbackHandler struct {
	callbackMgr *gateway.CallbackManager
}

func NewCallbackHandler(callbackMgr *gateway.CallbackManager) *CallbackHandler {
	return &CallbackHandler{callbackMgr: callbackMgr}
}

// callbackRequest 是 Agent 回调时发送的请求体。
type callbackRequest struct {
	Status string          `json:"status" binding:"required"` // "completed" | "failed"
	Output json.RawMessage `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Handle 处理异步回调请求（POST /api/v1/callbacks/:request_id）。
//
// Agent 完成处理后，将结果 POST 到此端点。
// Platform 将结果投递到等待中的 WebhookTransport.SendInvoke 调用。
func (h *CallbackHandler) Handle(c *gin.Context) {
	requestID := c.Param("request_id")

	var req callbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result := &protocol.ResultPayload{
		Status: req.Status,
		Output: req.Output,
		Error:  req.Error,
	}

	if !h.callbackMgr.Deliver(requestID, result) {
		response.NotFound(c, "no pending request for this callback (may have timed out)")
		return
	}

	c.JSON(200, gin.H{"status": "delivered"})
}
