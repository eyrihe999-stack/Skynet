package gateway

import (
	"sync"

	"github.com/eyrihe999-stack/Skynet-sdk/logger"
)

// TunnelManager 是隧道连接管理器，维护所有在线 Agent 的 WebSocket 连接映射。
//
// 它是网关服务查找目标 Agent 连接的唯一入口，提供连接的注册、注销和查询功能。
// 使用读写锁（sync.RWMutex）保证并发安全，支持多个读操作同时进行，写操作互斥。
//
// 在架构中，TunnelManager 位于 Service 和 AgentConn 之间，Service 通过它
// 查找目标 Agent 的连接，HTTP handler 层通过它注册新建立的 WebSocket 连接。
//
// 字段说明：
//   - mu：读写互斥锁，保护 conns 映射表的并发访问安全
//   - conns：Agent 连接映射表，key 为 agentID，value 为对应的 AgentConn 实例
type TunnelManager struct {
	mu    sync.RWMutex
	conns map[string]*AgentConn
}

// NewTunnelManager 创建并返回一个新的隧道连接管理器实例。
//
// 初始化时创建空的连接映射表，准备接受 Agent 连接的注册。
//
// 返回值：
//   - *TunnelManager：初始化完成的隧道管理器实例
func NewTunnelManager() *TunnelManager {
	return &TunnelManager{
		conns: make(map[string]*AgentConn),
	}
}

// Register 将一个 Agent 连接注册到管理器中。
//
// 如果该 Agent 已存在旧连接，会先关闭旧连接再替换为新连接，
// 确保同一个 Agent 在任意时刻只有一个活跃连接。
// 这处理了 Agent 重连的场景（例如网络断开后重新建立 WebSocket 连接）。
//
// 参数：
//   - agentID：Agent 的唯一标识符
//   - conn：新建立的 Agent WebSocket 连接封装实例
func (m *TunnelManager) Register(agentID string, conn *AgentConn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果已存在旧连接，先关闭旧连接再替换
	if old, ok := m.conns[agentID]; ok {
		old.Close()
		logger.Warnf("Replaced existing tunnel for agent: %s", agentID)
	}

	m.conns[agentID] = conn
	logger.Infof("Tunnel registered: %s (total: %d)", agentID, len(m.conns))
}

// Unregister 从管理器中移除指定 Agent 的连接记录。
//
// 注意：此方法仅从映射表中删除记录，不会主动关闭 WebSocket 连接。
// 通常在 Agent 断开连接后由 handler 层调用。
//
// 参数：
//   - agentID：要移除的 Agent 唯一标识符
func (m *TunnelManager) Unregister(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conns, agentID)
	logger.Infof("Tunnel unregistered: %s (total: %d)", agentID, len(m.conns))
}

// GetConn 根据 Agent ID 查找并返回对应的 WebSocket 连接。
//
// 此方法是 Service.Invoke 调用链的关键一环，用于获取目标 Agent 的连接
// 以便发送 invoke 消息。使用读锁以支持并发查询。
//
// 参数：
//   - agentID：目标 Agent 的唯一标识符
//
// 返回值：
//   - *AgentConn：对应的连接实例；若 Agent 不在线则返回 nil
func (m *TunnelManager) GetConn(agentID string) *AgentConn {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.conns[agentID]
}

// IsOnline 检查指定 Agent 是否有活跃的隧道连接。
//
// 可用于在发起调用前快速判断 Agent 的在线状态，
// 或在 API 层向用户展示 Agent 的连接状态。
//
// 参数：
//   - agentID：要检查的 Agent 唯一标识符
//
// 返回值：
//   - bool：Agent 在线返回 true，离线返回 false
func (m *TunnelManager) IsOnline(agentID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.conns[agentID]
	return ok
}

// Count 返回当前活跃连接的数量。
//
// 可用于监控和统计当前在线的 Agent 总数。
//
// 返回值：
//   - int：当前注册在管理器中的活跃连接数
func (m *TunnelManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.conns)
}
