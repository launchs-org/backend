package services

import (
	"backend/database"
	"backend/deploy/dto"
	"backend/deploy/kubernetes"
	"backend/deploy/models"
	"backend/logger"
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProjectService はプロジェクトに関するビジネスロジックを提供します
type ProjectService struct{}

// NewProjectService は ProjectService の新しいインスタンスを作成します
func NewProjectService() *ProjectService {
	return &ProjectService{}
}

// GetAllProjects は指定されたユーザーのすべてのプロジェクトを取得します
func (service *ProjectService) GetAllProjects(ownerID string) ([]models.Project, error) {
	logger.Println("プロジェクト一覧取得開始 オーナーID:", ownerID)
	projectModel := &models.Project{}
	projects, err := projectModel.FindAllByOwner(database.DB, ownerID)
	if err != nil {
		logger.PrintErr("プロジェクト一覧取得失敗:", err)
		return nil, err
	}
	return projects, nil
}

// CreateProject は新しいプロジェクトを作成します
func (service *ProjectService) CreateProject(req dto.ProjectCreateRequest, ownerID string) (*models.Project, error) {
	logger.Println("プロジェクト作成開始 名前:", req.Name, "オーナーID:", ownerID)
	
	// サービス層でモデルを初期化
	project := &models.Project{
		ID:              uuid.New().String(),
		Name:            req.Name,
		K8sResourceName: req.K8sResourceName,
		Namespace:       req.Namespace,
		OwnerID:         ownerID,
	}
	
	// Kubernetes Namespace の作成
	if err := kubernetes.CreateNamespace(context.Background(), project.Namespace); err != nil {
		logger.PrintErr("K8s Namespace作成失敗:", err)
		return nil, err
	}
	
	if err := project.Create(database.DB); err != nil {
		// 失敗時は Namespace も削除すべきだが、ここでは簡易化
		logger.PrintErr("プロジェクトDB作成失敗:", err)
		return nil, err
	}
	logger.Println("プロジェクト作成成功 ID:", project.ID)
	return project, nil
}

// GetProjectByID は指定された ID のプロジェクトを取得します
func (service *ProjectService) GetProjectByID(projectID string, ownerID string) (*models.Project, error) {
	logger.Println("プロジェクト詳細取得開始 プロジェクトID:", projectID, "オーナーID:", ownerID)
	project := &models.Project{}
	err := project.FindByID(database.DB, projectID, ownerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Println("プロジェクトが見つかりません ID:", projectID)
			return nil, errors.New("project not found")
		}
		logger.PrintErr("プロジェクト詳細取得失敗:", err)
		return nil, err
	}
	return project, nil
}

// DeleteProject はプロジェクトとその関連リソースを削除します
func (service *ProjectService) DeleteProject(projectID string, ownerID string) error {
	logger.Println("プロジェクト削除開始 プロジェクトID:", projectID, "オーナーID:", ownerID)
	project := &models.Project{}
	if err := project.FindByID(database.DB, projectID, ownerID); err != nil {
		logger.PrintErr("削除対象プロジェクトの取得失敗:", err)
		return err
	}

	// Kubernetes Namespace の削除 (全リソースが削除される)
	if err := kubernetes.DeleteNamespace(context.Background(), project.Namespace); err != nil {
		logger.PrintErr("K8s Namespace削除失敗:", err)
		// 続行
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// 関連リソースの削除
		containerModel := &models.Container{}
		if err := containerModel.DeleteAllByProjectID(tx, projectID); err != nil {
			return err
		}
		
		if err := project.Delete(tx); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		logger.PrintErr("プロジェクト削除トランザクション失敗:", err)
		return err
	}
	logger.Println("プロジェクト削除成功 ID:", projectID)
	return nil
}
