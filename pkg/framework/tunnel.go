package framework

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/skynetplatform/skynet/pkg/logger"
	"github.com/skynetplatform/skynet/pkg/protocol"
)

// TunnelClient 是 WebSocket 反向通道客户端，负责管理 Agent 与 Gateway 之间的持久连接。
//
// TunnelClient 是生产模式下 Agent 的核心网络组件。它通过 WebSocket 连接到 Skynet Gateway，
// 完成 Agent 注册，然后在持久连接上接收 Skill 调用请求并返回执行结果。
// 同时维护心跳机制保持连接活跃，并在连接断开时自动重连。
//
// 字段说明：
//   - gatewayURL: Gateway 的 WebSocket 基础 URL（wss:// 或 ws://）
//   - card: Agent Card，注册时发送给 Gateway 的能力名片
//   - skills: 已注册的 Skill 集合，用于根据调用请求路由到对应的 Handler
//   - conn: 当前活跃的 WebSocket 连接实例
//   - done: 关闭信号通道，用于协调各 goroutine 的优雅退出
//   - mu: 互斥锁，保护 WebSocket 写操作的并发安全（WebSocket 不支持并发写入）
type TunnelClient struct {
	gatewayURL string
	card       protocol.AgentCard
	skills     map[string]Skill
	conn       *websocket.Conn
	done       chan struct{}
	mu         sync.Mutex
	wg         sync.WaitGroup
}

// NewTunnelClient 创建一个新的 WebSocket 反向通道客户端实例。
//
// 该函数仅初始化 TunnelClient 的内部状态，不会立即建立连接。
// 需要调用 Connect() 或 ConnectWithRetry() 来实际建立 WebSocket 连接。
//
// 参数：
//   - gatewayURL: Gateway 的 WebSocket 基础 URL（如 "wss://gateway.skynet.io"）
//   - card: Agent Card，包含 Agent 的身份和能力信息
//   - skills: 已注册的 Skill 集合，以 Skill 名称为键
//
// 返回值：
//   - *TunnelClient: 初始化完成的通道客户端实例
func NewTunnelClient(gatewayURL string, card protocol.AgentCard, skills map[string]Skill) *TunnelClient {
	return &TunnelClient{
		gatewayURL: gatewayURL,
		card:       card,
		skills:     skills,
		done:       make(chan struct{}),
	}
}

// Connect 建立 WebSocket 连接并启动消息读取和心跳循环。
//
// 连接流程：
//  1. 拨号连接到 Gateway 的 /api/v1/tunnel 端点
//  2. 发送 Register 消息（包含 Agent Card）
//  3. 等待并验证 Registered 响应（10 秒超时）
//  4. 启动 readLoop（读取消息循环）和 heartbeatLoop（心跳循环）两个后台 goroutine
//
// 返回值：
//   - error: 连接失败、注册失败或被拒绝时返回错误，成功时返回 nil
func (t *TunnelClient) Connect() error {
	url := t.gatewayURL + "/api/v1/tunnel"
	logger.Infof("Connecting to gateway: %s", url)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("tunnel dial failed: %w", err)
	}
	t.conn = conn

	// 发送注册消息
	if err := t.sendRegister(); err != nil {
		conn.Close()
		return fmt.Errorf("registration failed: %w", err)
	}

	// 等待注册响应
	if err := t.waitRegistered(); err != nil {
		conn.Close()
		return err
	}

	logger.Infof("Agent '%s' registered and connected", t.card.AgentID)

	// 启动消息读取和心跳循环
	go t.readLoop()
	go t.heartbeatLoop()

	return nil
}

// Close 优雅关闭 WebSocket 通道连接。
//
// 该方法会关闭 done 通道以通知所有后台 goroutine 退出，
// 然后发送 WebSocket Close 帧并关闭底层连接。
// 该方法是并发安全的，通过互斥锁保护写操作。
func (t *TunnelClient) Close() {
	close(t.done)
	t.wg.Wait()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn != nil {
		t.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		t.conn.Close()
	}
}

// Wait 阻塞等待通道连接关闭。
//
// 该方法会一直阻塞直到 done 通道被关闭（连接断开或调用 Close()）。
// 通常与 Connect() 配合使用，在连接建立后等待连接结束。
func (t *TunnelClient) Wait() {
	<-t.done
}

// sendRegister 向 Gateway 发送注册消息，将 Agent Card 提交给 Gateway 进行注册。
//
// 注册消息包含完整的 Agent Card 信息（Agent 身份、Skill 能力列表等），
// Gateway 收到后会将该 Agent 登记到网络中。
//
// 返回值：
//   - error: 消息构造或发送失败时返回错误
func (t *TunnelClient) sendRegister() error {
	payload := protocol.RegisterPayload{Card: t.card}
	msg, err := protocol.NewMessage(protocol.TypeRegister, "", payload)
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn.WriteJSON(msg)
}

// waitRegistered 等待并验证 Gateway 的注册响应。
//
// 设置 10 秒读取超时，期望收到 TypeRegistered 类型的成功响应。
// 如果收到错误消息、非预期的消息类型或注册被拒绝，返回对应的错误。
//
// 返回值：
//   - error: 注册失败、超时或被拒绝时返回错误，成功时返回 nil
func (t *TunnelClient) waitRegistered() error {
	t.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer t.conn.SetReadDeadline(time.Time{})

	var msg protocol.Message
	if err := t.conn.ReadJSON(&msg); err != nil {
		return fmt.Errorf("failed to read registration response: %w", err)
	}

	if msg.Type == protocol.TypeError {
		return fmt.Errorf("registration error: %s", string(msg.Payload))
	}
	if msg.Type != protocol.TypeRegistered {
		return fmt.Errorf("unexpected message type: %s", msg.Type)
	}

	var resp protocol.RegisteredPayload
	if err := msg.ParsePayload(&resp); err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("registration rejected: %s", resp.Error)
	}
	return nil
}

// readLoop 是 WebSocket 消息读取循环，持续监听来自 Gateway 的消息。
//
// 该方法在独立的 goroutine 中运行，处理以下消息类型：
//   - TypeInvoke: Skill 调用请求，异步分发到 handleInvoke 处理
//   - TypeReply: 多轮对话回复消息，异步分发到 handleReply 处理
//   - TypePing: Gateway 发送的心跳请求，回复 Pong
//
// 当读取发生错误或 done 通道被关闭时，循环退出。
// 退出时会关闭 done 通道（如果尚未关闭），通知其他 goroutine。
func (t *TunnelClient) readLoop() {
	defer func() {
		select {
		case <-t.done:
		default:
			close(t.done)
		}
	}()

	for {
		var msg protocol.Message
		err := t.conn.ReadJSON(&msg)
		if err != nil {
			select {
			case <-t.done:
				return
			default:
				logger.Errorf("Tunnel read error: %v", err)
				return
			}
		}

		switch msg.Type {
		case protocol.TypeInvoke:
			t.wg.Add(1)
			go t.handleInvoke(msg)
		case protocol.TypeReply:
			t.wg.Add(1)
			go t.handleReply(msg)
		case protocol.TypePing:
			t.sendPong()
		}
	}
}

// heartbeatLoop 是心跳发送循环，每 30 秒向 Gateway 发送一次 Ping 消息以保持连接活跃。
//
// 该方法在独立的 goroutine 中运行，当 done 通道被关闭时退出。
// 心跳机制防止长时间空闲导致的连接超时断开。
func (t *TunnelClient) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.done:
			return
		case <-ticker.C:
			t.sendPing()
		}
	}
}

// handleInvoke 处理来自 Gateway 的 Skill 调用请求。
//
// 处理流程：
//  1. 解析 InvokePayload，获取要调用的 Skill 名称、输入参数和调用者信息
//  2. 在已注册的 Skill 中查找目标 Skill
//  3. 构造 Context 和 Input，调用 Skill 的 Handler 函数
//  4. 将执行结果（成功或失败）通过 sendResult 发送回 Gateway
//
// 该方法在独立的 goroutine 中执行，支持并发处理多个调用请求。
//
// 参数：
//   - msg: 来自 Gateway 的调用请求消息，包含 RequestID 和 InvokePayload
func (t *TunnelClient) handleInvoke(msg protocol.Message) {
	defer t.wg.Done()

	// panic recovery：捕获 Handler 内部的 panic，返回 failed 状态而非断掉整个连接
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Skill handler panic (request=%s): %v", msg.RequestID, r)
			t.sendResult(msg.RequestID, protocol.ResultPayload{
				Status: "failed",
				Error:  fmt.Sprintf("internal error: handler panic: %v", r),
			})
		}
	}()

	var payload protocol.InvokePayload
	if err := msg.ParsePayload(&payload); err != nil {
		t.sendError(msg.RequestID, "invalid invoke payload")
		return
	}

	skill, ok := t.skills[payload.Skill]
	if !ok {
		t.sendResult(msg.RequestID, protocol.ResultPayload{
			Status: "failed",
			Error:  fmt.Sprintf("skill '%s' not found", payload.Skill),
		})
		return
	}

	// 输入校验：按 Skill 定义的 Input Schema 校验必填字段和类型
	input := NewInput(payload.Input)
	if skill.Input != nil {
		if err := skill.Input.Validate(input.Raw()); err != nil {
			t.sendResult(msg.RequestID, protocol.ResultPayload{
				Status: "failed",
				Error:  fmt.Sprintf("input validation failed: %s", err.Error()),
			})
			return
		}
	}

	ctx := Context{
		Context:   nil, // TODO: add timeout context
		RequestID: msg.RequestID,
		Caller:    payload.Caller,
	}

	output, err := skill.Handler(ctx, input)
	if err != nil {
		t.sendResult(msg.RequestID, protocol.ResultPayload{
			Status: "failed",
			Error:  err.Error(),
		})
		return
	}

	// 检查是否为多轮对话追问
	if needInput, ok := output.(*NeedInputResult); ok {
		t.sendNeedInput(msg.RequestID, needInput.Question)
		return
	}

	outputBytes, _ := json.Marshal(output)
	t.sendResult(msg.RequestID, protocol.ResultPayload{
		Status: "completed",
		Output: outputBytes,
	})
}

// handleReply 处理来自 Gateway 的多轮对话回复消息。
//
// 当用户回复了 Agent 的追问后，Gateway 将回复通过 TypeReply 消息转发到 Agent。
// 框架解析回复中的合并输入，重新调用 Skill Handler。
// Handler 可能返回最终结果（发 result）或继续追问（再发 need_input）。
//
// 参数：
//   - msg: 来自 Gateway 的回复消息，包含 RequestID 和 ReplyPayload
func (t *TunnelClient) handleReply(msg protocol.Message) {
	defer t.wg.Done()

	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Reply handler panic (request=%s): %v", msg.RequestID, r)
			t.sendResult(msg.RequestID, protocol.ResultPayload{
				Status: "failed",
				Error:  fmt.Sprintf("internal error: handler panic: %v", r),
			})
		}
	}()

	var payload protocol.ReplyPayload
	if err := msg.ParsePayload(&payload); err != nil {
		t.sendError(msg.RequestID, "invalid reply payload")
		return
	}

	skill, ok := t.skills[payload.Skill]
	if !ok {
		t.sendResult(msg.RequestID, protocol.ResultPayload{
			Status: "failed",
			Error:  fmt.Sprintf("skill '%s' not found", payload.Skill),
		})
		return
	}

	input := NewInput(payload.Input)
	if skill.Input != nil {
		if err := skill.Input.Validate(input.Raw()); err != nil {
			t.sendResult(msg.RequestID, protocol.ResultPayload{
				Status: "failed",
				Error:  fmt.Sprintf("input validation failed: %s", err.Error()),
			})
			return
		}
	}

	ctx := Context{
		Context:   nil,
		RequestID: msg.RequestID,
		Caller:    payload.Caller,
	}

	output, err := skill.Handler(ctx, input)
	if err != nil {
		t.sendResult(msg.RequestID, protocol.ResultPayload{
			Status: "failed",
			Error:  err.Error(),
		})
		return
	}

	if needInput, ok := output.(*NeedInputResult); ok {
		t.sendNeedInput(msg.RequestID, needInput.Question)
		return
	}

	outputBytes, _ := json.Marshal(output)
	t.sendResult(msg.RequestID, protocol.ResultPayload{
		Status: "completed",
		Output: outputBytes,
	})
}

// sendResult 向 Gateway 发送 Skill 执行结果消息。
//
// 该方法将执行结果封装为 TypeResult 类型的协议消息并通过 WebSocket 发送。
// 通过互斥锁保证并发写入的安全性。
//
// 参数：
//   - requestID: 对应调用请求的唯一标识符，用于关联请求和响应
//   - result: 执行结果载荷，包含状态（completed/failed）、输出数据或错误信息
func (t *TunnelClient) sendResult(requestID string, result protocol.ResultPayload) {
	msg, _ := protocol.NewMessage(protocol.TypeResult, requestID, result)
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.conn.WriteJSON(msg); err != nil {
		logger.Errorf("Failed to send result: %v", err)
	}
}

// sendError 向 Gateway 发送 Skill 执行失败的结果消息。
//
// 该方法是 sendResult 的便捷封装，用于快速发送状态为 "failed" 的错误响应。
//
// 参数：
//   - requestID: 对应调用请求的唯一标识符
//   - errMsg: 错误描述信息
func (t *TunnelClient) sendError(requestID string, errMsg string) {
	t.sendResult(requestID, protocol.ResultPayload{Status: "failed", Error: errMsg})
}

// sendNeedInput 向 Gateway 发送多轮对话追问消息。
//
// 当 Skill Handler 返回 NeedInputResult 时，框架调用此方法将追问请求
// 发送给 Gateway，Gateway 会转发给调用方以获取额外输入。
//
// 参数：
//   - requestID: 对应调用请求的唯一标识符
//   - question: 追问的问题定义，包含字段名、提示文本和可选选项
func (t *TunnelClient) sendNeedInput(requestID string, question protocol.Question) {
	payload := protocol.NeedInputPayload{
		TaskID:   requestID,
		Question: question,
	}
	msg, _ := protocol.NewMessage(protocol.TypeNeedInput, requestID, payload)
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.conn.WriteJSON(msg); err != nil {
		logger.Errorf("Failed to send need_input: %v", err)
	}
}

// sendPing 向 Gateway 发送 Ping 心跳消息。
//
// 心跳消息用于保持 WebSocket 连接活跃，防止被中间代理或 Gateway 因超时断开。
// 通过互斥锁保证并发写入的安全性。
func (t *TunnelClient) sendPing() {
	msg, _ := protocol.NewMessage(protocol.TypePing, "", nil)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.conn.WriteJSON(msg)
}

// sendPong 向 Gateway 发送 Pong 心跳响应消息。
//
// 当收到 Gateway 发送的 Ping 消息时，回复 Pong 以确认连接正常。
// 通过互斥锁保证并发写入的安全性。
func (t *TunnelClient) sendPong() {
	msg, _ := protocol.NewMessage(protocol.TypePong, "", nil)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.conn.WriteJSON(msg)
}

// ConnectWithRetry 以指数退避策略连接 Gateway，连接断开后自动重连。
//
// 重连策略：
//   - 前 3 次重连：每次间隔 1 秒
//   - 第 4 次起：使用指数退避，初始 1 秒，每次翻倍，最大 60 秒
//   - 连接成功后重置退避计数器
//   - 连接成功但后来断开时，自动发起重连
//
// 该方法为阻塞调用，仅在 done 通道被关闭（如收到系统终止信号）时返回。
func (t *TunnelClient) ConnectWithRetry() {
	backoff := time.Second
	maxBackoff := 60 * time.Second
	attempt := 0

	for {
		err := t.Connect()
		if err == nil {
			t.Wait()
			// 连接曾建立但后来断开
			select {
			case <-t.done:
				return
			default:
			}
			// 重置状态准备重连
			t.done = make(chan struct{})
			backoff = time.Second
			attempt = 0
			logger.Info("Tunnel disconnected, reconnecting...")
			continue
		}

		attempt++
		logger.Errorf("Tunnel connect failed (attempt %d): %v", attempt, err)

		if attempt <= 3 {
			time.Sleep(time.Second)
		} else {
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}
