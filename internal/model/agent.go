package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Agent 表示 Skynet 平台上注册的智能体（Agent），对应数据库 agents 表。
// Agent 是 Skynet 网络的核心节点，可以注册能力（Capability）、接收调用请求、
// 与其他 Agent 协作。每个 Agent 归属于一个 User（Owner），并通过 AgentID 在
// 全平台范围内唯一标识。store 层 AgentRepository 基于该模型进行 Agent 管理。
type Agent struct {
	// ID 是 Agent 记录的自增主键，由数据库自动生成，仅用于内部关联。
	// gorm:"primaryKey;autoIncrement" 指定为 GORM 主键并启用自增。
	// json:"id" 序列化为 JSON 时字段名为 "id"。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`

	// AgentID 是 Agent 在平台上的全局唯一标识符（如 "weather-bot"），由注册者指定。
	// 该字段用于 API 路由、Agent 间调用引用等场景，是面向外部的主要标识。
	// gorm:"type:varchar(100);uniqueIndex;not null" 限制最大长度 100，建立唯一索引，不允许为空。
	// json:"agent_id" 序列化为 JSON 时字段名为 "agent_id"。
	AgentID string `gorm:"type:varchar(100);uniqueIndex;not null" json:"agent_id"`

	// OwnerID 是该 Agent 所属用户的 ID，作为外键关联 users 表。
	// gorm:"not null;index" 不允许为空，并建立普通索引以加速按所有者查询。
	// json:"owner_id" 序列化为 JSON 时字段名为 "owner_id"。
	OwnerID uint64 `gorm:"not null;index" json:"owner_id"`

	// DisplayName 是 Agent 的显示名称，用于界面展示。
	// gorm:"type:varchar(200);not null" 限制最大长度 200，不允许为空。
	// json:"display_name" 序列化为 JSON 时字段名为 "display_name"。
	DisplayName string `gorm:"type:varchar(200);not null" json:"display_name"`

	// Description 是 Agent 的详细描述信息，说明该 Agent 的用途和功能。
	// gorm:"type:text" 使用 TEXT 类型，允许较长文本。
	// json:"description" 序列化为 JSON 时字段名为 "description"。
	Description string `gorm:"type:text" json:"description"`

	// AvatarURL 是 Agent 的头像地址。
	// gorm:"type:varchar(512)" 限制最大长度 512。
	// json:"avatar_url,omitempty" 序列化为 JSON 时字段名为 "avatar_url"，为空时省略。
	AvatarURL string `gorm:"type:varchar(512)" json:"avatar_url,omitempty"`

	// ConnectionMode 表示 Agent 与平台的连接方式。
	// 取值范围：
	//   - "tunnel"：通过平台内置隧道连接（默认），Agent 主动连接到平台
	//   - "direct"：平台直接调用 Agent 提供的 EndpointURL
	// gorm:"type:enum('tunnel','direct');default:'tunnel';not null" 使用 MySQL 枚举类型，默认 "tunnel"。
	// json:"connection_mode" 序列化为 JSON 时字段名为 "connection_mode"。
	ConnectionMode string `gorm:"type:enum('tunnel','direct');default:'tunnel';not null" json:"connection_mode"`

	// EndpointURL 是 Agent 在 direct 连接模式下的回调地址。
	// 当 ConnectionMode 为 "direct" 时，平台通过此 URL 向 Agent 发送调用请求。
	// gorm:"type:varchar(512)" 限制最大长度 512。
	// json:"endpoint_url,omitempty" 序列化为 JSON 时字段名为 "endpoint_url"，为空时省略。
	EndpointURL string `gorm:"type:varchar(512)" json:"endpoint_url,omitempty"`

	// AgentSecretHash 存储 Agent 认证密钥的哈希值，用于 Agent 接入平台时的身份验证。
	// gorm:"type:varchar(255);not null" 限制最大长度 255，不允许为空。
	// json:"-" 表示该字段在 JSON 序列化时始终忽略，避免泄露敏感信息。
	AgentSecretHash string `gorm:"type:varchar(255);not null" json:"-"`

	// DataPolicy 是 Agent 的数据处理策略配置，以 JSON 对象形式存储。
	// 例如可包含数据保留期限、是否允许日志记录等键值对。
	// gorm:"type:json" 使用 MySQL JSON 列类型。
	// json:"data_policy,omitempty" 序列化为 JSON 时字段名为 "data_policy"，为空时省略。
	DataPolicy JSONMap `gorm:"type:json" json:"data_policy,omitempty"`

	// Status 表示 Agent 的当前在线状态。
	// 取值范围：
	//   - "online"：在线，可接收调用
	//   - "offline"：离线（默认）
	//   - "removed"：已移除，不再参与任何调用
	// gorm:"type:enum('online','offline','removed');default:'offline';not null" 使用 MySQL 枚举类型，默认 "offline"。
	// json:"status" 序列化为 JSON 时字段名为 "status"。
	Status string `gorm:"type:enum('online','offline','removed');default:'offline';not null" json:"status"`

	// LastHeartbeatAt 记录 Agent 最近一次心跳上报的时间。
	// 使用指针类型以支持 NULL 值（Agent 从未上报过心跳时为 nil）。
	// json:"last_heartbeat_at,omitempty" 序列化为 JSON 时字段名为 "last_heartbeat_at"，为空时省略。
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`

	// FrameworkVersion 是 Agent 所使用的 Skynet Agent SDK/框架版本号。
	// gorm:"type:varchar(50)" 限制最大长度 50。
	// json:"framework_version,omitempty" 序列化为 JSON 时字段名为 "framework_version"，为空时省略。
	FrameworkVersion string `gorm:"type:varchar(50)" json:"framework_version,omitempty"`

	// Version 是 Agent 自身的业务版本号，由 Agent 开发者指定。
	// gorm:"type:varchar(50);default:'1.0.0'" 限制最大长度 50，默认值为 "1.0.0"。
	// json:"version" 序列化为 JSON 时字段名为 "version"。
	Version string `gorm:"type:varchar(50);default:'1.0.0'" json:"version"`

	// CreatedAt 是 Agent 记录的创建时间，由 GORM 自动填充。
	// gorm:"autoCreateTime:milli" 在插入时自动设置为当前时间（毫秒精度）。
	// json:"created_at" 序列化为 JSON 时字段名为 "created_at"。
	CreatedAt time.Time `gorm:"autoCreateTime:milli" json:"created_at"`

	// UpdatedAt 是 Agent 记录的最后更新时间，由 GORM 自动维护。
	// gorm:"autoUpdateTime:milli" 在每次更新时自动设置为当前时间（毫秒精度）。
	// json:"updated_at" 序列化为 JSON 时字段名为 "updated_at"。
	UpdatedAt time.Time `gorm:"autoUpdateTime:milli" json:"updated_at"`

	// --- 关联关系 ---

	// Owner 是该 Agent 所属的用户对象（延迟加载）。
	// gorm:"foreignKey:OwnerID" 指定外键为 OwnerID，关联 User 表的主键。
	// json:"owner,omitempty" 序列化为 JSON 时字段名为 "owner"，未加载时省略。
	Owner *User `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`

	// Capabilities 是该 Agent 注册的所有能力/Skill 列表（一对多关系）。
	// gorm:"foreignKey:AgentID;references:AgentID" 指定外键为 Capability.AgentID，
	// 引用 Agent.AgentID（而非默认主键），使能力通过 agent_id 字符串关联。
	// json:"capabilities,omitempty" 序列化为 JSON 时字段名为 "capabilities"，未加载时省略。
	Capabilities []Capability `gorm:"foreignKey:AgentID;references:AgentID" json:"capabilities,omitempty"`
}

// JSONMap 是一个可被 GORM 序列化/反序列化为 JSON 的通用映射类型。
// 底层类型为 map[string]any，适用于存储非固定结构的 JSON 对象（如 Agent 的 DataPolicy）。
// 实现了 driver.Valuer 和 sql.Scanner 接口以支持 GORM 的自动读写。
type JSONMap map[string]any

// Value 实现 driver.Valuer 接口，将 JSONMap 序列化为 JSON 字节数组以写入数据库。
//
// 返回值：
//   - driver.Value：JSON 编码后的 []byte，或 nil（当 JSONMap 本身为 nil 时）。
//   - error：JSON 编码过程中的错误，正常情况下为 nil。
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口，将数据库中的 JSON 字节数据反序列化为 JSONMap。
//
// 参数：
//   - value：数据库驱动返回的原始值，预期为 []byte 类型或 nil。
//
// 返回值：
//   - error：JSON 反序列化过程中的错误，正常情况下为 nil。
//     当 value 为 nil 时，JSONMap 被置为 nil；当 value 不是 []byte 类型时静默忽略。
func (j *JSONMap) Scan(value any) error {
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
