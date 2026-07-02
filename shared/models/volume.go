package models

import (
	"time"

	"gorm.io/datatypes"
)

type VolumeStatus string

const (
	VolumeStatusPending  VolumeStatus = "pending"  // 作成済み・未 apply
	VolumeStatusBound    VolumeStatus = "bound"
	VolumeStatusDeleting VolumeStatus = "deleting"
)

type Volume struct {
	ID        string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ProjectID string         `gorm:"type:uuid;not null;index"                       json:"project_id"`
	Name      string         `gorm:"type:varchar(63);not null"                      json:"name"`
	SizeMB    int            `gorm:"not null"                                       json:"size_mb"` // 作成後変更不可。PVC ReclaimPolicy = Delete
	Status    VolumeStatus   `gorm:"type:varchar(32);not null;default:'pending'"    json:"status"`
	K8sStatus datatypes.JSON `gorm:"type:jsonb"                                     json:"k8s_status"` // null = 未同期
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (Volume) TableName() string { return "volumes" }
