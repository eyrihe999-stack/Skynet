// Package gateway 实现了 Skynet 平台的调用网关服务。
//
// Gateway 是平台的调用核心，负责接收技能调用请求，通过 WebSocket 反向通道
// 将请求转发到目标 Agent，并等待结果返回。它是连接 HTTP API 层和 Agent 运行时的桥梁。
//
// 本包包含三个核心组件：
//   - Service：调用网关服务，处理调用请求的完整生命周期
//   - AgentConn：单个 Agent 的 WebSocket 连接封装
//   - TunnelManager：所有在线 Agent 的连接映射管理器
package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/eyrihe999-stack/Skynet/internal/model"
	"github.com/eyrihe999-stack/Skynet/internal/store"
	"github.com/eyrihe999-stack/Skynet-sdk/logger"
	"github.com/eyrihe999-stack/Skynet-sdk/protocol"
)

// ErrCallChainTooDeep 表示调用链深度超过最大限制。
var ErrCallChainTooDeep = errors.New("call chain too deep")

// ErrCallChainLoop 表示调用链中检测到环路。
var ErrCallChainLoop = errors.New("call chain loop detected")

// ErrRateLimited 表示调用频率超过限制。
var ErrRateLimited = errors.New("rate limited")

// MaxCallChainDepth 是调用链允许的最大深度。
var MaxCallChainDepth = 3

// ErrPermissionDenied 表示调用方没有权限调用目标 Skill。
// 当 Skill 的 visibility 为 private（仅 Owner 可用）或 restricted（仅白名单可用）
// 而调用方不满足条件时返回此错误。
var ErrPermissionDenied = errors.New("permission denied")

// ErrApprovalRequired 表示目标 Skill 需要人工审批后才能执行。
// 当 Skill 的 approval_mode 为 "manual" 时，网关不会立即执行调用，
// 而是创建一条待审批记录，等待 Owner 审批通过后再执行。
var ErrApprovalRequired = errors.New("approval required")

// Service 是调用网关的核心服务，负责处理技能调用请求的完整生命周期。
//
// 当用户或 Agent 发起技能调用时，Service 首先校验调用方对目标 Skill 的访问权限，
// 然后通过 TunnelManager 查找目标 Agent 的 WebSocket 连接，将调用请求转发到目标 Agent，
// 等待执行结果，并记录调用日志和统计数据。
//
// 字段说明：
//   - tunnelMgr：隧道连接管理器，用于查找目标 Agent 的 WebSocket 连接
//   - invRepo：调用记录仓库，用于持久化调用日志（创建、更新状态）
//   - capRepo：能力（技能）仓库，用于查询 Skill 元信息和更新调用统计
//   - agentRepo：Agent 仓库，用于查询 Agent 的 Owner 信息（权限校验需要）
type Service struct {
	tunnelMgr    *TunnelManager
	invRepo      *store.InvocationRepo
	capRepo      *store.CapabilityRepo
	agentRepo    *store.AgentRepo
	permRepo     *store.PermissionRepo
	rateLimiter  *RateLimiter
	approvalRepo *store.ApprovalRepo
	taskSessions *TaskSessionManager
	taskMsgRepo  *store.TaskMessageRepo
}

// NewService 创建并返回一个新的调用网关服务实例。
//
// 参数：
//   - tunnelMgr：隧道连接管理器，提供 Agent WebSocket 连接的查找能力
//   - invRepo：调用记录仓库，用于持久化调用日志
//   - capRepo：能力仓库，用于查询 Skill 信息和更新调用统计
//   - agentRepo：Agent 仓库，用于查询 Agent 所有者信息
//
// 返回值：
//   - *Service：初始化完成的网关服务实例
func NewService(tunnelMgr *TunnelManager, invRepo *store.InvocationRepo, capRepo *store.CapabilityRepo, agentRepo *store.AgentRepo, permRepo *store.PermissionRepo, rateLimiter *RateLimiter, approvalRepo *store.ApprovalRepo, taskSessions *TaskSessionManager, taskMsgRepo *store.TaskMessageRepo) *Service {
	return &Service{
		tunnelMgr:    tunnelMgr,
		invRepo:      invRepo,
		capRepo:      capRepo,
		agentRepo:    agentRepo,
		permRepo:     permRepo,
		rateLimiter:  rateLimiter,
		approvalRepo: approvalRepo,
		taskSessions: taskSessions,
		taskMsgRepo:  taskMsgRepo,
	}
}

// InvokeRequest 表示一次技能调用请求的完整参数。
//
// 由 HTTP API 层构造后传递给 Service.Invoke 方法处理。
//
// 字段说明：
//   - TargetAgent：目标 Agent 的唯一标识符，指定调用哪个 Agent
//   - Skill：要调用的技能名称
//   - Input：技能输入参数，以原始 JSON 字节形式传递
//   - TimeoutMs：调用超时时间（毫秒），若 <= 0 则使用默认值 30 秒
//   - Caller：调用方信息，包含发起调用的 Agent ID 和用户 ID
type InvokeRequest struct {
	TargetAgent string
	Skill       string
	Input       []byte // raw JSON
	TimeoutMs   int
	Caller      protocol.CallerInfo
	CallChain   []string
}

// InvokeResponse 表示技能调用的响应结果，返回给 HTTP API 层。
//
// 字段说明：
//   - TaskID：本次调用的唯一任务标识符，可用于后续查询调用记录
//   - Status：调用结果状态，如 "completed"（成功）或 "failed"（失败）
//   - Output：调用成功时的输出结果，失败时为空
//   - Error：调用失败时的错误信息，成功时为空
type InvokeResponse struct {
	TaskID   string             `json:"task_id"`
	Status   string             `json:"status"`
	Output   any                `json:"output,omitempty"`
	Error    string             `json:"error,omitempty"`
	Question *protocol.Question `json:"question,omitempty"`
}

// Invoke 执行一次同步技能调用，是网关服务的核心方法。
//
// 调用流程：
//  1. 通过 TunnelManager 检查目标 Agent 是否在线
//  2. 生成唯一任务 ID 并创建调用记录
//  3. 通过 Agent 的 WebSocket 连接发送 invoke 消息
//  4. 等待 Agent 返回执行结果（或超时）
//  5. 更新调用记录状态和技能调用统计
//
// 参数：
//   - req：调用请求参数，包含目标 Agent、技能名称、输入数据、超时设置和调用方信息
//
// 返回值：
//   - *InvokeResponse：调用响应，包含任务 ID、状态、输出结果或错误信息
//   - error：当目标 Agent 不在线、消息发送失败或调用超时时返回错误
func (s *Service) Invoke(req InvokeRequest) (*InvokeResponse, error) {
	// 权限校验：检查调用方是否有权访问目标 Skill
	if err := s.checkPermission(req); err != nil {
		return nil, err
	}

	// Call chain 风控
	if err := s.checkCallChain(req); err != nil {
		return nil, err
	}

	// 限流检查
	if err := s.checkRateLimit(req); err != nil {
		return nil, err
	}

	// 审批检查：manual 模式创建审批记录而不是立即执行
	needsApproval, _ := s.checkApproval(req)
	if needsApproval {
		return s.createPendingApproval(req)
	}

	// 检查目标 Agent 是否在线
	conn := s.tunnelMgr.GetConn(req.TargetAgent)
	if conn == nil {
		return nil, fmt.Errorf("agent '%s' is not online", req.TargetAgent)
	}

	taskID := uuid.New().String()

	// 创建调用记录，初始状态为 "submitted"
	inv := &model.Invocation{
		TaskID:        taskID,
		CallerAgentID: req.Caller.AgentID,
		TargetAgentID: req.TargetAgent,
		SkillName:     req.Skill,
		Status:        "submitted",
		Mode:          "sync",
	}
	if req.Caller.UserID > 0 {
		inv.CallerUserID = &req.Caller.UserID
	}
	s.invRepo.Create(inv)

	// 确定超时时间，若未指定或非法则使用默认值 30 秒
	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// 构造调用载荷，通过 WebSocket 隧道发送 invoke 消息
	payload := protocol.InvokePayload{
		Skill:     req.Skill,
		Input:     req.Input,
		Caller:    req.Caller,
		TimeoutMs: req.TimeoutMs,
		CallChain: req.CallChain,
	}

	startTime := time.Now()
	requestID, invokeResult, err := conn.SendInvoke(payload, timeout)
	latencyMs := uint(time.Since(startTime).Milliseconds())

	if err != nil {
		s.invRepo.UpdateStatus(taskID, "failed", &latencyMs, err.Error())
		s.capRepo.IncrementCallCount(req.TargetAgent, req.Skill, latencyMs, false)
		return nil, err
	}

	// 多轮对话：Agent 返回 need_input
	if invokeResult.Type == "need_input" {
		s.invRepo.UpdateStatus(taskID, "input_required", nil, "")
		s.taskMsgRepo.Create(&model.TaskMessage{
			TaskID: taskID, Direction: "from_agent", MessageType: "question",
		})
		session := &TaskSession{
			TaskID:        taskID,
			TargetAgent:   req.TargetAgent,
			RequestID:     requestID,
			Skill:         req.Skill,
			Caller:        req.Caller,
			Status:        "input_required",
			Question:      &invokeResult.NeedInput.Question,
			StartTime:     startTime,
			TimeoutMs:     req.TimeoutMs,
			OriginalInput: req.Input,
		}
		s.taskSessions.Store(session)
		logger.Infof("Invoke %s.%s needs input (task=%s)", req.TargetAgent, req.Skill, taskID)
		return &InvokeResponse{
			TaskID:   taskID,
			Status:   "input_required",
			Question: &invokeResult.NeedInput.Question,
		}, nil
	}

	// 单轮完成：原有逻辑
	result := invokeResult.Result
	if result.Status == "completed" {
		s.invRepo.UpdateStatus(taskID, "completed", &latencyMs, "")
		s.capRepo.IncrementCallCount(req.TargetAgent, req.Skill, latencyMs, true)
	} else {
		s.invRepo.UpdateStatus(taskID, "failed", &latencyMs, result.Error)
		s.capRepo.IncrementCallCount(req.TargetAgent, req.Skill, latencyMs, false)
	}

	logger.Infof("Invoke %s.%s completed in %dms (status=%s)", req.TargetAgent, req.Skill, latencyMs, result.Status)

	return &InvokeResponse{
		TaskID: taskID,
		Status: result.Status,
		Output: result.Output,
		Error:  result.Error,
	}, nil
}

// checkPermission 校验调用方是否有权限访问目标 Skill。
//
// 权限规则基于 Skill 的 visibility 字段：
//   - "public"：所有认证用户均可调用
//   - "private"：仅 Agent 的 Owner 可调用
//   - "restricted"：查询 permission_rules 白名单，匹配规则决定是否放行
//
// Owner 始终拥有所有权限，无需查询白名单。
//
// 参数：
//   - req：调用请求，包含目标 Agent、Skill 名称和调用方信息
//
// 返回值：
//   - error：权限不足时返回 ErrPermissionDenied，Skill 不存在时返回对应错误，
//     权限通过时返回 nil
func (s *Service) checkPermission(req InvokeRequest) error {
	cap, err := s.capRepo.FindByAgentAndName(req.TargetAgent, req.Skill)
	if err != nil {
		return fmt.Errorf("skill '%s' not found on agent '%s'", req.Skill, req.TargetAgent)
	}

	if cap.Visibility == "public" {
		return nil
	}

	agent, err := s.agentRepo.FindByAgentID(req.TargetAgent)
	if err != nil {
		return fmt.Errorf("agent '%s' not found", req.TargetAgent)
	}

	isOwner := req.Caller.UserID > 0 && req.Caller.UserID == agent.OwnerID
	if isOwner {
		return nil // Owner 始终有权限
	}

	if cap.Visibility == "private" {
		return fmt.Errorf("%w: skill '%s' is private", ErrPermissionDenied, req.Skill)
	}

	// restricted: 查询 permission_rules 白名单
	rules, _ := s.permRepo.FindRules(req.TargetAgent, req.Skill)
	for _, rule := range rules {
		if matchCaller(rule, req.Caller) {
			if rule.Action == "allow" {
				return nil
			}
			return fmt.Errorf("%w: denied by permission rule", ErrPermissionDenied)
		}
	}

	return fmt.Errorf("%w: skill '%s' is restricted", ErrPermissionDenied, req.Skill)
}

// matchCaller 检查权限规则是否匹配当前调用方。
func matchCaller(rule model.PermissionRule, caller protocol.CallerInfo) bool {
	switch rule.CallerType {
	case "any":
		return true
	case "user":
		if rule.CallerID != nil {
			return fmt.Sprintf("%d", caller.UserID) == *rule.CallerID
		}
		return caller.UserID > 0
	case "agent":
		if rule.CallerID != nil {
			return caller.AgentID == *rule.CallerID
		}
		return caller.AgentID != ""
	}
	return false
}

// checkCallChain 校验调用链深度和环路。
//
// 防止 Agent 间互调形成无限递归或过深的调用链。
//
// 参数：
//   - req：调用请求，包含当前调用链信息
//
// 返回值：
//   - error：深度超限返回 ErrCallChainTooDeep，环路返回 ErrCallChainLoop，通过时返回 nil
func (s *Service) checkCallChain(req InvokeRequest) error {
	if len(req.CallChain) >= MaxCallChainDepth {
		return fmt.Errorf("%w: depth %d exceeds max %d", ErrCallChainTooDeep, len(req.CallChain), MaxCallChainDepth)
	}
	// 环路检测
	for _, id := range req.CallChain {
		if id == req.TargetAgent {
			return fmt.Errorf("%w: agent '%s' already in chain", ErrCallChainLoop, req.TargetAgent)
		}
	}
	return nil
}

// checkRateLimit 校验调用频率是否超过限制。
//
// 从 permission_rules 中查找匹配的限流配置，使用滑动窗口限流器检查。
//
// 参数：
//   - req：调用请求，包含目标 Agent、Skill 和调用方信息
//
// 返回值：
//   - error：超过限流阈值时返回 ErrRateLimited，通过时返回 nil
func (s *Service) checkRateLimit(req InvokeRequest) error {
	rules, _ := s.permRepo.FindRules(req.TargetAgent, req.Skill)
	for _, rule := range rules {
		if rule.RateLimitMax != nil && *rule.RateLimitMax > 0 && rule.RateLimitWindow != nil {
			window, err := time.ParseDuration(*rule.RateLimitWindow)
			if err != nil {
				continue
			}
			callerKey := fmt.Sprintf("user:%d", req.Caller.UserID)
			if req.Caller.AgentID != "" {
				callerKey = fmt.Sprintf("agent:%s", req.Caller.AgentID)
			}
			if !s.rateLimiter.Allow(req.TargetAgent, req.Skill, callerKey, int(*rule.RateLimitMax), window) {
				return fmt.Errorf("%w: exceeded %d calls per %s", ErrRateLimited, *rule.RateLimitMax, *rule.RateLimitWindow)
			}
			break // 使用第一个匹配的限流规则
		}
	}
	return nil
}

// checkApproval 检查目标 Skill 是否需要人工审批。
//
// 当 Skill 的 approval_mode 为 "manual" 时，调用请求不会立即执行，
// 而是进入审批队列等待 Owner 审批。
//
// 参数：
//   - req：调用请求，包含目标 Agent 和 Skill 名称
//
// 返回值：
//   - bool：是否需要审批
//   - error：查询失败时返回错误
func (s *Service) checkApproval(req InvokeRequest) (bool, error) {
	cap, err := s.capRepo.FindByAgentAndName(req.TargetAgent, req.Skill)
	if err != nil {
		return false, nil // skill not found handled elsewhere
	}
	return cap.ApprovalMode == "manual", nil
}

// createPendingApproval 为需要审批的调用创建审批记录。
//
// 该方法在目标 Skill 的 approval_mode 为 "manual" 时被调用。
// 它会创建一条异步调用记录（mode=async, status=submitted）和一条待审批记录，
// 然后返回 ErrApprovalRequired 错误，通知调用方需要等待审批。
//
// 参数：
//   - req：调用请求，包含目标 Agent、Skill 和调用方信息
//
// 返回值：
//   - *InvokeResponse：包含 task_id 和 pending 状态的响应
//   - error：始终返回 ErrApprovalRequired
func (s *Service) createPendingApproval(req InvokeRequest) (*InvokeResponse, error) {
	agent, err := s.agentRepo.FindByAgentID(req.TargetAgent)
	if err != nil {
		return nil, fmt.Errorf("agent '%s' not found", req.TargetAgent)
	}

	taskID := uuid.New().String()

	inv := &model.Invocation{
		TaskID:        taskID,
		CallerAgentID: req.Caller.AgentID,
		TargetAgentID: req.TargetAgent,
		SkillName:     req.Skill,
		Status:        "submitted",
		Mode:          "async",
	}
	if req.Caller.UserID > 0 {
		inv.CallerUserID = &req.Caller.UserID
	}
	s.invRepo.Create(inv)

	approval := &model.Approval{
		InvocationID: inv.ID,
		OwnerID:      agent.OwnerID,
		Status:       "pending",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	s.approvalRepo.Create(approval)

	logger.Infof("Approval required for %s.%s (task=%s, approval=%d)", req.TargetAgent, req.Skill, taskID, approval.ID)

	return &InvokeResponse{
		TaskID: taskID,
		Status: "pending",
	}, ErrApprovalRequired
}

// ReplyToTask 处理多轮对话的用户回复。
// 查找活跃的 TaskSession，合并输入后通过 SendReply 发送给 Agent。
func (s *Service) ReplyToTask(taskID string, replyInput json.RawMessage, callerUserID uint64) (*InvokeResponse, error) {
	session, ok := s.taskSessions.Get(taskID)
	if !ok {
		return nil, fmt.Errorf("no active session for task '%s'", taskID)
	}

	if session.Caller.UserID != callerUserID {
		return nil, fmt.Errorf("%w: not the original caller", ErrPermissionDenied)
	}

	conn := s.tunnelMgr.GetConn(session.TargetAgent)
	if conn == nil {
		s.taskSessions.Delete(taskID)
		s.invRepo.UpdateStatus(taskID, "failed", nil, "agent disconnected")
		return nil, fmt.Errorf("agent '%s' is not online", session.TargetAgent)
	}

	// 合并原始输入和回复输入
	mergedInput := mergeJSON(session.OriginalInput, replyInput)

	s.taskMsgRepo.Create(&model.TaskMessage{
		TaskID: taskID, Direction: "to_agent", MessageType: "reply",
	})
	s.invRepo.UpdateStatus(taskID, "working", nil, "")

	replyPayload := protocol.ReplyPayload{
		TaskID: taskID,
		Skill:  session.Skill,
		Input:  mergedInput,
		Caller: session.Caller,
	}

	timeout := time.Duration(session.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	invokeResult, err := conn.SendReply(session.RequestID, replyPayload, timeout)
	if err != nil {
		s.taskSessions.Delete(taskID)
		s.invRepo.UpdateStatus(taskID, "failed", nil, err.Error())
		return nil, err
	}

	if invokeResult.Type == "need_input" {
		session.Question = &invokeResult.NeedInput.Question
		session.Status = "input_required"
		// 更新 OriginalInput 为合并后的输入，以便下一轮继续合并
		session.OriginalInput = mergedInput
		s.invRepo.UpdateStatus(taskID, "input_required", nil, "")
		s.taskMsgRepo.Create(&model.TaskMessage{
			TaskID: taskID, Direction: "from_agent", MessageType: "question",
		})
		return &InvokeResponse{
			TaskID:   taskID,
			Status:   "input_required",
			Question: &invokeResult.NeedInput.Question,
		}, nil
	}

	// 最终结果
	s.taskSessions.Delete(taskID)
	result := invokeResult.Result
	latencyMs := uint(time.Since(session.StartTime).Milliseconds())

	if result.Status == "completed" {
		s.invRepo.UpdateStatus(taskID, "completed", &latencyMs, "")
		s.capRepo.IncrementCallCount(session.TargetAgent, session.Skill, latencyMs, true)
	} else {
		s.invRepo.UpdateStatus(taskID, "failed", &latencyMs, result.Error)
		s.capRepo.IncrementCallCount(session.TargetAgent, session.Skill, latencyMs, false)
	}

	return &InvokeResponse{
		TaskID: taskID,
		Status: result.Status,
		Output: result.Output,
		Error:  result.Error,
	}, nil
}

// mergeJSON 将两个 JSON 对象合并，reply 中的字段覆盖 original 中的同名字段。
func mergeJSON(original, reply json.RawMessage) json.RawMessage {
	var base, extra map[string]json.RawMessage
	if err := json.Unmarshal(original, &base); err != nil {
		base = make(map[string]json.RawMessage)
	}
	if err := json.Unmarshal(reply, &extra); err != nil {
		return original
	}
	for k, v := range extra {
		base[k] = v
	}
	merged, _ := json.Marshal(base)
	return merged
}

// GetTask 查询任务状态。
func (s *Service) GetTask(taskID string) (*InvokeResponse, error) {
	inv, err := s.invRepo.FindByTaskID(taskID)
	if err != nil {
		return nil, fmt.Errorf("task '%s' not found", taskID)
	}
	resp := &InvokeResponse{
		TaskID: inv.TaskID,
		Status: inv.Status,
		Error:  inv.ErrorMessage,
	}
	if session, ok := s.taskSessions.Get(taskID); ok && session.Question != nil {
		resp.Question = session.Question
	}
	return resp, nil
}

// CancelTask 取消一个多轮对话任务。
func (s *Service) CancelTask(taskID string, callerUserID uint64) error {
	session, ok := s.taskSessions.Get(taskID)
	if !ok {
		return fmt.Errorf("no active session for task '%s'", taskID)
	}
	if session.Caller.UserID != callerUserID {
		return ErrPermissionDenied
	}
	s.taskSessions.Delete(taskID)
	s.invRepo.UpdateStatus(taskID, "cancelled", nil, "cancelled by caller")
	return nil
}
