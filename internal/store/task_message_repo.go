package store

import (
	"github.com/eyrihe999-stack/Skynet/internal/model"
	"gorm.io/gorm"
)

type TaskMessageRepo struct {
	db *gorm.DB
}

func NewTaskMessageRepo(db *gorm.DB) *TaskMessageRepo {
	return &TaskMessageRepo{db: db}
}

func (r *TaskMessageRepo) Create(msg *model.TaskMessage) error {
	return r.db.Create(msg).Error
}

func (r *TaskMessageRepo) FindByTaskID(taskID string) ([]model.TaskMessage, error) {
	var msgs []model.TaskMessage
	err := r.db.Where("task_id = ?", taskID).Order("created_at ASC").Find(&msgs).Error
	return msgs, err
}
