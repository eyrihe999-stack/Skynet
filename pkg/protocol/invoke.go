package protocol

import "encoding/json"

// InvokePayload 是技能调用请求的载荷结构。
//
// 当外部调用方（其他 Agent 或用户）需要调用某个 Agent 的技能时，
// Gateway 会通过 WebSocket 隧道向目标 Agent 发送类型为 TypeInvoke 的消息，
// 其载荷即为此结构。Agent 收到后应执行对应技能并返回结果。
//
// 字段说明：
//   - Skill: 要调用的技能名称，对应 AgentCard.Capabilities 中某个 CapabilityDef 的 Name。
//   - Input: 技能调用的输入参数，为原始 JSON 数据，具体结构由技能的 InputSchema 定义。
//   - Caller: 调用方信息，标识发起此次调用的 Agent 或用户。
//   - TimeoutMs: 调用超时时间（毫秒），Agent 应在此时间内完成技能执行并返回结果。
type InvokePayload struct {
	Skill     string          `json:"skill"`
	Input     json.RawMessage `json:"input"`
	Caller    CallerInfo      `json:"caller"`
	TimeoutMs int             `json:"timeout_ms"`
	CallChain []string        `json:"call_chain,omitempty"`
}

// ResultPayload 是技能调用结果的载荷结构。
//
// Agent 完成技能执行后，会通过 WebSocket 隧道向 Gateway 发送类型为 TypeResult 的消息，
// 其载荷即为此结构。Gateway 收到后会将结果转发给原始调用方。
//
// 字段说明：
//   - Status: 执行状态，取值为 "completed"（执行成功）或 "failed"（执行失败）。
//   - Output: 技能执行的输出结果，为原始 JSON 数据，具体结构由技能的 OutputSchema 定义。
//     仅在 Status 为 "completed" 时有值。
//   - Error: 错误描述信息，仅在 Status 为 "failed" 时有值。
type ResultPayload struct {
	Status string          `json:"status"` // "completed" 或 "failed"
	Output json.RawMessage `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// CallerInfo 标识发起技能调用的调用方信息。
//
// 在 Skynet 平台中，技能调用可能由其他 Agent 或人类用户发起。
// CallerInfo 提供调用方的身份标识，便于被调用的 Agent 进行权限校验、
// 日志记录或个性化处理。
//
// 字段说明：
//   - AgentID: 调用方 Agent 的唯一标识符。当调用方为另一个 Agent 时有值，用户直接调用时为空。
//   - UserID: 调用方用户的唯一标识符。当调用方为人类用户时有值。
//   - DisplayName: 调用方的显示名称，用于日志展示或界面呈现。
type CallerInfo struct {
	AgentID     string `json:"agent_id,omitempty"`
	UserID      uint64 `json:"user_id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

type NeedInputPayload struct {
	TaskID   string   `json:"task_id"`
	Question Question `json:"question"`
}

type Question struct {
	Field   string   `json:"field"`
	Prompt  string   `json:"prompt"`
	Options []string `json:"options,omitempty"`
}

type ReplyPayload struct {
	TaskID string          `json:"task_id"`
	Skill  string          `json:"skill"`
	Input  json.RawMessage `json:"input"`
	Caller CallerInfo      `json:"caller"`
}

type ProgressPayload struct {
	TaskID   string  `json:"task_id"`
	Progress float64 `json:"progress"`
	Message  string  `json:"message,omitempty"`
}

// InvokeResult 是 Agent 对调用/回复的响应结果，可能是最终结果或追问。
// Gateway 的 AgentConn 使用此类型统一处理 TypeResult 和 TypeNeedInput 两种响应。
type InvokeResult struct {
	Type      string            `json:"type"`       // "result" 或 "need_input"
	Result    *ResultPayload    `json:"result,omitempty"`
	NeedInput *NeedInputPayload `json:"need_input,omitempty"`
}
