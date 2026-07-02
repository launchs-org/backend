package models

import (
	"time"

	"gorm.io/datatypes"
)

type ProjectStatus string

const (
	ProjectStatusProvisioning ProjectStatus = "provisioning"
	ProjectStatusActive       ProjectStatus = "active"
	ProjectStatusDeleting     ProjectStatus = "deleting"
)

// ステータス遷移: provisioning → active / active → deleting
type Project struct {
	ID        string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID    string         `gorm:"type:varchar(255);not null;index"               json:"user_id"`
	Name      string         `gorm:"type:varchar(63);not null;uniqueIndex"           json:"name"`
	Namespace string         `gorm:"type:varchar(63);not null;uniqueIndex"           json:"namespace"`
	Status    ProjectStatus  `gorm:"type:varchar(32);not null;default:'provisioning'" json:"status"`
	K8sStatus datatypes.JSON `gorm:"type:jsonb"                                     json:"k8s_status"` // null = 未同期
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (Project) TableName() string { return "projects" }
