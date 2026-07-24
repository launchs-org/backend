package models

import "time"

type DeploymentWebhook struct {
	ID             string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`              // UUID 主キー
	DeploymentID   string    `gorm:"type:uuid;not null;index"                       json:"deployment_id"`   // 紐づく deployment の ID
	Secret         string    `gorm:"type:varchar(255);not null"                     json:"secret"`          // HMAC 検証用シークレット
	GithubRepoURL  string    `gorm:"type:text;not null"                             json:"github_repo_url"` // GitHub リポジトリ URL
	IsActive       bool      `gorm:"not null;default:true"                          json:"is_active"`       // Webhook が有効かどうか
	CreatedAt      time.Time `                                                       json:"created_at"`      // 作成日時
	UpdatedAt      time.Time `                                                       json:"updated_at"`      // 更新日時
}

func (DeploymentWebhook) TableName() string { return "deployment_webhooks" }
