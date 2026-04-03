package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Capability 表示 Agent 注册到平台上的一项能力（Skill），对应数据库 capabilities 表。
// 能力是 Skynet 网络中可被发现和调用的最小功能单元。其他 Agent 或用户可以通过
// 平台发现某个 Agent 的 Capability 并发起 Invocation 调用。
// 每个 Capability 通过 AgentID + Name 的联合唯一索引保证同一 Agent 下不出现重名能力。
// store 层 CapabilityRepository 基于该模型进行能力管理和检索。
type Capability struct {
	// ID 是能力记录的自增主键，由数据库自动生成。
	// gorm:"primaryKey;autoIncrement" 指定为 GORM 主键并启用自增。
	// json:"id" 序列化为 JSON 时字段名为 "id"。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`

	// AgentID 是该能力所属 Agent 的唯一标识符，关联 agents 表的 agent_id 字段。
	// gorm:"type:varchar(100);not null;uniqueIndex:uk_agent_skill" 限制最大长度 100，
	// 不允许为空，与 Name 共同组成联合唯一索引 uk_agent_skill。
	// json:"agent_id" 序列化为 JSON 时字段名为 "agent_id"。
	AgentID string `gorm:"type:varchar(100);not null;uniqueIndex:uk_agent_skill" json:"agent_id"`

	// Name 是能力的程序化名称（如 "get_weather"），用于 API 调用时的标识。
	// gorm:"type:varchar(100);not null;uniqueIndex:uk_agent_skill" 限制最大长度 100，
	// 不允许为空，与 AgentID 共同组成联合唯一索引 uk_agent_skill。
	// json:"name" 序列化为 JSON 时字段名为 "name"。
	Name string `gorm:"type:varchar(100);not null;uniqueIndex:uk_agent_skill" json:"name"`

	// DisplayName 是能力的人类可读显示名称，用于界面展示。
	// gorm:"type:varchar(200);not null" 限制最大长度 200，不允许为空。
	// json:"display_name" 序列化为 JSON 时字段名为 "display_name"。
	DisplayName string `gorm:"type:varchar(200);not null" json:"display_name"`

	// Description 是能力的详细描述信息，说明该能力的功能、用途和使用场景。
	// gorm:"type:text" 使用 TEXT 类型，允许较长文本。
	// json:"description" 序列化为 JSON 时字段名为 "description"。
	Description string `gorm:"type:text" json:"description"`

	// Category 是能力的分类标签，用于能力市场的分组展示和筛选。
	// 默认值为 "general"。
	// gorm:"type:varchar(50);default:'general';index" 限制最大长度 50，建立普通索引以加速按分类查询。
	// json:"category" 序列化为 JSON 时字段名为 "category"。
	Category string `gorm:"type:varchar(50);default:'general';index" json:"category"`

	// Tags 是能力的标签列表，以 JSON 字符串数组形式存储，用于灵活的多维检索。
	// gorm:"type:json" 使用 MySQL JSON 列类型。
	// json:"tags,omitempty" 序列化为 JSON 时字段名为 "tags"，为空时省略。
	Tags JSONArray `gorm:"type:json" json:"tags,omitempty"`

	// InputSchema 是能力输入参数的 JSON Schema 定义，描述调用时需要传入的参数结构。
	// 调用方据此构造请求体，平台可据此进行参数校验。
	// gorm:"type:json;not null" 使用 MySQL JSON 列类型，不允许为空。
	// json:"input_schema" 序列化为 JSON 时字段名为 "input_schema"。
	InputSchema JSONRaw `gorm:"type:json;not null" json:"input_schema"`

	// OutputSchema 是能力输出结果的 JSON Schema 定义，描述返回值的结构。
	// gorm:"type:json" 使用 MySQL JSON 列类型，允许为空（部分能力可能不定义输出 Schema）。
	// json:"output_schema,omitempty" 序列化为 JSON 时字段名为 "output_schema"，为空时省略。
	OutputSchema JSONRaw `gorm:"type:json" json:"output_schema,omitempty"`

	// Visibility 控制能力的可见性/访问范围。
	// 取值范围：
	//   - "public"：公开，所有人可见可调用（默认）
	//   - "restricted"：受限，仅获授权的调用方可调用
	//   - "private"：私有，仅能力所属 Agent 的 Owner 可调用
	// gorm:"type:enum('public','restricted','private');default:'public';not null" 使用 MySQL 枚举类型。
	// json:"visibility" 序列化为 JSON 时字段名为 "visibility"。
	Visibility string `gorm:"type:enum('public','restricted','private');default:'public';not null" json:"visibility"`

	// ApprovalMode 控制调用请求的审批方式。
	// 取值范围：
	//   - "auto"：自动批准，调用请求直接执行（默认）
	//   - "manual"：手动批准，需要 Agent Owner 确认后才执行
	// gorm:"type:enum('auto','manual');default:'auto';not null" 使用 MySQL 枚举类型。
	// json:"approval_mode" 序列化为 JSON 时字段名为 "approval_mode"。
	ApprovalMode string `gorm:"type:enum('auto','manual');default:'auto';not null" json:"approval_mode"`

	// MultiTurn 表示该能力是否支持多轮交互。
	// 若为 true，则调用方可在一次会话中与该能力进行多次请求-响应交互。
	// gorm:"default:false;not null" 默认值为 false。
	// json:"multi_turn" 序列化为 JSON 时字段名为 "multi_turn"。
	MultiTurn bool `gorm:"default:false;not null" json:"multi_turn"`

	// EstimatedLatencyMs 是该能力预估的响应延迟（毫秒），供调用方参考。
	// 使用指针类型以支持 NULL 值（未设置预估延迟时为 nil）。
	// json:"estimated_latency_ms,omitempty" 序列化为 JSON 时字段名为 "estimated_latency_ms"，为空时省略。
	EstimatedLatencyMs *uint `json:"estimated_latency_ms,omitempty"`

	// CallCount 是该能力被调用的总次数，由平台在每次调用完成后自增。
	// gorm:"default:0;not null" 默认值为 0。
	// json:"call_count" 序列化为 JSON 时字段名为 "call_count"。
	CallCount uint64 `gorm:"default:0;not null" json:"call_count"`

	// SuccessCount 是该能力调用成功的次数，配合 CallCount 可计算成功率。
	// gorm:"default:0;not null" 默认值为 0。
	// json:"success_count" 序列化为 JSON 时字段名为 "success_count"。
	SuccessCount uint64 `gorm:"default:0;not null" json:"success_count"`

	// TotalLatencyMs 是该能力所有调用的累计延迟（毫秒），配合 CallCount 可计算平均延迟。
	// gorm:"default:0;not null" 默认值为 0。
	// json:"-" 该字段不暴露给 API 客户端，仅用于内部统计计算。
	TotalLatencyMs uint64 `gorm:"default:0;not null" json:"-"`

	// CreatedAt 是能力记录的创建时间，由 GORM 自动填充。
	// gorm:"autoCreateTime:milli" 在插入时自动设置为当前时间（毫秒精度）。
	// json:"created_at" 序列化为 JSON 时字段名为 "created_at"。
	CreatedAt time.Time `gorm:"autoCreateTime:milli" json:"created_at"`

	// UpdatedAt 是能力记录的最后更新时间，由 GORM 自动维护。
	// gorm:"autoUpdateTime:milli" 在每次更新时自动设置为当前时间（毫秒精度）。
	// json:"updated_at" 序列化为 JSON 时字段名为 "updated_at"。
	UpdatedAt time.Time `gorm:"autoUpdateTime:milli" json:"updated_at"`
}

// JSONArray 是一个可被 GORM 序列化/反序列化为 JSON 的字符串切片类型。
// 底层类型为 []string，适用于存储标签列表等简单数组（如 Capability 的 Tags 字段）。
// 实现了 driver.Valuer 和 sql.Scanner 接口以支持 GORM 的自动读写。
type JSONArray []string

// Value 实现 driver.Valuer 接口，将 JSONArray 序列化为 JSON 字节数组以写入数据库。
//
// 返回值：
//   - driver.Value：JSON 编码后的 []byte（如 ["tag1","tag2"]），或 nil（当 JSONArray 本身为 nil 时）。
//   - error：JSON 编码过程中的错误，正常情况下为 nil。
func (j JSONArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口，将数据库中的 JSON 字节数据反序列化为 JSONArray。
//
// 参数：
//   - value：数据库驱动返回的原始值，预期为 []byte 类型或 nil。
//
// 返回值：
//   - error：JSON 反序列化过程中的错误，正常情况下为 nil。
//     当 value 为 nil 时，JSONArray 被置为 nil；当 value 不是 []byte 类型时静默忽略。
func (j *JSONArray) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// JSONRaw 是一个可被 GORM 序列化/反序列化的原始 JSON 值类型。
// 底层类型为 json.RawMessage（即 []byte），适用于存储结构不固定的 JSON 数据
// （如 Capability 的 InputSchema 和 OutputSchema），在读写时不进行结构化解析，
// 保持原始 JSON 原貌。
// 实现了 driver.Valuer、sql.Scanner、json.Marshaler 和 json.Unmarshaler 接口。
type JSONRaw json.RawMessage

// Value 实现 driver.Valuer 接口，将 JSONRaw 的原始字节数据写入数据库。
//
// 返回值：
//   - driver.Value：原始 JSON 字节数据的 []byte 副本，或 nil（当 JSONRaw 本身为 nil 时）。
//   - error：始终为 nil。
func (j JSONRaw) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return []byte(j), nil
}

// Scan 实现 sql.Scanner 接口，将数据库中的原始字节数据复制到 JSONRaw。
//
// 参数：
//   - value：数据库驱动返回的原始值，预期为 []byte 类型或 nil。
//
// 返回值：
//   - error：始终为 nil。
//     当 value 为 nil 时，JSONRaw 被置为 nil；当 value 不是 []byte 类型时静默忽略。
func (j *JSONRaw) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	*j = make(JSONRaw, len(bytes))
	copy(*j, bytes)
	return nil
}

// MarshalJSON 实现 json.Marshaler 接口，在 JSON 序列化时直接输出原始 JSON 内容。
// 这保证了存储在数据库中的 JSON 数据在 API 响应中原样输出，不会被二次编码。
//
// 返回值：
//   - []byte：原始 JSON 字节数据，或 "null"（当 JSONRaw 本身为 nil 时）。
//   - error：始终为 nil。
func (j JSONRaw) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON 实现 json.Unmarshaler 接口，在 JSON 反序列化时直接存储原始字节数据。
// 不对 JSON 内容进行任何结构化解析，保持数据原样。
//
// 参数：
//   - data：待反序列化的原始 JSON 字节数据。
//
// 返回值：
//   - error：始终为 nil。
func (j *JSONRaw) UnmarshalJSON(data []byte) error {
	*j = make(JSONRaw, len(data))
	copy(*j, data)
	return nil
}
