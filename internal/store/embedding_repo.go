package store

import (
	"encoding/binary"
	"math"

	"github.com/skynetplatform/skynet/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EmbeddingRepo 是 capability embedding 数据仓库，封装了对 capability_embeddings 表的数据库操作。
type EmbeddingRepo struct {
	db *gorm.DB
}

// NewEmbeddingRepo 创建并返回一个新的 EmbeddingRepo 实例。
func NewEmbeddingRepo(db *gorm.DB) *EmbeddingRepo {
	return &EmbeddingRepo{db: db}
}

// Upsert 创建或更新 capability 的 embedding 向量。
func (r *EmbeddingRepo) Upsert(capID uint64, embedding []float32, modelVersion string) error {
	record := model.CapabilityEmbedding{
		CapabilityID: capID,
		Embedding:    Float32ToBytes(embedding),
		ModelVersion: modelVersion,
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "capability_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"embedding", "model_version", "updated_at"}),
	}).Create(&record).Error
}

// GetAll 获取所有 embedding 记录。
func (r *EmbeddingRepo) GetAll() ([]model.CapabilityEmbedding, error) {
	var embeddings []model.CapabilityEmbedding
	err := r.db.Find(&embeddings).Error
	return embeddings, err
}

// Float32ToBytes 将 float32 切片序列化为字节数组（小端序）。
func Float32ToBytes(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// BytesToFloat32 将字节数组反序列化为 float32 切片。
func BytesToFloat32(buf []byte) []float32 {
	v := make([]float32, len(buf)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return v
}
