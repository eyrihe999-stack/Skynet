// Package handler 实现 Skynet 平台 API 层的各 HTTP 请求处理器。
// 该包包含 Agent 注册中心管理、技能调用、WebSocket 隧道等核心端点的处理逻辑，
// 是 API 路由（router.go）与内部服务（Registry、Gateway、AuthZ）之间的桥梁。
package handler

import (
	"encoding/json"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/skynetplatform/skynet/internal/authz"
	"github.com/skynetplatform/skynet/internal/gateway"
	"github.com/skynetplatform/skynet-sdk/protocol"
	"github.com/skynetplatform/skynet/pkg/response"
)

// InvokeHandler 是技能调用的 HTTP 处理器。
// 它接收来自 CLI、Dashboard 或其他 Agent 的调用请求，
// 构建调用上下文后转发给 Gateway 服务执行，再将结果返回给调用方。
//
// 字段说明：
//   - gatewaySvc: Gateway 网关服务实例，负责通过 WebSocket 隧道将请求路由到目标 Agent。
type InvokeHandler struct {
	gatewaySvc *gateway.Service
}

// NewInvokeHandler 创建并返回一个新的 InvokeHandler 实例。
//
// 参数：
//   - gatewaySvc: Gateway 网关服务，用于实际执行对目标 Agent 的技能调用。
//
// 返回值：
//   - *InvokeHandler: 初始化完成的调用处理器实例。
func NewInvokeHandler(gatewaySvc *gateway.Service) *InvokeHandler {
	return &InvokeHandler{gatewaySvc: gatewaySvc}
}

// invokeRequest 定义了技能调用 API 的请求体结构。
//
// 字段说明：
//   - TargetAgent: 目标 Agent 的唯一标识符，指定将技能调用路由到哪个 Agent（必填）。
//   - Skill: 要调用的技能名称（必填）。
//   - Input: 技能调用的输入参数，以 JSON 原始格式传递，保持灵活性，不同技能可接收不同结构。
//   - TimeoutMs: 调用超时时间（毫秒），为 0 时使用 Gateway 的默认超时。
type invokeRequest struct {
	TargetAgent string          `json:"target_agent" binding:"required"`
	Skill       string          `json:"skill" binding:"required"`
	Input       json.RawMessage `json:"input"`
	TimeoutMs   int             `json:"timeout_ms"`
	CallChain   []string        `json:"call_chain"`
}

// Invoke 处理技能调用请求（POST /api/v1/invoke）。
// 该方法是 Skynet 平台技能调用的入口端点。
//
// 处理流程：
//  1. 解析并校验请求体中的 JSON 参数（目标 Agent、技能名等）。
//  2. 从请求上下文中获取当前认证用户信息，构建调用者信息（CallerInfo）。
//  3. 将请求转发给 Gateway 服务，由 Gateway 通过 WebSocket 隧道路由到目标 Agent。
//  4. 等待 Agent 返回结果后，将结果以标准格式响应给调用方。
//
// 参数：
//   - c: Gin 请求上下文，包含 HTTP 请求信息和认证用户数据。
//
// 响应：
//   - 200: 调用成功，返回 Agent 执行结果。
//   - 400: 请求参数校验失败（如缺少必填字段）。
//   - 500: Gateway 调用过程中发生内部错误。
func (h *InvokeHandler) Invoke(c *gin.Context) {
	var req invokeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 获取当前认证用户信息，用于构建调用者上下文
	user := authz.GetCurrentUser(c)
	caller := protocol.CallerInfo{
		DisplayName: user.DisplayName,
		UserID:      user.ID,
	}

	// 构建 Gateway 调用请求并执行
	result, err := h.gatewaySvc.Invoke(gateway.InvokeRequest{
		TargetAgent: req.TargetAgent,
		Skill:       req.Skill,
		Input:       req.Input,
		TimeoutMs:   req.TimeoutMs,
		Caller:      caller,
		CallChain:   req.CallChain,
	})
	if err != nil {
		if errors.Is(err, gateway.ErrApprovalRequired) && result != nil {
			c.JSON(202, gin.H{"code": 0, "message": "approval required", "data": result})
			return
		}
		if errors.Is(err, gateway.ErrPermissionDenied) {
			response.Forbidden(c, err.Error())
			return
		}
		if errors.Is(err, gateway.ErrCallChainTooDeep) || errors.Is(err, gateway.ErrCallChainLoop) {
			response.BadRequest(c, err.Error())
			return
		}
		if errors.Is(err, gateway.ErrRateLimited) {
			// 429 Too Many Requests
			c.JSON(429, gin.H{"code": 429, "message": err.Error()})
			return
		}
		response.InternalServerError(c, err.Error())
		return
	}

	if result.Status == "input_required" {
		c.JSON(202, gin.H{"code": 0, "message": "input required", "data": result})
		return
	}

	response.Success(c, result)
}
