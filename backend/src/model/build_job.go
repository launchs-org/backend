package model

import (
	"context"
	"time"

	"backend/database"
	"gorm.io/gorm"
)

// BuildJob はビルドジョブを表すモデルです
type BuildJob struct {
	ID            string     `gorm:"primaryKey;type:varchar(255)" json:"id"`                     // ビルドジョブID
	ProjectID     string     `gorm:"index;type:varchar(255)" json:"project_id"`                  // プロジェクトID
	ContainerID   string     `gorm:"index;type:varchar(255)" json:"container_id"`                // コンテナID
	RepositoryURL string     `json:"repository_url"`                           // リポジトリURL
	Branch        string     `json:"branch"`                                   // ブランチ
	Directory     string     `json:"directory"`                                // ディレクトリ
	Status        string     `json:"status"`                                   // ステータス
	BuildLog      []byte     `gorm:"type:longblob" json:"-"`                  // ビルドログ
	StartedAt     *time.Time `json:"started_at"`                               // 開始日時
	FinishedAt    *time.Time `json:"finished_at"`                              // 終了日時
	CreatedAt     time.Time  `json:"created_at"`                               // 作成日時
	UpdatedAt     time.Time  `json:"updated_at"`                               // 更新日時
}

// UpdateBuildJobStatus はビルドジョブのステータスなどを更新します
func UpdateBuildJobStatus(id string, updates map[string]interface{}) error {
	return database.DB.Model(&BuildJob{}).Where("id = ?", id).Updates(updates).Error
}

// GetBuildJobsByContainerID はコンテナIDからビルドジョブのリストを作成日時の降順で取得します
func GetBuildJobsByContainerID(containerID string) ([]BuildJob, error) {
	var jobs []BuildJob
	err := database.DB.Where("container_id = ?", containerID).Order("created_at desc").Find(&jobs).Error
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

// GetBuildJobByID はIDからビルドジョブを取得します
func GetBuildJobByID(id string) (*BuildJob, error) {
	var job BuildJob
	err := database.DB.Where("id = ?", id).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// AppendBuildLog はビルドログを追記します
func AppendBuildLog(id string, log []byte) error {
	// MySQLの CONCAT を使用して追記します
	// 初期値が NULL の場合を考慮して COALESCE を使用します
	err := database.DB.Model(&BuildJob{}).Where("id = ?", id).Update("build_log", gorm.Expr("CONCAT(COALESCE(build_log, ''), ?)", log)).Error
	if err != nil {
		database.DB.Logger.Error(context.Background(), "failed to append build log: %v", err)
	}
	return err
}

// DeleteBuildJobsByContainerID はコンテナIDに紐づくビルドジョブを削除します
func DeleteBuildJobsByContainerID(containerID string) error {
	return database.DB.Where("container_id = ?", containerID).Delete(&BuildJob{}).Error
}

// GetBuildJobLog はビルドログを取得します
func GetBuildJobLog(id string) ([]byte, error) {
	var job BuildJob
	err := database.DB.Select("build_log").Where("id = ?", id).First(&job).Error
	if err != nil {
		return nil, err
	}
	return job.BuildLog, nil
}
