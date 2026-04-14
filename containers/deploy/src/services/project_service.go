package services

import (
	s_models "github.com/launchs-org/backend/shared/models"
)

// ProjectService プロジェクトに関するビジネスロジックを定義するインターフェース
type ProjectService interface {
	CreateProject(project *s_models.Project) error
	GetProjectByID(projectID string) (*s_models.Project, error)
	ListProjectsByOwner(ownerID string) ([]s_models.Project, error)
	DeleteProject(projectID string) error
}

// projectService ProjectService の実装
type projectService struct {
	db *s_models.Database
}

// NewProjectService ProjectService の新しいインスタンスを作成する
func NewProjectService(db *s_models.Database) ProjectService {
	return &projectService{
		db: db,
	}
}

// CreateProject 新しいプロジェクトを作成し、DB に記録します。
func (service *projectService) CreateProject(project *s_models.Project) error {
	return service.db.Conn.Create(project).Error
}

// GetProjectByID 指定されたプロジェクト ID の詳細情報を取得します。
func (service *projectService) GetProjectByID(projectID string) (*s_models.Project, error) {
	var project s_models.Project
	if err := service.db.Conn.First(&project, "id = ?", projectID).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

// ListProjectsByOwner 指定されたオーナーに関連付けられたプロジェクトの一覧を取得します。
func (service *projectService) ListProjectsByOwner(ownerID string) ([]s_models.Project, error) {
	var projects []s_models.Project
	if err := service.db.Conn.Find(&projects, "owner_id = ?", ownerID).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

// DeleteProject 指定されたプロジェクトを削除します。
func (service *projectService) DeleteProject(projectID string) error {
	return service.db.Conn.Delete(&s_models.Project{}, "id = ?", projectID).Error
}
