package models

import "time"

// UserQuota はユーザーごとのプラン割り当てと個別上書き上限を管理する
// 実効上限値は COALESCE(override_*, plan.*) で解決する
type UserQuota struct {
	ID     string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID string `gorm:"type:varchar(255);not null;uniqueIndex"          json:"user_id"` // ユーザー ID（一意）
	PlanID string `gorm:"type:uuid;not null"                              json:"plan_id"` // 割り当てプラン ID
	Plan   Plan   `gorm:"foreignKey:PlanID"                               json:"-"`       // プランのプリロード用（JSON 非公開）

	// 個別ユーザー上書き（nil の場合はプランの値を使う）
	OverrideMaxProjects              *int `gorm:"column:override_max_projects"                json:"override_max_projects"`               // プロジェクト上限の個別上書き
	OverrideMaxDeployments           *int `gorm:"column:override_max_deployments"             json:"override_max_deployments"`            // デプロイメント上限の個別上書き
	OverrideMaxReplicasPerDeployment *int `gorm:"column:override_max_replicas_per_deployment" json:"override_max_replicas_per_deployment"` // レプリカ上限の個別上書き
	OverrideMaxVolumes               *int `gorm:"column:override_max_volumes"                 json:"override_max_volumes"`                // ボリューム数上限の個別上書き
	OverrideMaxVolumeSizeMB          *int `gorm:"column:override_max_volume_size_mb"          json:"override_max_volume_size_mb"`         // 1ボリューム最大サイズの個別上書き（MB）
	OverrideMaxTotalVolumeMB         *int `gorm:"column:override_max_total_volume_mb"         json:"override_max_total_volume_mb"`        // ボリューム総容量上限の個別上書き（MB）

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserQuota) TableName() string { return "user_quotas" }
