// Package api 定义 Skynet 平台的 HTTP API 路由层。
// 该包是 Platform 的对外入口，负责将来自 CLI、Dashboard、其他 Agent 的 HTTP 请求
// 路由到内部的 Registry（注册中心）、Gateway（网关）、AuthZ（认证授权）等服务。
// 路由分为公开路由（注册/登录）、WebSocket 隧道路由（Agent 连接）和需认证路由（Agent 管理、能力搜索、技能调用等）。
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/eyrihe999-stack/Skynet/internal/api/handler"
	"github.com/eyrihe999-stack/Skynet/internal/authz"
)

// Deps 是路由初始化所需的全部外部依赖集合。
// 它将认证服务、各 Handler 实例统一注入到路由配置函数中，
// 避免在路由层直接创建或查找服务实例，便于测试和解耦。
//
// 字段说明：
//   - AuthSvc: 认证授权服务，用于生成 AuthRequired 中间件以校验请求令牌。
//   - AuthHandler: 认证相关的 HTTP 处理器，提供注册、登录、用户信息等端点。
//   - RegistryHandler: Agent 注册中心的 HTTP 处理器，提供 Agent 增删查及能力搜索等端点。
//   - InvokeHandler: 技能调用的 HTTP 处理器，负责将调用请求转发到 Gateway 服务。
//   - TunnelHandler: WebSocket 隧道的 HTTP 处理器，负责 Agent 反向通道的建立与管理。
//   - InvocationHandler: 调用历史记录的 HTTP 处理器，提供调用记录的查询端点。
//   - EventsHandler: SSE 事件流处理器，向客户端实时推送 Agent 上下线、调用完成等平台事件。
type Deps struct {
	AuthSvc           *authz.Service
	AuthHandler       *authz.Handler
	RegistryHandler   *handler.RegistryHandler
	InvokeHandler     *handler.InvokeHandler
	TunnelHandler     *handler.TunnelHandler
	InvocationHandler *handler.InvocationHandler
	ApprovalHandler   *handler.ApprovalHandler
	PermissionHandler *handler.PermissionHandler
	TaskHandler       *handler.TaskHandler
	EventsHandler     *handler.EventsHandler
}

// SetupRouter 根据传入的依赖创建并配置 Gin 引擎及全部 API 路由。
//
// 参数：
//   - deps: 包含所有 Handler 和服务依赖的 Deps 结构体。
//
// 返回值：
//   - *gin.Engine: 已配置完成的 Gin HTTP 引擎实例，可直接用于启动 HTTP 服务。
//
// 路由结构如下：
//   - GET  /health                         — 健康检查（公开）
//   - POST /api/v1/auth/register           — 用户注册（公开）
//   - POST /api/v1/auth/login              — 用户登录（公开）
//   - GET  /api/v1/tunnel                  — Agent WebSocket 隧道（使用自身的 API Key 认证）
//   - GET  /api/v1/agents                  — 获取 Agent 列表（需认证）
//   - GET  /api/v1/agents/:agent_id        — 获取单个 Agent 详情（需认证）
//   - DELETE /api/v1/agents/:agent_id      — 删除指定 Agent（需认证）
//   - POST /api/v1/agents/:agent_id/heartbeat — Agent 心跳上报（需认证）
//   - GET  /api/v1/capabilities            — 搜索 Agent 能力（需认证）
//   - POST /api/v1/invoke                  — 调用 Agent 技能（需认证）
//   - GET  /api/v1/invocations             — 查询调用历史记录（需认证）
//   - GET  /api/v1/events                  — 实时事件流 SSE（需认证）
//   - GET  /api/v1/auth/profile            — 获取当前用户信息（需认证）
func SetupRouter(deps Deps) *gin.Engine {
	r := gin.Default()

	// 健康检查端点，用于负载均衡器或监控系统探测服务是否存活
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API v1 版本路由组
	v1 := r.Group("/api/v1")
	{
		// 公开的认证路由组，无需携带令牌即可访问
		auth := v1.Group("/auth")
		{
			auth.POST("/register", deps.AuthHandler.Register)
			auth.POST("/login", deps.AuthHandler.Login)
		}

		// WebSocket 隧道端点，Agent 通过此端点建立持久连接。
		// 该端点使用 Agent 自身携带的 API Key 进行认证，不经过通用 AuthRequired 中间件。
		v1.GET("/tunnel", deps.TunnelHandler.HandleTunnel)

		// 需要认证的路由组，所有请求必须通过 AuthRequired 中间件校验 JWT 令牌
		authenticated := v1.Group("")
		authenticated.Use(authz.AuthRequired(deps.AuthSvc))
		{
			// Agent 管理相关路由
			authenticated.GET("/agents", deps.RegistryHandler.ListAgents)
			authenticated.GET("/agents/:agent_id", deps.RegistryHandler.GetAgent)
			authenticated.DELETE("/agents/:agent_id", deps.RegistryHandler.DeleteAgent)
			authenticated.POST("/agents/:agent_id/heartbeat", deps.RegistryHandler.Heartbeat)

			// 能力搜索路由
			authenticated.GET("/capabilities", deps.RegistryHandler.SearchCapabilities)

			// 技能调用路由
			authenticated.POST("/invoke", deps.InvokeHandler.Invoke)

			// 调用历史查询路由
			authenticated.GET("/invocations", deps.InvocationHandler.ListInvocations)

			// 权限规则管理路由
			authenticated.GET("/agents/:agent_id/permissions", deps.PermissionHandler.ListRules)
			authenticated.POST("/agents/:agent_id/permissions", deps.PermissionHandler.CreateRule)
			authenticated.DELETE("/agents/:agent_id/permissions/:rule_id", deps.PermissionHandler.DeleteRule)

			// 审批流程路由
			authenticated.GET("/approvals", deps.ApprovalHandler.ListPending)
			authenticated.POST("/approvals/:id", deps.ApprovalHandler.Decide)

			// 任务管理路由（多轮对话）
			authenticated.GET("/tasks/:task_id", deps.TaskHandler.GetTask)
			authenticated.POST("/tasks/:task_id/reply", deps.TaskHandler.Reply)
			authenticated.POST("/tasks/:task_id/cancel", deps.TaskHandler.Cancel)

			// 实时事件流路由（SSE）
			authenticated.GET("/events", deps.EventsHandler.Stream)

			// 当前用户信息路由
			authenticated.GET("/auth/profile", deps.AuthHandler.Profile)
			authenticated.POST("/auth/regenerate-key", deps.AuthHandler.RegenerateKey)
		}
	}

	return r
}
