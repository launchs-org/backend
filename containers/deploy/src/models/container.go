package models

import (
	"time"
)

// Container ビルドごとのバージョン実体を保持するモデル
type Container struct {
	ID           string    `gorm:"primaryKey" json:"id"`        // UUID
	DeploymentID string    `gorm:"index" json:"deployment_id"`
	ProjectID    string    `gorm:"index" json:"project_id"`             // GC・削除時の効率化
	Image        string    `json:"image"`                               // Harbor URL (tag=UUID)
	Status       string    `json:"status"`    // Building / Success / Failed / Retired
	BuildLog     []byte    `gorm:"type:blob" json:"-"`         // 圧縮ログ (JSONには含めない)
	CreatedAt    time.Time `json:"created_at"`
}
