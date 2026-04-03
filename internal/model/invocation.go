package model

import "time"

// Invocation 表示一次能力调用记录，对应数据库 invocations 表。
// 每当一个 Agent 或用户发起对某个 Capability 的调用时，平台会创建一条 Invocation 记录，
// 用于追踪调用的完整生命周期（从提交到完成/失败）以及性能指标（延迟等）。
// store 层 InvocationRepository 基于该模型进行调用记录的创建、查询和状态更新。
type Invocation struct {
	// ID 是调用记录的自增主键，由数据库自动生成。
	// gorm:"primaryKey;autoIncrement" 指定为 GORM 主键并启用自增。
	// json:"id" 序列化为 JSON 时字段名为 "id"。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`

	// TaskID 是调用任务的全局唯一标识符（UUID 格式，36 字符），由平台在创建调用时生成。
	// 用于跨系统追踪和幂等性保证。
	// gorm:"type:varchar(36);uniqueIndex;not null" 限制长度 36，建立唯一索引，不允许为空。
	// json:"task_id" 序列化为 JSON 时字段名为 "task_id"。
	TaskID string `gorm:"type:varchar(36);uniqueIndex;not null" json:"task_id"`

	// CallerAgentID 是发起调用的 Agent 的唯一标识符。
	// 当调用由另一个 Agent 发起时填充此字段（Agent-to-Agent 调用场景）。
	// gorm:"type:varchar(100);index:idx_caller_agent" 限制最大长度 100，
	// 建立复合索引 idx_caller_agent（与 CreatedAt 共同索引）以加速按调用方 Agent 查询。
	// json:"caller_agent_id,omitempty" 序列化为 JSON 时字段名为 "caller_agent_id"，为空时省略。
	CallerAgentID string `gorm:"type:varchar(100);index:idx_caller_agent" json:"caller_agent_id,omitempty"`

	// CallerUserID 是发起调用的用户 ID。
	// 当调用由用户直接发起时填充此字段（User-to-Agent 调用场景）。
	// 使用指针类型以支持 NULL 值（Agent-to-Agent 调用时为 nil）。
	// gorm:"index:idx_caller_user" 建立复合索引 idx_caller_user（与 CreatedAt 共同索引）以加速按调用方用户查询。
	// json:"caller_user_id,omitempty" 序列化为 JSON 时字段名为 "caller_user_id"，为空时省略。
	CallerUserID *uint64 `gorm:"index:idx_caller_user" json:"caller_user_id,omitempty"`

	// TargetAgentID 是被调用的目标 Agent 的唯一标识符。
	// gorm:"type:varchar(100);not null;index:idx_target" 限制最大长度 100，不允许为空，
	// 建立复合索引 idx_target（与 CreatedAt 共同索引）以加速按目标 Agent 查询。
	// json:"target_agent_id" 序列化为 JSON 时字段名为 "target_agent_id"。
	TargetAgentID string `gorm:"type:varchar(100);not null;index:idx_target" json:"target_agent_id"`

	// SkillName 是被调用的能力/Skill 名称，对应 Capability.Name。
	// gorm:"type:varchar(100);not null" 限制最大长度 100，不允许为空。
	// json:"skill_name" 序列化为 JSON 时字段名为 "skill_name"。
	SkillName string `gorm:"type:varchar(100);not null" json:"skill_name"`

	// InputRef 是调用输入数据的外部引用地址（如对象存储路径）。
	// gorm:"type:varchar(512)" 限制最大长度 512。
	// json:"input_ref,omitempty" 序列化为 JSON 时字段名为 "input_ref"，为空时省略。
	InputRef string `gorm:"type:varchar(512)" json:"input_ref,omitempty"`

	// OutputRef 是调用输出数据的外部引用地址（如对象存储路径）。
	// gorm:"type:varchar(512)" 限制最大长度 512。
	// json:"output_ref,omitempty" 序列化为 JSON 时字段名为 "output_ref"，为空时省略。
	OutputRef string `gorm:"type:varchar(512)" json:"output_ref,omitempty"`

	// CallChain 记录本次调用涉及的 Agent 调用链路，以 JSON 数组形式存储。
	// gorm:"type:json" 使用 MySQL JSON 列类型。
	// json:"call_chain,omitempty" 序列化为 JSON 时字段名为 "call_chain"，为空时省略。
	CallChain JSONArray `gorm:"type:json" json:"call_chain,omitempty"`

	// Status 表示调用的当前状态，跟踪调用的完整生命周期。
	// 取值范围及含义：
	//   - "submitted"：已提交，等待分配（默认初始状态）
	//   - "assigned"：已分配给目标 Agent，等待处理
	//   - "working"：目标 Agent 正在处理中
	//   - "input_required"：等待调用方提供额外输入（多轮交互场景）
	//   - "completed"：调用成功完成
	//   - "failed"：调用失败
	//   - "cancelled"：调用被取消
	// gorm:"type:enum('submitted','assigned','working','input_required','completed','failed','cancelled');default:'submitted';not null"
	// 使用 MySQL 枚举类型，默认 "submitted"。
	// json:"status" 序列化为 JSON 时字段名为 "status"。
	Status string `gorm:"type:enum('submitted','assigned','working','input_required','completed','failed','cancelled');default:'submitted';not null" json:"status"`

	// Mode 表示调用的执行模式。
	// 取值范围：
	//   - "sync"：同步调用（默认），调用方阻塞等待结果返回
	//   - "async"：异步调用，调用方提交后立即返回 TaskID，后续通过轮询或回调获取结果
	// gorm:"type:enum('sync','async');default:'sync';not null" 使用 MySQL 枚举类型，默认 "sync"。
	// json:"mode" 序列化为 JSON 时字段名为 "mode"。
	Mode string `gorm:"type:enum('sync','async');default:'sync';not null" json:"mode"`

	// ErrorMessage 记录调用失败时的错误信息。
	// 仅在 Status 为 "failed" 时有意义。
	// gorm:"type:text" 使用 TEXT 类型，允许较长的错误描述。
	// json:"error_message,omitempty" 序列化为 JSON 时字段名为 "error_message"，为空时省略。
	ErrorMessage string `gorm:"type:text" json:"error_message,omitempty"`

	// LatencyMs 是调用的端到端延迟（毫秒），从提交到完成的耗时。
	// 使用指针类型以支持 NULL 值（调用尚未完成时为 nil）。
	// json:"latency_ms,omitempty" 序列化为 JSON 时字段名为 "latency_ms"，为空时省略。
	LatencyMs *uint `json:"latency_ms,omitempty"`

	// CreatedAt 是调用记录的创建时间（即调用提交时间），由 GORM 自动填充。
	// gorm:"autoCreateTime:milli" 在插入时自动设置为当前时间（毫秒精度）。
	// 该字段同时参与 idx_caller_agent、idx_caller_user、idx_target 三个复合索引，
	// 支持按时间范围过滤和排序。
	// json:"created_at" 序列化为 JSON 时字段名为 "created_at"。
	CreatedAt time.Time `gorm:"autoCreateTime:milli;index:idx_caller_agent;index:idx_caller_user;index:idx_target" json:"created_at"`

	// CompletedAt 是调用完成（成功或失败）的时间。
	// 使用指针类型以支持 NULL 值（调用尚未完成时为 nil）。
	// json:"completed_at,omitempty" 序列化为 JSON 时字段名为 "completed_at"，为空时省略。
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
