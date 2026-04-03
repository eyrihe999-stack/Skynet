package handler

import (
	"io"

	"github.com/gin-gonic/gin"
	"github.com/eyrihe999-stack/Skynet/internal/gateway"
)

// EventsHandler 处理 SSE 事件流端点。
// 向已连接的客户端实时推送平台事件（Agent 上下线、调用完成等）。
type EventsHandler struct {
	eventBus *gateway.EventBus
}

// NewEventsHandler 创建并返回一个新的 EventsHandler 实例。
func NewEventsHandler(eventBus *gateway.EventBus) *EventsHandler {
	return &EventsHandler{eventBus: eventBus}
}

// Stream 处理 SSE 事件流 GET /api/v1/events。
// 客户端连接后将持续接收平台事件，直到客户端断开连接。
func (h *EventsHandler) Stream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ch := h.eventBus.Subscribe()
	defer h.eventBus.Unsubscribe(ch)

	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-ch:
			if !ok {
				return false
			}
			data, err := gateway.FormatSSE(event)
			if err != nil {
				return true
			}
			c.Writer.Write(data)
			c.Writer.Flush()
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}
