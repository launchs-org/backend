package models

import (
	"time"

	"gorm.io/gorm"
)

// Image はビルドされたイメージ情報を管理するモデルです
type Image struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	ContainerID string    `gorm:"index" json:"container_id"`
	Type        string    `json:"type"` // "system" or "user"
	Name        string    `json:"name"`
	Registry    string    `json:"registry"`
	CreatedAt   time.Time `json:"created_at"`
}

// BuildJob はビルド実行履歴を管理するモデルです
type BuildJob struct {
	ID            string     `gorm:"primaryKey" json:"id"`
	ProjectID     string     `gorm:"index" json:"project_id"`
	ContainerID   string     `gorm:"index" json:"container_id"`

	// 実行時のスナップショット情報
	RepositoryURL string     `json:"repository_url"`
	Branch        string     `json:"branch"`
	Directory     string     `json:"directory"`

	// Status: "Queued" | "Running" | "Cancelled" | "Failed" | "Success"
	Status        string     `json:"status"`
	BuildLog      []byte     `gorm:"type:blob" json:"build_log"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
}

// Create はビルドジョブを作成します
func (job *BuildJob) Create(db *gorm.DB) error {
	return db.Create(job).Error
}

// RotateBuildLogs は古いビルドログをクリアします
func (job *BuildJob) RotateBuildLogs(db *gorm.DB, threshold time.Time) (int64, error) {
	result := db.Model(&BuildJob{}).
		Where("finished_at < ? AND build_log IS NOT NULL", threshold).
		Update("build_log", nil)
	return result.RowsAffected, result.Error
}
