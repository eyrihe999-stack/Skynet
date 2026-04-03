package protocol

import "encoding/json"

// AgentCard 描述一个 Agent 的完整能力卡片信息。
//
// AgentCard 是 Skynet 平台中 Agent 自我描述的核心数据结构，
// 由框架根据 skynet.yaml 配置文件和技能定义自动生成。
// Agent 在注册时将此卡片提交给 Gateway，Gateway 据此完成 Agent 的注册、
// 能力发现和路由等功能。
//
// 字段说明：
//   - AgentID: Agent 的全局唯一标识符，在整个 Skynet 网络中唯一。
//   - OwnerAPIKey: Agent 所有者的 API 密钥，仅在注册时发送用于身份验证，不会被持久化存储。
//   - DisplayName: Agent 的显示名称，用于在平台界面中展示。
//   - Description: Agent 的功能描述，帮助用户和其他 Agent 了解其用途。
//   - Version: Agent 的版本号，遵循语义化版本规范。
//   - FrameworkVersion: Agent 所使用的 Skynet 框架版本号。
//   - ConnectionMode: Agent 的连接模式，如 "tunnel"（WebSocket 隧道）等。
//   - DataPolicy: 数据处理策略声明，描述 Agent 如何处理调用方数据。可为 nil 表示使用默认策略。
//   - Capabilities: Agent 暴露的技能列表，每个元素描述一个可被调用的技能。
type AgentCard struct {
	AgentID          string          `json:"agent_id"`
	OwnerAPIKey      string          `json:"owner_api_key,omitempty"` // 仅在注册时发送，不被持久化存储
	DisplayName      string          `json:"display_name"`
	Description      string          `json:"description"`
	Version          string          `json:"version"`
	FrameworkVersion string          `json:"framework_version"`
	ConnectionMode   string          `json:"connection_mode"`
	DataPolicy       *DataPolicy     `json:"data_policy,omitempty"`
	Capabilities     []CapabilityDef `json:"capabilities"`
}

// CapabilityDef 描述 Agent 暴露的单个技能（Skill）的完整定义。
//
// 每个 CapabilityDef 代表 Agent 可以被外部调用的一项能力。
// Gateway 使用这些定义进行技能发现、输入校验和调用路由。
//
// 字段说明：
//   - Name: 技能的唯一名称（在同一 Agent 内唯一），用于调用时的技能标识。
//   - DisplayName: 技能的显示名称，用于在平台界面中展示。
//   - Description: 技能的功能描述，帮助调用方了解技能的用途和行为。
//   - Category: 技能所属分类，用于技能的组织和检索。
//   - Tags: 技能的标签列表，用于更灵活的分类和搜索。
//   - InputSchema: 技能输入参数的 JSON Schema 定义，用于调用时的参数校验。
//   - OutputSchema: 技能输出结果的 JSON Schema 定义，用于结果格式的描述。
//   - Visibility: 技能的可见性，如 "public"（公开）或 "private"（私有）等。
//   - ApprovalMode: 调用审批模式，如 "auto"（自动通过）或 "manual"（需人工审批）等。
//   - MultiTurn: 是否支持多轮对话交互。true 表示技能支持上下文连续对话。
//   - EstimatedLatencyMs: 技能执行的预估延迟时间（毫秒），帮助调用方设置合理的超时时间。
type CapabilityDef struct {
	Name               string          `json:"name"`
	DisplayName        string          `json:"display_name"`
	Description        string          `json:"description"`
	Category           string          `json:"category"`
	Tags               []string        `json:"tags,omitempty"`
	InputSchema        json.RawMessage `json:"input_schema"`
	OutputSchema       json.RawMessage `json:"output_schema,omitempty"`
	Visibility         string          `json:"visibility"`
	ApprovalMode       string          `json:"approval_mode"`
	MultiTurn          bool            `json:"multi_turn"`
	EstimatedLatencyMs uint            `json:"estimated_latency_ms,omitempty"`
}

// DataPolicy 声明 Agent 对调用方数据的处理策略。
//
// 在 Skynet 平台中，调用方的输入和输出数据可能包含敏感信息。
// DataPolicy 允许 Agent 明确声明其数据处理方式，
// 帮助调用方在调用前评估数据安全风险。
//
// 字段说明：
//   - StoreInput: 是否存储调用方的输入数据。true 表示 Agent 会持久化存储输入。
//   - StoreOutput: 是否存储技能执行的输出数据。true 表示 Agent 会持久化存储输出。
//   - RetentionHours: 数据保留时长（小时）。超过该时长后数据将被删除。0 表示不保留。
type DataPolicy struct {
	StoreInput     bool `json:"store_input"`
	StoreOutput    bool `json:"store_output"`
	RetentionHours int  `json:"retention_hours"`
}
