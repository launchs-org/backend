package services

import (
	repo "deploy/models"
	sharedModels "shared/models"
)

// ProjectService プロジェクトに関するビジネスロジックを定義するインターフェース
type ProjectService interface {
	CreateProject(project *sharedModels.Project) error
	GetProjectByID(projectID string) (*sharedModels.Project, error)
	ListProjectsByOwner(ownerID string) ([]sharedModels.Project, error)
	DeleteProject(projectID string) error
}

// projectService ProjectService の実装
type projectService struct {
}

// NewProjectService ProjectService の新しいインスタンスを作成する
func NewProjectService() ProjectService {
	return &projectService{}
}

// CreateProject 新しいプロジェクトを作成し、DB に記録します。
func (service *projectService) CreateProject(project *sharedModels.Project) error {
	return repo.CreateProject(project)
}

// GetProjectByID 指定されたプロジェクト ID の詳細情報を取得します。
func (service *projectService) GetProjectByID(projectID string) (*sharedModels.Project, error) {
	return repo.GetProjectByID(projectID)
}

// ListProjectsByOwner 指定されたオーナーに関連付けられたプロジェクトの一覧を取得します。
func (service *projectService) ListProjectsByOwner(ownerID string) ([]sharedModels.Project, error) {
	return repo.GetProjectsByOwner(ownerID)
}

// DeleteProject 指定されたプロジェクトを削除します。
func (service *projectService) DeleteProject(projectID string) error {
	return repo.DeleteProject(projectID)
}
