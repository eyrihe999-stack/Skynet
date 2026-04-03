package protocol

// RegisterPayload 是 Agent 注册请求的载荷结构。
//
// 当 Agent 通过 WebSocket 隧道连接到 Gateway 时，
// 会发送类型为 TypeRegister 的消息，其载荷即为此结构。
// Agent 通过此载荷将自身的能力卡片（AgentCard）提交给 Gateway 进行注册。
//
// 字段说明：
//   - Card: Agent 能力卡片，包含 Agent 的标识、描述、版本、技能列表等完整信息。
type RegisterPayload struct {
	Card AgentCard `json:"card"`
}

// RegisteredPayload 是 Gateway 对 Agent 注册请求的响应载荷结构。
//
// Gateway 收到 Agent 的注册请求后，处理完成后会发送类型为 TypeRegistered 的消息，
// 其载荷即为此结构，用于告知 Agent 注册是否成功。
//
// 字段说明：
//   - Success: 注册是否成功的标志。true 表示注册成功，false 表示注册失败。
//   - AgentSecret: Agent 密钥，仅在首次注册时由 Gateway 生成并返回。
//     Agent 应妥善保存此密钥，后续重连时用于身份验证。
//   - Error: 当注册失败时，包含具体的错误描述信息。注册成功时为空。
type RegisteredPayload struct {
	Success     bool   `json:"success"`
	AgentSecret string `json:"agent_secret,omitempty"` // 仅在首次注册时返回
	Error       string `json:"error,omitempty"`
}
