package models

import (
	"time"
)

// Project プロジェクトの詳細情報を保持するモデル
type Project struct {
	ID        string    `gorm:"primaryKey" json:"id"`        // UUID
	Name      string    `json:"name"`
	Namespace string    `gorm:"uniqueIndex" json:"namespace"` // K8s Namespace 名
	OwnerID   string    `gorm:"index" json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}
