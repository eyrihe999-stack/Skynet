package model

import "time"

type PermissionRule struct {
	ID              uint64  `gorm:"primaryKey;autoIncrement" json:"id"`
	AgentID         string  `gorm:"type:varchar(100);not null;index:idx_agent_skill" json:"agent_id"`
	SkillName       *string `gorm:"type:varchar(100);index:idx_agent_skill" json:"skill_name,omitempty"`
	CallerType      string  `gorm:"type:enum('user','agent','any');not null;default:'any'" json:"caller_type"`
	CallerID        *string `gorm:"type:varchar(100)" json:"caller_id,omitempty"`
	Action          string  `gorm:"type:enum('allow','deny');not null;default:'allow'" json:"action"`
	ApprovalMode    string  `gorm:"type:enum('auto','manual');not null;default:'auto'" json:"approval_mode"`
	RateLimitMax    *uint   `json:"rate_limit_max,omitempty"`
	RateLimitWindow *string `gorm:"type:varchar(10)" json:"rate_limit_window,omitempty"`
	Priority        int     `gorm:"not null;default:0" json:"priority"`
	CreatedAt       time.Time `gorm:"autoCreateTime:milli" json:"created_at"`
}
