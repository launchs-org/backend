package model

import (
	"time"
	"backend/database"
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

// DeleteImagesByContainerID はコンテナIDに紐づくイメージを削除します
func DeleteImagesByContainerID(containerID string) error {
	return database.DB.Where("container_id = ?", containerID).Delete(&Image{}).Error
}
