package model

import "time"

type Approval struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	InvocationID uint64     `gorm:"not null" json:"invocation_id"`
	OwnerID      uint64     `gorm:"not null;index:idx_owner_status" json:"owner_id"`
	Status       string     `gorm:"type:enum('pending','approved','denied','expired');not null;default:'pending';index:idx_owner_status" json:"status"`
	DecidedAt    *time.Time `json:"decided_at,omitempty"`
	ExpiresAt    time.Time  `gorm:"not null" json:"expires_at"`
	CreatedAt    time.Time  `gorm:"autoCreateTime:milli" json:"created_at"`
}

func (Approval) TableName() string { return "approval_queue" }
