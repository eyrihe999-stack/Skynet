package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/skynetplatform/skynet/internal/authz"
	"github.com/skynetplatform/skynet/internal/registry"
	"github.com/skynetplatform/skynet/internal/store"
	"github.com/skynetplatform/skynet/pkg/response"
)

// RegistryHandler 是 Agent 注册中心的 HTTP 处理器。
// 它封装了 Agent 列表查询、详情获取、删除、心跳上报、能力搜索等端点的处理逻辑，
// 作为 API 层与 Registry 服务之间的桥梁。
//
// 字段说明：
//   - registrySvc: Registry 注册中心服务实例，提供 Agent 和能力的增删改查功能。
type RegistryHandler struct {
	registrySvc *registry.Service
}

// NewRegistryHandler 创建并返回一个新的 RegistryHandler 实例。
//
// 参数：
//   - registrySvc: Registry 注册中心服务，处理器将通过它访问 Agent 和能力数据。
//
// 返回值：
//   - *RegistryHandler: 初始化完成的注册中心处理器实例。
func NewRegistryHandler(registrySvc *registry.Service) *RegistryHandler {
	return &RegistryHandler{registrySvc: registrySvc}
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
