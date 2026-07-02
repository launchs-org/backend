package models

import "time"

type VolumeMountStatus string

const (
	VolumeMountStatusPending  VolumeMountStatus = "pending"
	VolumeMountStatusMounted  VolumeMountStatus = "mounted"
	VolumeMountStatusDeleting VolumeMountStatus = "deleting"
)

// VolumeMount は volumes と deployments の中間テーブル
// UNIQUE 制約: (volume_id, deployment_id)
type VolumeMount struct {
	ID               string            `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	VolumeID         string            `gorm:"type:uuid;not null;index"                       json:"volume_id"`
	DeploymentID     string            `gorm:"type:uuid;not null;index"                       json:"deployment_id"`
	MountPath        string            `gorm:"type:varchar(255);not null"                     json:"mount_path"`
	PendingMountPath string            `gorm:"type:varchar(255)"                              json:"pending_mount_path"`
	Status           VolumeMountStatus `gorm:"type:varchar(32);not null;default:'pending'"    json:"status"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

func (VolumeMount) TableName() string { return "volume_mounts" }
