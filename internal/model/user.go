// Package model 定义 Skynet 平台的 GORM 数据模型。
//
// 本包中的结构体与 MySQL 数据库表一一对应，是所有数据库操作的基础。
// store 层的 Repository 通过这些模型执行增删改查操作。
// 模型涵盖用户（User）、Agent（Agent）、能力/Skill（Capability）和
// 调用记录（Invocation）四大核心实体，以及 JSONMap、JSONArray、JSONRaw
// 等自定义 GORM 类型。
package model

import "time"

// User 表示 Skynet 平台的注册用户，对应数据库 users 表。
// 用户是平台的基本身份实体，可以拥有多个 Agent、通过 API Key 进行程序化访问、
// 通过邮箱密码进行交互式登录。store 层 UserRepository 基于该模型进行用户管理。
type User struct {
	// ID 是用户的自增主键，由数据库自动生成。
	// gorm:"primaryKey;autoIncrement" 指定为 GORM 主键并启用自增。
	// json:"id" 序列化为 JSON 时字段名为 "id"。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`

	// Email 是用户的电子邮箱地址，用于登录和唯一标识用户。
	// gorm:"type:varchar(255);uniqueIndex;not null" 限制最大长度 255，建立唯一索引，不允许为空。
	// json:"email" 序列化为 JSON 时字段名为 "email"。
	Email string `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`

	// DisplayName 是用户的显示名称，用于界面展示。
	// gorm:"type:varchar(100);not null" 限制最大长度 100，不允许为空。
	// json:"display_name" 序列化为 JSON 时字段名为 "display_name"。
	DisplayName string `gorm:"type:varchar(100);not null" json:"display_name"`

	// AvatarURL 是用户的头像地址。
	// gorm:"type:varchar(512)" 限制最大长度 512。
	// json:"avatar_url,omitempty" 序列化为 JSON 时字段名为 "avatar_url"，为空时省略。
	AvatarURL string `gorm:"type:varchar(512)" json:"avatar_url,omitempty"`

	// AuthProvider 是用户的认证提供方，如 "local"、"github"、"google" 等。
	// gorm:"type:varchar(50);not null;default:'local'" 限制最大长度 50，默认为 "local"。
	// json:"auth_provider" 序列化为 JSON 时字段名为 "auth_provider"。
	AuthProvider string `gorm:"type:varchar(50);not null;default:'local'" json:"auth_provider"`

	// PasswordHash 存储用户密码的哈希值（如 bcrypt），用于交互式登录验证。
	// gorm:"type:varchar(255)" 限制最大长度 255。
	// json:"-" 表示该字段在 JSON 序列化时始终忽略，避免泄露敏感信息。
	PasswordHash string `gorm:"type:varchar(255)" json:"-"`

	// APIKeyHash 存储用户 API Key 的哈希值，用于程序化 API 鉴权。
	// gorm:"type:varchar(255)" 限制最大长度 255。
	// json:"-" 表示该字段在 JSON 序列化时始终忽略，避免泄露敏感信息。
	APIKeyHash string `gorm:"type:varchar(255)" json:"-"`

	// Status 表示用户账户的当前状态。
	// 取值范围：
	//   - "active"：正常激活状态
	//   - "suspended"：被管理员挂起
	//   - "deleted"：已标记删除（软删除）
	// gorm:"type:enum('active','suspended','deleted');default:'active';not null" 使用 MySQL 枚举类型，默认为 "active"。
	// json:"status" 序列化为 JSON 时字段名为 "status"。
	Status string `gorm:"type:enum('active','suspended','deleted');default:'active';not null" json:"status"`

	// CreatedAt 是用户记录的创建时间，由 GORM 自动填充。
	// gorm:"autoCreateTime:milli" 在插入时自动设置为当前时间（毫秒精度）。
	// json:"created_at" 序列化为 JSON 时字段名为 "created_at"。
	CreatedAt time.Time `gorm:"autoCreateTime:milli" json:"created_at"`

	// UpdatedAt 是用户记录的最后更新时间，由 GORM 自动维护。
	// gorm:"autoUpdateTime:milli" 在每次更新时自动设置为当前时间（毫秒精度）。
	// json:"updated_at" 序列化为 JSON 时字段名为 "updated_at"。
	UpdatedAt time.Time `gorm:"autoUpdateTime:milli" json:"updated_at"`
}
