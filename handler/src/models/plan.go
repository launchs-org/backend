package models

import "time"

// Plan はユーザーに割り当てるプランとそのリソース上限を管理する
type Plan struct {
	ID                       string              `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name                     string              `gorm:"type:varchar(64);not null;uniqueIndex"           json:"name"`                       // プラン名（free / pro / enterprise）
	MaxProjects              int                 `gorm:"not null;default:5"                              json:"max_projects"`               // プロジェクト上限数
	MaxDeployments           int                 `gorm:"not null;default:20"                             json:"max_deployments"`            // デプロイメント上限数
	MaxReplicasPerDeployment int                 `gorm:"not null;default:5"                              json:"max_replicas_per_deployment"` // デプロイメントあたりのレプリカ上限
	MaxVolumes               int                 `gorm:"not null;default:10"                             json:"max_volumes"`                // ボリューム数上限
	MaxVolumeSizeMB          int                 `gorm:"not null;default:10240"                          json:"max_volume_size_mb"`         // 1ボリュームあたりの最大サイズ（MB）
	MaxTotalVolumeMB         int                 `gorm:"not null;default:51200"                          json:"max_total_volume_mb"`        // ボリューム総容量上限（MB）
	InstanceLimits           []PlanInstanceLimit `gorm:"foreignKey:PlanID"                               json:"instance_limits,omitempty"`  // スペック別デプロイメント上限
	CreatedAt                time.Time           `json:"created_at"`
	UpdatedAt                time.Time           `json:"updated_at"`
}

func (Plan) TableName() string { return "plans" }

// PlanInstanceLimit はプランごとのインスタンスサイズ別デプロイメント数上限を管理する
type PlanInstanceLimit struct {
	PlanID       string `gorm:"primaryKey;type:uuid"        json:"plan_id"`       // プラン ID
	InstanceSize string `gorm:"primaryKey;type:varchar(16)" json:"instance_size"` // インスタンスサイズ（small / medium / large）
	MaxCount     int    `gorm:"not null;default:0"          json:"max_count"`     // このサイズのデプロイメント上限数
}

func (PlanInstanceLimit) TableName() string { return "plan_instance_limits" }
