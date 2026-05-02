package model

import (
	"time"

	"backend/database"
	"gorm.io/gorm"
)

// Container はコンテナを表すモデルです
type Container struct {
	ID            string    `gorm:"primaryKey;type:varchar(255)" json:"id"`                      // コンテナID
	ProjectID     string    `gorm:"index;type:varchar(255)" json:"project_id"`                   // プロジェクトID
	Name          string    `gorm:"index;type:varchar(255)" json:"name"`                         // コンテナ名
	ImageID       string    `gorm:"index;type:varchar(255)" json:"image_id"`                     // イメージID
	RepositoryURL string    `json:"repository_url"`                            // リポジトリURL
	Branch        string    `gorm:"default:'main'" json:"branch"`              // ブランチ
	Directory     string    `gorm:"default:'/'" json:"directory"`              // ディレクトリ
	Version       string    `json:"version"`                                   // バージョン
	Replicas      int       `gorm:"default:1" json:"replicas"`                 // レプリカ数
	EnvVars       string    `gorm:"type:text" json:"env_vars"`                 // 環境変数(JSON)
	Resources     string    `gorm:"type:text" json:"resources"`                // リソース制限(JSON)
	Status        string    `gorm:"default:'Stopped'" json:"status"`           // ステータス
	CreatedAt     time.Time `json:"created_at"`                                // 作成日時
	UpdatedAt     time.Time `json:"updated_at"`                                // 更新日時
}

// GetContainerCountByProjectIDAndName はプロジェクトIDとコンテナ名からコンテナの数を取得します
func GetContainerCountByProjectIDAndName(projectID, name string) (int64, error) {
	var count int64
	err := database.DB.Model(&Container{}).Where("project_id = ? AND name = ?", projectID, name).Count(&count).Error
	return count, err
}

// CreateContainerWithRelatedRecords はコンテナと関連リソースをトランザクションで作成します
func CreateContainerWithRelatedRecords(image *Image, container *Container, service *Service, buildJob *BuildJob) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(image).Error; err != nil {
			return err
		}
		if err := tx.Create(container).Error; err != nil {
			return err
		}
		if err := tx.Create(service).Error; err != nil {
			return err
		}
		if err := tx.Create(buildJob).Error; err != nil {
			return err
		}
		return nil
	})
}

// UpdateContainerStatus はコンテナのステータスを更新します
func UpdateContainerStatus(id, status string) error {
	return database.DB.Model(&Container{}).Where("id = ?", id).Update("status", status).Error
}
