package model

import (
	"time"
)

// Image はコンテナイメージを表すモデルです
type Image struct {
	ID          string    `gorm:"primaryKey;type:varchar(255)" json:"id"`                        // イメージID
	ContainerID string    `gorm:"index;type:varchar(255)" json:"container_id"`                   // コンテナID
	Type        string    `json:"type"`                                        // タイプ (user, system)
	Name        string    `json:"name"`                                        // イメージ名
	Registry    string    `json:"registry"`                                    // レジストリURL
	CreatedAt   time.Time `json:"created_at"`                                  // 作成日時
	UpdatedAt   time.Time `json:"updated_at"`                                  // 更新日時
}
