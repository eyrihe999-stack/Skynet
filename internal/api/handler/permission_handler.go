package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/eyrihe999-stack/Skynet/internal/authz"
	"github.com/eyrihe999-stack/Skynet/internal/model"
	"github.com/eyrihe999-stack/Skynet/internal/store"
	"github.com/eyrihe999-stack/Skynet/pkg/response"
)

// createRuleRequest 是创建权限规则接口的请求体结构，用于绑定和校验 JSON 请求参数。
type createRuleRequest struct {
	SkillName       *string `json:"skill_name"`
	CallerType      string  `json:"caller_type" binding:"required"`
	CallerID        *string `json:"caller_id"`
	Action          string  `json:"action" binding:"required"`
	ApprovalMode    string  `json:"approval_mode"`
	RateLimitMax    *uint   `json:"rate_limit_max"`
	RateLimitWindow *string `json:"rate_limit_window"`
	Priority        int     `json:"priority"`
}

// PermissionHandler 是权限规则管理的 HTTP 处理器。
// 它封装了权限规则的列表查询、创建和删除等端点的处理逻辑，
// 作为 API 层与 PermissionRepo 数据仓库之间的桥梁。
//
// 字段说明：
//   - permRepo: 权限规则数据仓库实例，提供权限规则的增删查功能。
//   - agentRepo: Agent 数据仓库实例，用于校验 Agent 是否存在及所有者验证。
type PermissionHandler struct {
	permRepo  *store.PermissionRepo
	agentRepo *store.AgentRepo
}

// NewPermissionHandler 创建并返回一个新的 PermissionHandler 实例。
//
// 参数：
//   - permRepo: 权限规则数据仓库，处理器将通过它管理权限规则数据。
//   - agentRepo: Agent 数据仓库，处理器将通过它验证 Agent 存在性和所有权。
//
// 返回值：
//   - *PermissionHandler: 初始化完成的权限规则处理器实例。
func NewPermissionHandler(permRepo *store.PermissionRepo, agentRepo *store.AgentRepo) *PermissionHandler {
	return &PermissionHandler{permRepo: permRepo, agentRepo: agentRepo}
}

// ListRules 处理获取指定 Agent 的权限规则列表请求（GET /api/v1/agents/:agent_id/rules）。
//
// 处理流程：
//  1. 从 URL 路径参数获取 agent_id。
//  2. 查询 Agent 是否存在。
//  3. 查询该 Agent 的所有权限规则并返回。
//
// 参数：
//   - c: Gin 请求上下文，包含路径参数 agent_id。
//
// 响应：
//   - 200: 查询成功，返回权限规则列表。
//   - 404: 指定 agent_id 对应的 Agent 不存在。
//   - 500: 查询过程中发生内部错误。
func (h *PermissionHandler) ListRules(c *gin.Context) {
	agentID := c.Param("agent_id")

	// 验证 Agent 是否存在
	_, err := h.agentRepo.FindByAgentID(agentID)
	if err != nil {
		response.NotFound(c, "agent not found")
		return
	}

	rules, err := h.permRepo.FindByAgent(agentID)
	if err != nil {
		response.InternalServerError(c, "failed to list permission rules")
		return
	}

	response.Success(c, rules)
}

// CreateRule 处理创建权限规则的请求（POST /api/v1/agents/:agent_id/rules）。
//
// 处理流程：
//  1. 从 URL 路径参数获取 agent_id。
//  2. 查询 Agent 是否存在。
//  3. 校验当前用户是否为该 Agent 的所有者（只有所有者才能创建规则）。
//  4. 绑定请求体并创建权限规则。
//
// 参数：
//   - c: Gin 请求上下文，包含路径参数 agent_id 和 JSON 请求体。
//
// 响应：
//   - 201: 创建成功，返回新创建的权限规则。
//   - 400: 请求参数校验失败。
//   - 403: 当前用户不是该 Agent 的所有者，无权创建规则。
//   - 404: 指定 agent_id 对应的 Agent 不存在。
//   - 500: 创建过程中发生内部错误。
func (h *PermissionHandler) CreateRule(c *gin.Context) {
	agentID := c.Param("agent_id")

	// 验证 Agent 是否存在
	agent, err := h.agentRepo.FindByAgentID(agentID)
	if err != nil {
		response.NotFound(c, "agent not found")
		return
	}

	// 校验当前用户是否为 Agent 所有者
	user := authz.GetCurrentUser(c)
	if user == nil || user.ID != agent.OwnerID {
		response.Forbidden(c, "not the owner of this agent")
		return
	}

	var req createRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	rule := &model.PermissionRule{
		AgentID:         agentID,
		SkillName:       req.SkillName,
		CallerType:      req.CallerType,
		CallerID:        req.CallerID,
		Action:          req.Action,
		ApprovalMode:    req.ApprovalMode,
		RateLimitMax:    req.RateLimitMax,
		RateLimitWindow: req.RateLimitWindow,
		Priority:        req.Priority,
	}

	if err := h.permRepo.Create(rule); err != nil {
		response.InternalServerError(c, "failed to create permission rule")
		return
	}

	response.Created(c, rule)
}

// DeleteRule 处理删除权限规则的请求（DELETE /api/v1/agents/:agent_id/rules/:rule_id）。
//
// 处理流程：
//  1. 从 URL 路径参数获取 agent_id 和 rule_id。
//  2. 查询 Agent 是否存在。
//  3. 校验当前用户是否为该 Agent 的所有者（只有所有者才能删除规则）。
//  4. 查询规则是否存在且属于该 Agent。
//  5. 删除权限规则。
//
// 参数：
//   - c: Gin 请求上下文，包含路径参数 agent_id 和 rule_id。
//
// 响应：
//   - 200: 删除成功，返回被删除的规则 ID 和状态。
//   - 403: 当前用户不是该 Agent 的所有者，无权删除规则。
//   - 404: 指定 agent_id 对应的 Agent 不存在，或规则不存在/不属于该 Agent。
//   - 500: 删除过程中发生内部错误。
func (h *PermissionHandler) DeleteRule(c *gin.Context) {
	agentID := c.Param("agent_id")

	// 验证 Agent 是否存在
	agent, err := h.agentRepo.FindByAgentID(agentID)
	if err != nil {
		response.NotFound(c, "agent not found")
		return
	}

	// 校验当前用户是否为 Agent 所有者
	user := authz.GetCurrentUser(c)
	if user == nil || user.ID != agent.OwnerID {
		response.Forbidden(c, "not the owner of this agent")
		return
	}

	// 解析 rule_id
	ruleID, err := strconv.ParseUint(c.Param("rule_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid rule_id")
		return
	}

	// 查询规则是否存在且属于该 Agent
	rule, err := h.permRepo.FindByID(ruleID)
	if err != nil || rule.AgentID != agentID {
		response.NotFound(c, "permission rule not found")
		return
	}

	if err := h.permRepo.Delete(ruleID); err != nil {
		response.InternalServerError(c, "failed to delete permission rule")
		return
	}

	response.Success(c, gin.H{"rule_id": ruleID, "status": "deleted"})
}
