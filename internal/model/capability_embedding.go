package model

import "time"

// CapabilityEmbedding 表示 capability 的向量化表示，对应数据库 capability_embeddings 表。
// 每个 capability 最多有一条 embedding 记录，通过 CapabilityID 与 capabilities 表关联。
// embedding 向量以二进制字节序列存储在 MEDIUMBLOB 列中（float32 小端序）。
type CapabilityEmbedding struct {
	// CapabilityID 是关联的 capability 主键，同时作为本表的主键。
	CapabilityID uint64 `gorm:"primaryKey" json:"capability_id"`

	// Embedding 是向量化后的二进制数据（float32 小端序字节数组）。
	Embedding []byte `gorm:"type:mediumblob;not null" json:"-"`

	// ModelVersion 记录生成该 embedding 时使用的模型版本。
	ModelVersion string `gorm:"type:varchar(50);not null" json:"model_version"`

	// UpdatedAt 是 embedding 记录的最后更新时间。
	UpdatedAt time.Time `gorm:"autoUpdateTime:milli" json:"updated_at"`
}

// TableName 返回 CapabilityEmbedding 对应的数据库表名。
func (CapabilityEmbedding) TableName() string { return "capability_embeddings" }
