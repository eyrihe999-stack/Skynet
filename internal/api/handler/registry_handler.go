package handler

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/eyrihe999-stack/Skynet/internal/authz"
	"github.com/eyrihe999-stack/Skynet/internal/gateway"
	"github.com/eyrihe999-stack/Skynet/internal/registry"
	"github.com/eyrihe999-stack/Skynet/internal/store"
	"github.com/eyrihe999-stack/Skynet/pkg/response"
	"github.com/eyrihe999-stack/Skynet-sdk/protocol"
)

// RegistryHandler 是 Agent 注册中心的 HTTP 处理器。
type RegistryHandler struct {
	registrySvc *registry.Service
	connMgr     *gateway.ConnectionManager
	callbackMgr *gateway.CallbackManager
	eventBus    *gateway.EventBus
}

// NewRegistryHandler 创建并返回一个新的 RegistryHandler 实例。
func NewRegistryHandler(registrySvc *registry.Service, connMgr *gateway.ConnectionManager, callbackMgr *gateway.CallbackManager, eventBus *gateway.EventBus) *RegistryHandler {
	return &RegistryHandler{
		registrySvc: registrySvc,
		connMgr:     connMgr,
		callbackMgr: callbackMgr,
		eventBus:    eventBus,
	}
}

// GetAgent 处理获取单个 Agent 详情的请求（GET /api/v1/agents/:agent_id）。
//
// 通过 URL 路径参数 agent_id 查询 Registry 中对应的 Agent 信息并返回。
//
// 参数：
//   - c: Gin 请求上下文，包含路径参数 agent_id。
//
// 响应：
//   - 200: 查询成功，返回 Agent 详情。
//   - 404: 指定 agent_id 对应的 Agent 不存在。
func (h *RegistryHandler) GetAgent(c *gin.Context) {
	agentID := c.Param("agent_id")
	agent, err := h.registrySvc.GetAgent(agentID)
	if err != nil {
		response.NotFound(c, "agent not found")
		return
	}
	response.Success(c, agent)
}

// ListAgents 处理获取 Agent 列表的请求（GET /api/v1/agents）。
//
// 支持分页和过滤功能：
//   - page: 页码，默认 1。
//   - page_size: 每页数量，默认 20。
//   - status: 按 Agent 状态过滤（如 "online"、"offline"）。
//   - mine: 设置为 "true" 时仅返回当前用户拥有的 Agent。
//
// 参数：
//   - c: Gin 请求上下文，包含查询参数 page、page_size、status、mine。
//
// 响应：
//   - 200: 查询成功，返回分页的 Agent 列表及总数。
//   - 500: 查询过程中发生内部错误。
func (h *RegistryHandler) ListAgents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")

	filter := store.AgentFilter{
		Status:   status,
		Page:     page,
		PageSize: pageSize,
	}

	// 若请求参数 mine=true，则仅查询当前认证用户拥有的 Agent
	if c.Query("mine") == "true" {
		user := authz.GetCurrentUser(c)
		if user != nil {
			filter.OwnerID = &user.ID
		}
	}

	agents, total, err := h.registrySvc.ListAgents(filter)
	if err != nil {
		response.InternalServerError(c, "failed to list agents")
		return
	}

	response.Paginated(c, agents, total, page, pageSize)
}

// DeleteAgent 处理删除指定 Agent 的请求（DELETE /api/v1/agents/:agent_id）。
//
// 处理流程：
//  1. 根据 agent_id 查询目标 Agent 是否存在。
//  2. 校验当前用户是否为该 Agent 的所有者（只有所有者才能删除）。
//  3. 调用 Registry 服务注销该 Agent。
//
// 参数：
//   - c: Gin 请求上下文，包含路径参数 agent_id 和认证用户信息。
//
// 响应：
//   - 200: 删除成功，返回被删除的 agent_id 和状态。
//   - 403: 当前用户不是该 Agent 的所有者，无权删除。
//   - 404: 指定 agent_id 对应的 Agent 不存在。
//   - 500: 删除过程中发生内部错误。
func (h *RegistryHandler) DeleteAgent(c *gin.Context) {
	agentID := c.Param("agent_id")
	user := authz.GetCurrentUser(c)

	// 验证 Agent 是否存在
	agent, err := h.registrySvc.GetAgent(agentID)
	if err != nil {
		response.NotFound(c, "agent not found")
		return
	}

	// 验证当前用户是否为 Agent 所有者
	if agent.OwnerID != user.ID {
		response.Forbidden(c, "not the owner of this agent")
		return
	}

	if err := h.registrySvc.UnregisterAgent(agentID); err != nil {
		response.InternalServerError(c, "failed to delete agent")
		return
	}

	response.Success(c, gin.H{"agent_id": agentID, "status": "removed"})
}

// Heartbeat 处理 Agent 心跳上报请求（POST /api/v1/agents/:agent_id/heartbeat）。
//
// Agent 定期调用此端点上报存活状态，Registry 服务据此更新 Agent 的最后活跃时间，
// 用于判定 Agent 在线/离线状态。
//
// 参数：
//   - c: Gin 请求上下文，包含路径参数 agent_id。
//
// 响应：
//   - 200: 心跳上报成功。
//   - 500: 心跳处理过程中发生内部错误（如 Agent 不存在或数据库写入失败）。
func (h *RegistryHandler) Heartbeat(c *gin.Context) {
	agentID := c.Param("agent_id")
	if err := h.registrySvc.Heartbeat(agentID); err != nil {
		response.InternalServerError(c, "heartbeat failed")
		return
	}
	response.Success(c, gin.H{"agent_id": agentID, "status": "ok"})
}

// SearchCapabilities 处理能力搜索请求（GET /api/v1/capabilities）。
//
// 支持按关键字和类别搜索平台中所有已注册 Agent 提供的能力（Capability），
// 并以分页方式返回结果。
//
// 查询参数：
//   - q: 搜索关键字，匹配能力名称或描述。
//   - category: 按能力类别过滤。
//   - page: 页码，默认 1。
//   - page_size: 每页数量，默认 20。
//
// 参数：
//   - c: Gin 请求上下文，包含查询参数 q、category、page、page_size。
//
// 响应：
//   - 200: 搜索成功，返回分页的能力列表及总数。
//   - 500: 搜索过程中发生内部错误。
func (h *RegistryHandler) SearchCapabilities(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	filter := store.CapabilityFilter{
		Query:    c.Query("q"),
		Category: c.Query("category"),
		Page:     page,
		PageSize: pageSize,
	}

	caps, total, err := h.registrySvc.SearchCapabilities(filter)
	if err != nil {
		response.InternalServerError(c, "search failed")
		return
	}

	response.Paginated(c, caps, total, page, pageSize)
}

// registerAgentRequest 是 Webhook/Direct 模式 Agent 通过 REST API 注册的请求体。
type registerAgentRequest struct {
	AgentID      string              `json:"agent_id" binding:"required"`
	DisplayName  string              `json:"display_name" binding:"required"`
	Description  string              `json:"description"`
	Version      string              `json:"version"`
	EndpointURL  string              `json:"endpoint_url" binding:"required"`
	Capabilities []registerCapDef    `json:"capabilities"`
}

type registerCapDef struct {
	Name               string          `json:"name" binding:"required"`
	DisplayName        string          `json:"display_name"`
	Description        string          `json:"description"`
	Category           string          `json:"category"`
	Tags               []string        `json:"tags"`
	InputSchema        json.RawMessage `json:"input_schema"`
	OutputSchema       json.RawMessage `json:"output_schema"`
	Visibility         string          `json:"visibility"`
	ApprovalMode       string          `json:"approval_mode"`
	EstimatedLatencyMs uint            `json:"estimated_latency_ms"`
}

// RegisterAgent 处理 Webhook/Direct 模式的 Agent 注册（POST /api/v1/agents/register）。
//
// 与 WebSocket Tunnel 模式不同，Webhook Agent 通过此 REST 端点注册，
// 并提供 endpoint_url 供 Platform 主动回调。Agent 无需维持长连接。
//
// 响应：
//   - 200: 注册成功，返回 agent_secret（仅首次注册返回，妥善保管）
//   - 400: 请求参数不合法
//   - 500: 注册失败
func (h *RegistryHandler) RegisterAgent(c *gin.Context) {
	var req registerAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user := authz.GetCurrentUser(c)

	// 为每个 capability 设置默认值
	caps := make([]protocol.CapabilityDef, len(req.Capabilities))
	for i, cap := range req.Capabilities {
		visibility := cap.Visibility
		if visibility == "" {
			visibility = "public"
		}
		approvalMode := cap.ApprovalMode
		if approvalMode == "" {
			approvalMode = "auto"
		}
		caps[i] = protocol.CapabilityDef{
			Name:               cap.Name,
			DisplayName:        cap.DisplayName,
			Description:        cap.Description,
			Category:           cap.Category,
			Tags:               cap.Tags,
			InputSchema:        cap.InputSchema,
			OutputSchema:       cap.OutputSchema,
			Visibility:         visibility,
			ApprovalMode:       approvalMode,
			EstimatedLatencyMs: cap.EstimatedLatencyMs,
		}
	}

	card := protocol.AgentCard{
		AgentID:        req.AgentID,
		DisplayName:    req.DisplayName,
		Description:    req.Description,
		Version:        req.Version,
		ConnectionMode: "direct",
		Capabilities:   caps,
	}

	agentSecret, err := h.registrySvc.RegisterAgent(card, user.ID, req.EndpointURL)
	if err != nil {
		response.InternalServerError(c, "registration failed: "+err.Error())
		return
	}

	// 获取用于 HMAC 签名的 secret（重复注册时 agentSecret 为空，需从 DB 获取）
	signingSecret := agentSecret
	if signingSecret == "" {
		signingSecret, _ = h.registrySvc.GetAgentSecret(req.AgentID)
	}

	// 创建 WebhookTransport 并注册到 ConnectionManager
	transport := gateway.NewWebhookTransport(req.AgentID, req.EndpointURL, signingSecret, h.callbackMgr)
	h.connMgr.Register(req.AgentID, transport)

	// 发布上线事件
	h.eventBus.PublishJSON("agent_online", map[string]string{"agent_id": req.AgentID})

	resp := gin.H{"agent_id": req.AgentID, "status": "registered"}
	if agentSecret != "" {
		resp["agent_secret"] = agentSecret
	}

	response.Success(c, resp)
}
