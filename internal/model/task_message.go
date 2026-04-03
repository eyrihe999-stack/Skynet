package model

import "time"

type TaskMessage struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID      string    `gorm:"type:varchar(36);not null;index:idx_task" json:"task_id"`
	Direction   string    `gorm:"type:enum('to_agent','from_agent');not null" json:"direction"`
	MessageType string    `gorm:"type:enum('input','output','question','reply','progress');not null" json:"message_type"`
	PayloadRef  *string   `gorm:"type:varchar(512)" json:"payload_ref,omitempty"`
	CreatedAt   time.Time `gorm:"autoCreateTime:milli" json:"created_at"`
}
