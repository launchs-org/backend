package models

import (
	"time"

	"gorm.io/gorm"
)

// ContainerLog はコンテナの実行ログを管理するモデルです
type ContainerLog struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	ContainerID string    `gorm:"index" json:"container_id"`
	PodName     string    `json:"pod_name"`
	Log         []byte    `gorm:"type:blob" json:"log"`
	CollectedAt time.Time `gorm:"index" json:"collected_at"`
}

// DeleteOldLogs は古いコンテナログを物理削除します
func (logRecord *ContainerLog) DeleteOldLogs(db *gorm.DB, threshold time.Time) (int64, error) {
	result := db.Unscoped().
		Where("collected_at < ?", threshold).
		Delete(&ContainerLog{})
	return result.RowsAffected, result.Error
}
