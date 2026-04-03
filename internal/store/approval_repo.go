package store

import (
	"time"

	"github.com/eyrihe999-stack/Skynet/internal/model"
	"gorm.io/gorm"
)

type ApprovalRepo struct {
	db *gorm.DB
}

func NewApprovalRepo(db *gorm.DB) *ApprovalRepo {
	return &ApprovalRepo{db: db}
}

func (r *ApprovalRepo) Create(approval *model.Approval) error {
	return r.db.Create(approval).Error
}

func (r *ApprovalRepo) FindByID(id uint64) (*model.Approval, error) {
	var a model.Approval
	if err := r.db.First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// ListPending 查询指定 owner 的待审批列表
func (r *ApprovalRepo) ListPending(ownerID uint64, page, pageSize int) ([]model.Approval, int64, error) {
	query := r.db.Model(&model.Approval{}).
		Where("owner_id = ? AND status = 'pending' AND expires_at > ?", ownerID, time.Now())

	var total int64
	query.Count(&total)

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	var approvals []model.Approval
	err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&approvals).Error

	return approvals, total, err
}

// Decide 更新审批决定（approve 或 deny）
func (r *ApprovalRepo) Decide(id uint64, status string) error {
	now := time.Now()
	return r.db.Model(&model.Approval{}).Where("id = ? AND status = 'pending'", id).
		Updates(map[string]any{
			"status":     status,
			"decided_at": now,
		}).Error
}

// ExpireStale 将过期的 pending 审批标记为 expired
func (r *ApprovalRepo) ExpireStale() (int64, error) {
	result := r.db.Model(&model.Approval{}).
		Where("status = 'pending' AND expires_at <= ?", time.Now()).
		Update("status", "expired")
	return result.RowsAffected, result.Error
}
