package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/eyrihe999-stack/Skynet/internal/authz"
	"github.com/eyrihe999-stack/Skynet/internal/store"
	"github.com/eyrihe999-stack/Skynet/pkg/response"
)

type ApprovalHandler struct {
	approvalRepo *store.ApprovalRepo
}

func NewApprovalHandler(approvalRepo *store.ApprovalRepo) *ApprovalHandler {
	return &ApprovalHandler{approvalRepo: approvalRepo}
}

// ListPending 查询当前用户的待审批列表
// GET /api/v1/approvals?page=1&page_size=20
func (h *ApprovalHandler) ListPending(c *gin.Context) {
	user := authz.GetCurrentUser(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	approvals, total, err := h.approvalRepo.ListPending(user.ID, page, pageSize)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}

	response.Paginated(c, approvals, total, page, pageSize)
}

// Decide 处理审批决定
// POST /api/v1/approvals/:id
// body: {"action": "approve"} 或 {"action": "deny"}
func (h *ApprovalHandler) Decide(c *gin.Context) {
	user := authz.GetCurrentUser(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid approval ID")
		return
	}

	var req struct {
		Action string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.Action != "approve" && req.Action != "deny" {
		response.BadRequest(c, "action must be 'approve' or 'deny'")
		return
	}

	approval, err := h.approvalRepo.FindByID(id)
	if err != nil {
		response.NotFound(c, "approval not found")
		return
	}

	// 只有 owner 可以审批
	if approval.OwnerID != user.ID {
		response.Forbidden(c, "not the owner of this approval")
		return
	}

	if approval.Status != "pending" {
		response.BadRequest(c, "approval already decided")
		return
	}

	status := "approved"
	if req.Action == "deny" {
		status = "denied"
	}

	if err := h.approvalRepo.Decide(id, status); err != nil {
		response.InternalServerError(c, err.Error())
		return
	}

	response.Success(c, gin.H{"id": id, "status": status})
}
