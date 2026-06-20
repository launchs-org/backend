package models

import "time"

// UserQuota はユーザーごとのリソース上限を管理する
// 認証は別サービスが担当し、user_id（UUID 文字列）のみがこのサービスに渡る
// レコードが存在しない場合は初回アクセス時に upsert で作成する
type UserQuota struct {
	ID                       string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID                   string    `gorm:"type:varchar(255);not null;uniqueIndex"          json:"user_id"`
	MaxProjects              int       `gorm:"not null;default:5"                             json:"max_projects"`
	MaxDeployments           int       `gorm:"not null;default:20"                            json:"max_deployments"`
	MaxReplicasPerDeployment int       `gorm:"not null;default:5"                             json:"max_replicas_per_deployment"`
	MaxVolumeMB              int       `gorm:"not null;default:10240"                         json:"max_volume_mb"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

func (UserQuota) TableName() string { return "user_quotas" }
