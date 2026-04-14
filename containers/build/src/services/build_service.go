package services

import (
	repo "build/models"
	s_models "github.com/launchs-org/backend/shared/models"
)

// BuildService ビルドとイメージ管理に関するビジネスロジックを定義するインターフェース
type BuildService interface {
	// ビルド管理
	TriggerBuild(projectID string, deploymentID string) (string, error)
	GetBuildStatus(containerID string) (string, error)
	GetBuildLogs(containerID string) ([]byte, error)

	// コンテナ管理 (CRUD)
	CreateContainer(container *s_models.Container) error
	GetContainer(containerID string) (*s_models.Container, error)
	UpdateContainer(container *s_models.Container) error
	DeleteContainer(containerID string) error
}

// buildService BuildService の実装
type buildService struct {
	db *s_models.Database
}

// NewBuildService BuildService の新しいインスタンスを作成する
func NewBuildService(db *s_models.Database) BuildService {
	return &buildService{
		db: db,
	}
}

// CreateContainer コンテナ情報をデータベースに保存します。
func (service *buildService) CreateContainer(container *s_models.Container) error {
	// GORMを使用してコンテナ設定を保存
	return service.db.Conn.Create(container).Error
}

// GetContainer コンテナIDを指定して詳細情報を取得します。
func (service *buildService) GetContainer(containerID string) (*s_models.Container, error) {
	var container s_models.Container
	// コンテナIDに基づいてデータベースから取得
	if err := service.db.Conn.First(&container, "id = ?", containerID).Error; err != nil {
		return nil, err
	}
	return &container, nil
}

// UpdateContainer コンテナのビルドステータスやログを更新します。
func (service *buildService) UpdateContainer(container *s_models.Container) error {
	// 指定された構造体の内容でデータベースを更新
	return service.db.Conn.Save(container).Error
}

// DeleteContainer コンテナ情報を削除します。
func (service *buildService) DeleteContainer(containerID string) error {
	// コンテナのメタデータを削除
	return service.db.Conn.Delete(&s_models.Container{}, "id = ?", containerID).Error
}

// TriggerBuild 新しいビルドジョブを開始し、対応するコンテナ ID を生成します。
func (service *buildService) TriggerBuild(projectID string, deploymentID string) (string, error) {
	// 実際にはここで K8s Job を投げるなどの処理が入る
	return "container-uuid", nil
}

// GetBuildStatus 指定されたコンテナ ID のビルド進捗状況（Building, Success, Failed など）を返します。
func (service *buildService) GetBuildStatus(containerID string) (string, error) {
	container, err := repo.GetContainerByID(containerID)
	if err != nil {
		return "", err
	}
	return container.Status, nil
}

// GetBuildLogs 指定されたコンテナのビルドログを取得します。
func (service *buildService) GetBuildLogs(containerID string) ([]byte, error) {
	container, err := repo.GetContainerByID(containerID)
	if err != nil {
		return nil, err
	}
	return container.BuildLog, nil
}
