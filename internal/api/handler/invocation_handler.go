package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/eyrihe999-stack/Skynet/internal/authz"
	"github.com/eyrihe999-stack/Skynet/internal/store"
	"github.com/eyrihe999-stack/Skynet/pkg/response"
)

// InvocationHandler 是调用历史记录的 HTTP 处理器。
// 它封装了调用记录查询相关端点的处理逻辑，
// 作为 API 层与 InvocationRepo 数据仓库之间的桥梁。
//
// 字段说明：
//   - invRepo: 调用记录数据仓库实例，提供调用记录的查询功能。
type InvocationHandler struct {
	invRepo *store.InvocationRepo
}

// NewInvocationHandler 创建并返回一个新的 InvocationHandler 实例。
//
// 参数：
//   - invRepo: 调用记录数据仓库，处理器将通过它访问调用历史数据。
//
// 返回值：
//   - *InvocationHandler: 初始化完成的调用记录处理器实例。
func NewInvocationHandler(invRepo *store.InvocationRepo) *InvocationHandler {
	return &InvocationHandler{invRepo: invRepo}
}

// ListInvocations 处理获取调用历史记录列表的请求（GET /api/v1/invocations）。
//
// 支持分页和过滤功能：
//   - caller_agent_id: 按调用方 Agent ID 过滤（可选）。
//   - target_agent_id: 按被调用方 Agent ID 过滤（可选）。
//   - caller_user_id: 按发起调用的用户 ID 过滤（可选）。
//   - page: 页码，默认 1。
//   - page_size: 每页数量，默认 20。
//
// 参数：
//   - c: Gin 请求上下文，包含查询参数。
//
// 响应：
//   - 200: 查询成功，返回分页的调用记录列表及总数。
//   - 500: 查询过程中发生内部错误。
func (h *InvocationHandler) ListInvocations(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	filter := store.InvocationFilter{
		CallerAgentID: c.Query("caller_agent_id"),
		TargetAgentID: c.Query("target_agent_id"),
		Status:        c.Query("status"),
		Page:          page,
		PageSize:      pageSize,
	}

	if v := c.Query("caller_user_id"); v != "" {
		uid, err := strconv.ParseUint(v, 10, 64)
		if err == nil {
			filter.CallerUserID = &uid
		}
	}

	// mine=true: 仅查询当前用户发起的调用
	if c.Query("mine") == "true" {
		user := authz.GetCurrentUser(c)
		if user != nil {
			filter.CallerUserID = &user.ID
		}
	}

	invocations, total, err := h.invRepo.List(filter)
	if err != nil {
		response.InternalServerError(c, "failed to list invocations")
		return
	}

	response.Paginated(c, invocations, total, page, pageSize)
}
