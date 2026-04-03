package handler

import (
	"encoding/json"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/skynetplatform/skynet/internal/authz"
	"github.com/skynetplatform/skynet/internal/gateway"
	"github.com/skynetplatform/skynet/pkg/response"
)

type TaskHandler struct {
	gatewaySvc *gateway.Service
}

func NewTaskHandler(gatewaySvc *gateway.Service) *TaskHandler {
	return &TaskHandler{gatewaySvc: gatewaySvc}
}

// GetTask 查询任务状态 GET /api/v1/tasks/:task_id
func (h *TaskHandler) GetTask(c *gin.Context) {
	taskID := c.Param("task_id")
	resp, err := h.gatewaySvc.GetTask(taskID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// Reply 回复多轮对话追问 POST /api/v1/tasks/:task_id/reply
func (h *TaskHandler) Reply(c *gin.Context) {
	taskID := c.Param("task_id")

	var req struct {
		Input json.RawMessage `json:"input" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user := authz.GetCurrentUser(c)
	resp, err := h.gatewaySvc.ReplyToTask(taskID, req.Input, user.ID)
	if err != nil {
		if errors.Is(err, gateway.ErrPermissionDenied) {
			response.Forbidden(c, err.Error())
			return
		}
		response.InternalServerError(c, err.Error())
		return
	}

	if resp.Status == "input_required" {
		c.JSON(202, gin.H{"code": 0, "message": "input required", "data": resp})
		return
	}

	response.Success(c, resp)
}

// Cancel 取消任务 POST /api/v1/tasks/:task_id/cancel
func (h *TaskHandler) Cancel(c *gin.Context) {
	taskID := c.Param("task_id")
	user := authz.GetCurrentUser(c)

	err := h.gatewaySvc.CancelTask(taskID, user.ID)
	if err != nil {
		if errors.Is(err, gateway.ErrPermissionDenied) {
			response.Forbidden(c, err.Error())
			return
		}
		response.InternalServerError(c, err.Error())
		return
	}

	response.Success(c, gin.H{"task_id": taskID, "status": "cancelled"})
}
