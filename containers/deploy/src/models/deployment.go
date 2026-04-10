package models

import (
	"time"
)

// Deployment ユーザー入力 ID と K8s リソース名を管理するモデル
type Deployment struct {
	ID                string    `gorm:"primaryKey" json:"id"`   // ユーザー入力 (例: "auth-server")
	ProjectID         string    `gorm:"index" json:"project_id"`
	K8sResourceName   string    `gorm:"uniqueIndex" json:"k8s_resource_name"`  // K8s 用生成名
	ActiveContainerID string    `json:"active_container_id"`
	IsDomainIssued    bool      `gorm:"default:false" json:"is_domain_issued"`
	Replicas          int       `gorm:"default:1" json:"replicas"`
	CreatedAt         time.Time `json:"created_at"`
	
	// Relationships
	Containers []Container `gorm:"foreignKey:DeploymentID" json:"containers,omitempty"`
	EnvVars    []EnvVar    `gorm:"foreignKey:DeploymentID" json:"env_vars,omitempty"`
	Ports      []Port      `gorm:"foreignKey:DeploymentID" json:"ports,omitempty"`
}
