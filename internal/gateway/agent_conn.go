package gateway

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/skynetplatform/skynet-sdk/logger"
	"github.com/skynetplatform/skynet-sdk/protocol"
)

// AgentConn 封装了与单个 Agent 之间的 WebSocket 连接。
//
// 它是网关与 Agent 运行时之间的通信通道，负责：
//   - 通过 WebSocket 发送 invoke（技能调用）消息到 Agent
//   - 接收 Agent 返回的 result（执行结果）消息
//   - 通过 requestID 将响应消息匹配到对应的请求
//   - 处理心跳（ping/pong）以维持连接活性
//   - 管理连接的生命周期（关闭通知与资源清理）
//
// 字段说明：
//   - agentID：所连接 Agent 的唯一标识符
//   - conn：底层的 gorilla/websocket 连接实例
//   - writeMu：写操作互斥锁，确保 WebSocket 写操作的线程安全
//   - pending：请求-响应映射表，key 为 requestID，value 为等待响应的通道
//   - pendMu：pending 映射表的互斥锁
//   - closeCh：连接关闭信号通道，关闭时通知所有等待中的操作
//   - closeOnce：确保关闭操作只执行一次
type AgentConn struct {
	agentID  string
	conn     *websocket.Conn
	writeMu  sync.Mutex
	pending  map[string]chan *protocol.Message // requestID → 响应通道
	pendMu   sync.Mutex
	closeCh  chan struct{}
	closeOnce sync.Once
}

// NewAgentConn 创建一个新的 Agent 连接封装实例，并启动后台消息读取循环。
//
// 创建后会自动启动一个 goroutine 运行 readLoop，持续从 WebSocket 读取消息
// 并进行分发处理（结果匹配、心跳响应等）。
//
// 参数：
//   - agentID：Agent 的唯一标识符，用于日志记录和连接标识
//   - conn：已建立的 WebSocket 连接实例
//
// 返回值：
//   - *AgentConn：初始化完成并已启动读取循环的连接实例
func NewAgentConn(agentID string, conn *websocket.Conn) *AgentConn {
	ac := &AgentConn{
		agentID: agentID,
		conn:    conn,
		pending: make(map[string]chan *protocol.Message),
		closeCh: make(chan struct{}),
	}
	go ac.readLoop()
	return ac
}

// SendInvoke 向 Agent 发送技能调用请求，并同步等待执行结果返回。
//
// 工作流程：
//  1. 生成唯一的 requestID 用于请求-响应匹配
//  2. 创建响应等待通道并注册到 pending 映射表
//  3. 构造 invoke 消息并通过 WebSocket 发送给 Agent
//  4. 阻塞等待 Agent 返回结果、超时或连接断开
//  5. 清理 pending 映射表中的等待通道
//
// 参数：
//   - payload：调用载荷，包含技能名称、输入参数、调用方信息和超时设置
//   - timeout：等待 Agent 响应的最大时间，超过后返回超时错误
//
// 返回值：
//   - string：本次调用生成的 requestID，用于后续多轮对话的 SendReply
//   - *protocol.InvokeResult：Agent 返回的响应，可能是最终结果或追问
//   - error：消息发送失败、响应解析失败、超时或连接断开时返回错误
func (ac *AgentConn) SendInvoke(payload protocol.InvokePayload, timeout time.Duration) (string, *protocol.InvokeResult, error) {
	requestID := uuid.New().String()

	// 创建带缓冲的响应通道并注册到 pending 映射表
	ch := make(chan *protocol.Message, 1)
	ac.pendMu.Lock()
	ac.pending[requestID] = ch
	ac.pendMu.Unlock()

	defer func() {
		ac.pendMu.Lock()
		delete(ac.pending, requestID)
		ac.pendMu.Unlock()
	}()

	// 构造并发送 invoke 消息
	msg, err := protocol.NewMessage(protocol.TypeInvoke, requestID, payload)
	if err != nil {
		return requestID, nil, err
	}

	ac.writeMu.Lock()
	err = ac.conn.WriteJSON(msg)
	ac.writeMu.Unlock()
	if err != nil {
		return requestID, nil, fmt.Errorf("write invoke failed: %w", err)
	}

	// 阻塞等待三种结果之一：收到响应、超时、连接断开
	select {
	case resp := <-ch:
		result, err := ac.parseInvokeResponse(resp)
		return requestID, result, err
	case <-time.After(timeout):
		return requestID, nil, fmt.Errorf("invoke timed out after %s", timeout)
	case <-ac.closeCh:
		return requestID, nil, fmt.Errorf("agent disconnected")
	}
}

// parseInvokeResponse 将 WebSocket 消息解析为统一的 InvokeResult。
func (ac *AgentConn) parseInvokeResponse(msg *protocol.Message) (*protocol.InvokeResult, error) {
	switch msg.Type {
	case protocol.TypeResult:
		var result protocol.ResultPayload
		if err := msg.ParsePayload(&result); err != nil {
			return nil, fmt.Errorf("parse result failed: %w", err)
		}
		return &protocol.InvokeResult{Type: "result", Result: &result}, nil
	case protocol.TypeNeedInput:
		var needInput protocol.NeedInputPayload
		if err := msg.ParsePayload(&needInput); err != nil {
			return nil, fmt.Errorf("parse need_input failed: %w", err)
		}
		return &protocol.InvokeResult{Type: "need_input", NeedInput: &needInput}, nil
	default:
		return nil, fmt.Errorf("unexpected message type: %s", msg.Type)
	}
}

// SendReply 向 Agent 发送多轮对话的回复消息，并等待下一轮响应。
// 返回 InvokeResult，可能是最终结果（TypeResult）或新一轮追问（TypeNeedInput）。
func (ac *AgentConn) SendReply(requestID string, payload protocol.ReplyPayload, timeout time.Duration) (*protocol.InvokeResult, error) {
	ch := make(chan *protocol.Message, 1)
	ac.pendMu.Lock()
	ac.pending[requestID] = ch
	ac.pendMu.Unlock()

	defer func() {
		ac.pendMu.Lock()
		delete(ac.pending, requestID)
		ac.pendMu.Unlock()
	}()

	msg, err := protocol.NewMessage(protocol.TypeReply, requestID, payload)
	if err != nil {
		return nil, err
	}

	ac.writeMu.Lock()
	err = ac.conn.WriteJSON(msg)
	ac.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write reply failed: %w", err)
	}

	select {
	case resp := <-ch:
		return ac.parseInvokeResponse(resp)
	case <-time.After(timeout):
		return nil, fmt.Errorf("reply timed out after %s", timeout)
	case <-ac.closeCh:
		return nil, fmt.Errorf("agent disconnected")
	}
}

// CloseCh 返回连接关闭信号通道。
//
// 当连接关闭时该通道会被 close，调用方可以通过 select 监听此通道
// 来感知连接断开事件，以便及时清理资源或中止等待操作。
//
// 返回值：
//   - <-chan struct{}：只读的关闭信号通道
func (ac *AgentConn) CloseCh() <-chan struct{} {
	return ac.closeCh
}

// Close 终止 WebSocket 连接并释放相关资源。
//
// 使用 sync.Once 确保关闭操作只执行一次，避免重复关闭导致的 panic。
// 关闭流程：先关闭 closeCh 通道通知所有等待中的操作，然后关闭底层 WebSocket 连接。
func (ac *AgentConn) Close() {
	ac.closeOnce.Do(func() {
		close(ac.closeCh)
		ac.conn.Close()
	})
}

// readLoop 是后台消息读取循环，持续从 WebSocket 读取消息并进行分发处理。
//
// 消息类型处理：
//   - TypeResult（执行结果）：根据 requestID 查找对应的等待通道，将结果发送到通道以唤醒等待方
//   - TypePong（心跳响应）：忽略，无需额外处理
//   - TypePing（心跳请求）：回复 pong 消息以维持连接活性
//
// 当读取出错时（连接断开、协议错误等），循环退出并通过 defer 调用 Close 清理资源。
func (ac *AgentConn) readLoop() {
	defer ac.Close()

	for {
		var msg protocol.Message
		if err := ac.conn.ReadJSON(&msg); err != nil {
			select {
			case <-ac.closeCh:
				// 连接已被主动关闭，静默退出
			default:
				logger.Debugf("Agent %s read error: %v", ac.agentID, err)
			}
			return
		}

		switch msg.Type {
		case protocol.TypeResult, protocol.TypeNeedInput:
			// 执行结果或追问消息：匹配 requestID 并发送到对应的等待通道
			ac.pendMu.Lock()
			ch, ok := ac.pending[msg.RequestID]
			ac.pendMu.Unlock()
			if ok {
				ch <- &msg
			}
		case protocol.TypePong:
			// 心跳响应，无需处理
		case protocol.TypePing:
			// 心跳请求，回复 pong
			ac.sendPong()
		}
	}
}

// sendPong 发送心跳响应（pong）消息到 Agent。
//
// 当收到 Agent 发来的 ping 消息时调用此方法，通过回复 pong 维持连接活性。
// 使用 writeMu 确保写操作的线程安全。
func (ac *AgentConn) sendPong() {
	msg, _ := protocol.NewMessage(protocol.TypePong, "", nil)
	ac.writeMu.Lock()
	defer ac.writeMu.Unlock()
	ac.conn.WriteJSON(msg)
}
