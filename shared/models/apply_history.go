package models

import (
	"time"

	"gorm.io/datatypes"
)

type ApplyStatus string

const (
	ApplyStatusApplied ApplyStatus = "applied"
	ApplyStatusFailed  ApplyStatus = "failed"
)

type ApplyHistory struct {
	ID           string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	DeploymentID string         `gorm:"type:uuid;not null;index"                       json:"deployment_id"`
	Manifests    datatypes.JSON `gorm:"type:jsonb;not null"                            json:"manifests"` // 生成した k8s manifest 全スナップショット
	Status       ApplyStatus    `gorm:"type:varchar(32);not null"                      json:"status"`
	ErrorMessage string         `gorm:"type:text"                                      json:"error_message"`
	AppliedAt    time.Time      `                                                      json:"applied_at"` // POST /apply が叩かれた時刻
}

func (ApplyHistory) TableName() string { return "apply_history" }
