package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/eyrihe999-stack/Skynet/internal/authz"
	"github.com/eyrihe999-stack/Skynet/internal/gateway"
	"github.com/eyrihe999-stack/Skynet/internal/registry"
	"github.com/eyrihe999-stack/Skynet-sdk/logger"
	"github.com/eyrihe999-stack/Skynet-sdk/protocol"
)

// upgrader 是 WebSocket 协议升级器，用于将普通 HTTP 连接升级为 WebSocket 连接。
// CheckOrigin 返回 true 表示允许所有来源的连接（开发阶段配置，生产环境应限制来源）。
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// TunnelHandler 是 WebSocket 隧道端点的 HTTP 处理器。
// Agent 通过此处理器建立与 Platform 的持久 WebSocket 连接（反向隧道），
// 使 Platform 能够主动向 Agent 发送技能调用请求。
// 整个隧道生命周期包括：协议升级 -> Agent 注册认证 -> 隧道建立 -> 持久通信 -> 断开清理。
//
// 字段说明：
//   - tunnelMgr: Gateway 隧道管理器，管理所有活跃的 Agent WebSocket 连接。
//   - registrySvc: Registry 注册中心服务，用于 Agent 的注册与注销。
//   - authSvc: 认证授权服务，用于验证 Agent 携带的 API Key 是否有效。
//   - eventBus: 事件总线，用于发布 Agent 上下线事件。
type TunnelHandler struct {
	tunnelMgr   *gateway.TunnelManager
	registrySvc *registry.Service
	authSvc     *authz.Service
	eventBus    *gateway.EventBus
}

// NewTunnelHandler 创建并返回一个新的 TunnelHandler 实例。
//
// 参数：
//   - tunnelMgr: Gateway 隧道管理器，负责维护 Agent 连接的注册和查找。
//   - registrySvc: Registry 注册中心服务，负责 Agent 元数据的持久化管理。
//   - authSvc: 认证授权服务，负责校验 Agent 提供的 API Key。
//   - eventBus: 事件总线，用于发布 Agent 上下线事件。
//
// 返回值：
//   - *TunnelHandler: 初始化完成的隧道处理器实例。
func NewTunnelHandler(tunnelMgr *gateway.TunnelManager, registrySvc *registry.Service, authSvc *authz.Service, eventBus *gateway.EventBus) *TunnelHandler {
	return &TunnelHandler{
		tunnelMgr:   tunnelMgr,
		registrySvc: registrySvc,
		authSvc:     authSvc,
		eventBus:    eventBus,
	}
}

// HandleTunnel 处理 Agent WebSocket 隧道连接请求（GET /api/v1/tunnel）。
//
// 该方法是 Agent 接入 Skynet 平台的核心入口。Agent 通过 WebSocket 连接到此端点，
// 发送注册消息完成身份认证后，建立持久的反向隧道。此后 Platform 可通过该隧道
// 向 Agent 转发技能调用请求。
//
// 处理流程：
//  1. 将 HTTP 连接升级为 WebSocket 连接。
//  2. 等待并读取 Agent 发送的注册消息（类型必须为 protocol.TypeRegister）。
//  3. 解析注册消息中的 Agent Card（包含 Agent 元信息和 API Key）。
//  4. 通过 AuthZ 服务验证 API Key 的有效性，确认 Agent 所有者身份。
//  5. 在 Registry 中注册 Agent，获取 Agent Secret。
//  6. 向 Agent 发送注册成功响应。
//  7. 在 TunnelManager 中注册 Agent 连接，使其可被 Gateway 路由到。
//  8. 阻塞等待连接关闭，连接断开后执行清理（从 TunnelManager 和 Registry 中注销）。
//
// 参数：
//   - c: Gin 请求上下文，包含待升级的 HTTP 连接。
//
// 注意：该方法会阻塞当前 goroutine 直到 Agent 断开连接。每个 Agent 连接独占一个 goroutine。
func (h *TunnelHandler) HandleTunnel(c *gin.Context) {
	// 将 HTTP 连接升级为 WebSocket 连接
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Errorf("WebSocket upgrade failed: %v", err)
		return
	}

	// 等待 Agent 发送注册消息
	var msg protocol.Message
	if err := ws.ReadJSON(&msg); err != nil {
		logger.Errorf("Failed to read registration: %v", err)
		ws.Close()
		return
	}

	// 校验消息类型必须为注册消息
	if msg.Type != protocol.TypeRegister {
		sendError(ws, "expected register message")
		ws.Close()
		return
	}

	// 解析注册消息的载荷，提取 Agent Card 信息
	var payload protocol.RegisterPayload
	if err := msg.ParsePayload(&payload); err != nil {
		sendError(ws, "invalid register payload")
		ws.Close()
		return
	}

	card := payload.Card

	// 通过 Agent Card 中的 API Key 验证 Agent 所有者身份
	user, err := h.authSvc.ValidateAPIKey(card.OwnerAPIKey)
	if err != nil {
		sendRegistered(ws, false, "", "invalid API key")
		ws.Close()
		return
	}

	// 在 Registry 中注册 Agent，获取分配的 Agent Secret
	agentSecret, err := h.registrySvc.RegisterAgent(card, user.ID)
	if err != nil {
		sendRegistered(ws, false, "", "registration failed: "+err.Error())
		ws.Close()
		return
	}

	// 向 Agent 发送注册成功响应，包含 Agent Secret
	sendRegistered(ws, true, agentSecret, "")

	// 创建 Agent 连接对象并注册到隧道管理器，使 Gateway 可通过隧道路由请求
	agentConn := gateway.NewAgentConn(card.AgentID, ws)
	h.tunnelMgr.Register(card.AgentID, agentConn)

	// 发布 Agent 上线事件
	h.eventBus.PublishJSON("agent_online", map[string]string{"agent_id": card.AgentID})

	// 启动数据库心跳更新定时器，每 30 秒更新一次 last_heartbeat_at，
	// 使心跳监控不会将活跃的 WebSocket 连接标记为 offline。
	heartbeatDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.registrySvc.Heartbeat(card.AgentID)
			case <-heartbeatDone:
				return
			}
		}
	}()

	// 阻塞等待连接关闭（Agent 主动断开或网络异常）
	<-agentConn.CloseCh()
	close(heartbeatDone)

	// Agent 断开后执行清理：从隧道管理器和注册中心中注销
	h.tunnelMgr.Unregister(card.AgentID)
	h.registrySvc.UnregisterAgent(card.AgentID)

	// 发布 Agent 下线事件
	h.eventBus.PublishJSON("agent_offline", map[string]string{"agent_id": card.AgentID})

	logger.Infof("Agent disconnected: %s", card.AgentID)
}

// sendRegistered 向 Agent 的 WebSocket 连接发送注册结果消息。
// 该函数在 Agent 注册流程的最后阶段调用，通知 Agent 注册是否成功。
//
// 参数：
//   - ws: Agent 的 WebSocket 连接实例。
//   - success: 注册是否成功。
//   - secret: 注册成功时分配给 Agent 的密钥，失败时为空字符串。
//   - errMsg: 注册失败时的错误信息，成功时为空字符串。
func sendRegistered(ws *websocket.Conn, success bool, secret, errMsg string) {
	resp := protocol.RegisteredPayload{
		Success:     success,
		AgentSecret: secret,
		Error:       errMsg,
	}
	msg, _ := protocol.NewMessage(protocol.TypeRegistered, "", resp)
	ws.WriteJSON(msg)
}

// sendError 向 WebSocket 连接发送通用错误消息。
// 用于在协议握手阶段（注册消息校验前）向 Agent 反馈错误信息。
//
// 参数：
//   - ws: 目标 WebSocket 连接实例。
//   - errMsg: 错误描述信息。
func sendError(ws *websocket.Conn, errMsg string) {
	msg, _ := protocol.NewMessage(protocol.TypeError, "", map[string]string{"error": errMsg})
	ws.WriteJSON(msg)
}
