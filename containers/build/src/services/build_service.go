package services

import (
	repo "build/models"
)

// BuildService ビルドとイメージ管理に関するビジネスロジックを定義するインターフェース
type BuildService interface {
	TriggerBuild(projectID string, deploymentID string) (string, error)
	GetBuildStatus(containerID string) (string, error)
	GetBuildLogs(containerID string) ([]byte, error)
}

// buildService BuildService の実装
type buildService struct {
}

// NewBuildService BuildService の新しいインスタンスを作成する
func NewBuildService() BuildService {
	return &buildService{}
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
