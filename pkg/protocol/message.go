// Package protocol 定义了 Skynet 平台中 Agent 与 Gateway 之间通过 WebSocket 隧道通信的消息协议。
//
// 所有通信消息都通过 Message 信封结构进行封装，使用 JSON 序列化。
// 消息类型（Type）决定了载荷（Payload）的具体结构，
// 各类载荷定义分布在 register.go、invoke.go 等文件中。
package protocol

import "encoding/json"

// WebSocket 隧道中交换的消息类型常量。
// 这些常量用于 Message.Type 字段，标识消息的用途和方向。
const (
	// TypeRegister 表示注册请求消息，由 Agent 发送给 Gateway，
	// 携带 Agent 的能力卡片信息以完成注册。
	TypeRegister = "register"

	// TypeRegistered 表示注册响应消息，由 Gateway 发送给 Agent，
	// 确认注册是否成功，并在首次注册时返回 Agent Secret。
	TypeRegistered = "registered"

	// TypeInvoke 表示技能调用请求消息，由 Gateway 发送给 Agent，
	// 请求 Agent 执行某个特定技能（Skill）。
	TypeInvoke = "invoke"

	// TypeResult 表示技能调用结果消息，由 Agent 发送给 Gateway，
	// 返回技能执行的结果或错误信息。
	TypeResult = "result"

	// TypePing 表示心跳探测消息，用于检测 WebSocket 连接是否存活。
	TypePing = "ping"

	// TypePong 表示心跳响应消息，作为对 TypePing 的回复，
	// 确认连接仍然存活。
	TypePong = "pong"

	// TypeError 表示错误消息，用于在通信过程中传递错误信息。
	TypeError = "error"

	// TypeNeedInput 表示多轮对话追问消息，由 Agent 发送给 Gateway，
	// 表示 Agent 需要更多输入信息才能完成技能执行。
	TypeNeedInput = "need_input"

	// TypeReply 表示多轮对话回复消息，由 Gateway 发送给 Agent，
	// 包含调用方对追问的回复输入。
	TypeReply = "reply"

	// TypeProgress 表示进度更新消息，由 Agent 发送给 Gateway，
	// 报告异步任务的执行进度。
	TypeProgress = "progress"
)

// Message 是 WebSocket 隧道通信的统一消息信封结构。
// 所有在 Agent 与 Gateway 之间传递的消息都封装在此结构中。
//
// 字段说明：
//   - Type: 消息类型，取值为上述常量之一（如 "register"、"invoke" 等），决定了 Payload 的解析方式。
//   - RequestID: 请求唯一标识符，用于将请求与响应进行关联匹配。对于无需关联的消息可为空。
//   - Payload: 消息载荷的原始 JSON 数据，具体结构由 Type 字段决定。
type Message struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// NewMessage 创建一个带有指定类型和载荷的消息信封。
//
// 该函数将 payload 序列化为 JSON，并封装进 Message 结构中。
// 是构造所有类型消息的通用工厂方法。
//
// 参数：
//   - typ: 消息类型字符串，应使用本包定义的类型常量（如 TypeRegister、TypeInvoke 等）。
//   - requestID: 请求唯一标识符，用于请求-响应的关联匹配。
//   - payload: 消息载荷对象，将被 JSON 序列化后存入 Message.Payload。可传 nil 表示无载荷。
//
// 返回值：
//   - *Message: 构造好的消息信封指针。
//   - error: 如果 payload 序列化失败则返回错误，否则为 nil。
func NewMessage(typ string, requestID string, payload any) (*Message, error) {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	return &Message{Type: typ, RequestID: requestID, Payload: raw}, nil
}

// ParsePayload 将消息中的原始 JSON 载荷反序列化到指定的目标对象中。
//
// 调用方需根据 Message.Type 选择正确的目标类型进行解析，
// 例如 TypeRegister 对应 RegisterPayload，TypeInvoke 对应 InvokePayload 等。
//
// 参数：
//   - target: 目标对象的指针，载荷将被反序列化到该对象中。
//
// 返回值：
//   - error: 如果 JSON 反序列化失败则返回错误，否则为 nil。
func (m *Message) ParsePayload(target any) error {
	return json.Unmarshal(m.Payload, target)
}
