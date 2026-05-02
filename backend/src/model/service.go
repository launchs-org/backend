package model

import (
	"time"
)

// Service はK8s Service設定を表すモデルです
type Service struct {
	ID          string    `gorm:"primaryKey;type:varchar(255)" json:"id"`                        // サービスID
	ContainerID string    `gorm:"uniqueIndex;type:varchar(255)" json:"container_id"`             // コンテナID
	Type        string    `json:"type"`                                        // Serviceタイプ
	Ports       string    `gorm:"type:text" json:"ports"`                      // ポート設定(JSON)
	IP          string    `json:"ip"`                                          // IPアドレス
	CreatedAt   time.Time `json:"created_at"`                                  // 作成日時
	UpdatedAt   time.Time `json:"updated_at"`                                  // 更新日時
}
