package models

import (
	"time"

	"gorm.io/datatypes"
)

type IngressRouteStatus string

const (
	IngressRouteStatusPending  IngressRouteStatus = "pending"
	IngressRouteStatusActive   IngressRouteStatus = "active"
	IngressRouteStatusDeleting IngressRouteStatus = "deleting"
)

type IngressRoute struct {
	ID        string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`         // UUID 主キー
	ProjectID string `gorm:"type:uuid;not null;uniqueIndex"                  json:"project_id"` // Project と 1:1

	Host        string `gorm:"type:varchar(253);not null" json:"host"`         // ホスト名（自動生成: {projectID[:8]}.launchs.org）
	PendingHost string `gorm:"type:varchar(253)"          json:"pending_host"` // 未 apply のホスト名

	Status    IngressRouteStatus `gorm:"type:varchar(32);not null;default:'pending'" json:"status"`     // ステータス
	K8sStatus datatypes.JSON     `gorm:"type:jsonb"                                   json:"k8s_status"` // null = 未同期
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

func (IngressRoute) TableName() string { return "ingress_routes" }
