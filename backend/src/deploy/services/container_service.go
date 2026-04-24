package services

import (
	"backend/database"
	"backend/deploy/dto"
	"backend/deploy/models"
	"backend/logger"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ContainerService はコンテナに関するビジネスロジックを提供します
type ContainerService struct{}

// NewContainerService は ContainerService の新しいインスタンスを作成します
func NewContainerService() *ContainerService {
	return &ContainerService{}
}

// CreateContainer は新しいコンテナを作成し、ビルドジョブをキューに追加します
func (service *ContainerService) CreateContainer(projectID string, req dto.ContainerCreateRequest) (*models.Container, error) {
	logger.Println("コンテナ作成開始 名前:", req.Name, "プロジェクトID:", projectID)
	
	// サービス層でモデルを初期化
	container := &models.Container{
		ID:            uuid.New().String(),
		ProjectID:     projectID,
		Name:          req.Name,
		RepositoryURL: req.RepositoryURL,
		Branch:        req.Branch,
		Directory:     req.Directory,
		Replicas:      req.Replicas,
		EnvVars:       req.EnvVars,
		Resources:     req.Resources,
		Status:        "Stopped",
		CreatedAt:     time.Now(),
	}

	return container, database.DB.Transaction(func(tx *gorm.DB) error {
		// コンテナの作成
		if err := container.Create(tx); err != nil {
			logger.PrintErr("コンテナレコード作成失敗:", err)
			return err
		}

		// ビルドジョブの作成 (Status: Queued)
		buildJob := &models.BuildJob{
			ID:            container.ID + "-" + time.Now().Format("20060102150405"),
			ProjectID:     projectID,
			ContainerID:   container.ID,
			RepositoryURL: container.RepositoryURL,
			Branch:        container.Branch,
			Directory:     container.Directory,
			Status:        "Queued",
			StartedAt:     time.Now(),
		}
		if err := buildJob.Create(tx); err != nil {
			logger.PrintErr("ビルドジョブ作成失敗:", err)
			return err
		}

		logger.Println("コンテナ作成およびビルドジョブ作成成功 ジョブID:", buildJob.ID)
		return nil
	})
}

// UpdateContainer はコンテナ情報を更新し、新しいビルドジョブをキューに追加します
func (service *ContainerService) UpdateContainer(containerID string, updates map[string]interface{}) error {
	logger.Println("コンテナ更新開始 コンテナID:", containerID)
	return database.DB.Transaction(func(tx *gorm.DB) error {
		container := &models.Container{}
		if err := container.FindByID(tx, containerID); err != nil {
			logger.PrintErr("更新対象コンテナの取得失敗:", err)
			return err
		}

		// コンテナの更新
		if err := container.Update(tx, updates); err != nil {
			logger.PrintErr("コンテナレコード更新失敗:", err)
			return err
		}

		// 再ビルドジョブの作成
		buildJob := &models.BuildJob{
			ID:            container.ID + "-" + time.Now().Format("20060102150405"),
			ProjectID:     container.ProjectID,
			ContainerID:   container.ID,
			RepositoryURL: container.RepositoryURL,
			Branch:        container.Branch,
			Directory:     container.Directory,
			Status:        "Queued",
			StartedAt:     time.Now(),
		}
		if err := buildJob.Create(tx); err != nil {
			logger.PrintErr("再ビルドジョブ作成失敗:", err)
			return err
		}

		logger.Println("コンテナ更新および新ビルドジョブ作成成功 ジョブID:", buildJob.ID)
		return nil
	})
}

// GetContainerByID はコンテナ情報を取得します
func (service *ContainerService) GetContainerByID(containerID string) (*models.Container, error) {
	logger.Println("コンテナ詳細取得開始 コンテナID:", containerID)
	container := &models.Container{}
	err := container.FindByID(database.DB, containerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Println("コンテナが見つかりません ID:", containerID)
			return nil, errors.New("container not found")
		}
		logger.PrintErr("コンテナ詳細取得失敗:", err)
		return nil, err
	}
	return container, nil
}

// DeleteContainer はコンテナと関連リソースを削除します
func (service *ContainerService) DeleteContainer(containerID string) error {
	logger.Println("コンテナ削除開始 コンテナID:", containerID)
	// TODO: K8s Deployment の削除ロジック

	return database.DB.Transaction(func(tx *gorm.DB) error {
		container := &models.Container{}
		if err := container.FindByID(tx, containerID); err != nil {
			logger.PrintErr("削除対象コンテナの取得失敗:", err)
			return err
		}

		if err := container.Delete(tx); err != nil {
			logger.PrintErr("コンテナデータベース削除失敗:", err)
			return err
		}
		logger.Println("コンテナ削除成功 ID:", containerID)
		return nil
	})
}
