package models

import (
	"time"

	"gorm.io/gorm"
)

// ProjectHistory はプロジェクトの構成履歴を管理するモデルです
type ProjectHistory struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	ProjectID   string    `gorm:"index" json:"project_id"`
	VersionName string    `json:"version_name"`
	ConfigData  string    `gorm:"type:text" json:"config_data"` // 全 Container 設定の JSON スナップショット
	CreatedAt   time.Time `json:"created_at"`
}

// Create は履歴を新規作成します
func (history *ProjectHistory) Create(db *gorm.DB) error {
	return db.Create(history).Error
}
