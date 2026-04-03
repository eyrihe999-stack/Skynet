package gateway

import (
	"sync"
	"time"

	"github.com/skynetplatform/skynet-sdk/protocol"
)

// TaskSession 记录一个多轮对话任务的内存状态。
// 当 Agent 返回 need_input 时创建，在最终结果返回或取消时删除。
type TaskSession struct {
	TaskID        string
	TargetAgent   string
	RequestID     string // AgentConn 层的 WS requestID
	Skill         string
	Caller        protocol.CallerInfo
	Status        string
	Question      *protocol.Question
	StartTime     time.Time
	TimeoutMs     int
	OriginalInput []byte
}

// TaskSessionManager 管理所有活跃的多轮对话会话。
type TaskSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*TaskSession
}

func NewTaskSessionManager() *TaskSessionManager {
	return &TaskSessionManager{
		sessions: make(map[string]*TaskSession),
	}
}

func (m *TaskSessionManager) Store(session *TaskSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.TaskID] = session
}

func (m *TaskSessionManager) Get(taskID string) (*TaskSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[taskID]
	return s, ok
}

func (m *TaskSessionManager) Delete(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, taskID)
}
