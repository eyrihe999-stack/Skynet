package handler

import (
	"io"

	"github.com/eyrihe999-stack/Skynet/internal/authz"
	"github.com/eyrihe999-stack/Skynet/internal/gateway"
	"github.com/gin-gonic/gin"
)

type EventsHandler struct {
	eventBus *gateway.EventBus
	authSvc  *authz.Service
}

func NewEventsHandler(eventBus *gateway.EventBus, authSvc *authz.Service) *EventsHandler {
	return &EventsHandler{eventBus: eventBus, authSvc: authSvc}
}

// Stream 处理 SSE 事件流 GET /api/v1/events?token=xxx
// SSE (EventSource) 不支持自定义 Header，通过 query param 传递 JWT token 认证。
func (h *EventsHandler) Stream(c *gin.Context) {
	// 认证：从 query param 获取 token
	token := c.Query("token")
	if token == "" {
		c.JSON(401, gin.H{"error": "token required"})
		return
	}
	if _, err := h.authSvc.ValidateJWT(token); err != nil {
		c.JSON(401, gin.H{"error": "invalid token"})
		return
	}

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
