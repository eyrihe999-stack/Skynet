package gateway

import (
	"sync"

	"github.com/eyrihe999-stack/Skynet-sdk/logger"
)

// ConnectionManager 管理所有在线 Agent 的通信连接。
//
// 支持两种传输方式：
//   - AgentConn（tunnel 模式）：WebSocket 长连接
//   - WebhookTransport（direct 模式）：HTTP 回调
//
// 是网关服务查找目标 Agent 连接的唯一入口。
type ConnectionManager struct {
	mu         sync.RWMutex
	transports map[string]AgentTransport
}

// NewConnectionManager 创建并返回一个新的连接管理器实例。
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		transports: make(map[string]AgentTransport),
	}
}

// Register 将一个 Agent 的传输通道注册到管理器中。
// 如果该 Agent 已存在旧连接，会先关闭旧连接再替换。
func (m *ConnectionManager) Register(agentID string, transport AgentTransport) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if old, ok := m.transports[agentID]; ok {
		old.Close()
		logger.Warnf("Replaced existing connection for agent: %s", agentID)
	}

	m.transports[agentID] = transport
	logger.Infof("Connection registered: %s (total: %d)", agentID, len(m.transports))
}

// Unregister 从管理器中移除指定 Agent 的连接记录。
func (m *ConnectionManager) Unregister(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.transports, agentID)
	logger.Infof("Connection unregistered: %s (total: %d)", agentID, len(m.transports))
}

// GetTransport 根据 Agent ID 查找并返回对应的传输通道。
// 若 Agent 不在线则返回 nil。
func (m *ConnectionManager) GetTransport(agentID string) AgentTransport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.transports[agentID]
}

// IsOnline 检查指定 Agent 是否有活跃连接。
func (m *ConnectionManager) IsOnline(agentID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.transports[agentID]
	return ok
}

// Count 返回当前活跃连接的数量。
func (m *ConnectionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.transports)
}
