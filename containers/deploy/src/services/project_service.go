package services

import (
	"deploy/models"
)

// ProjectService プロジェクトに関するビジネスロジックを定義するインターフェース
type ProjectService interface {
	CreateProject(project *models.Project) error
	GetProjectByID(projectID string) (*models.Project, error)
	ListProjectsByOwner(ownerID string) ([]models.Project, error)
	DeleteProject(projectID string) error
}

// projectService ProjectService の実装
type projectService struct {
	// DB インスタンスなどをここに保持する
}

// NewProjectService ProjectService の新しいインスタンスを作成する
func NewProjectService() ProjectService {
	return &projectService{}
}

// CreateProject 新しいプロジェクトを作成し、DB に記録します。
func (service *projectService) CreateProject(project *models.Project) error {
	// 実装はユーザーが行う
	return nil
}

// GetProjectByID 指定されたプロジェクト ID の詳細情報を取得します。
func (service *projectService) GetProjectByID(projectID string) (*models.Project, error) {
	// 実装はユーザーが行う
	return nil, nil
}

// ListProjectsByOwner 指定されたオーナーに関連付けられたプロジェクトの一覧を取得します。
func (service *projectService) ListProjectsByOwner(ownerID string) ([]models.Project, error) {
	// 実装はユーザーが行う
	return nil, nil
}

// DeleteProject 指定されたプロジェクトを削除します。
func (service *projectService) DeleteProject(projectID string) error {
	// 実装はユーザーが行う
	return nil
}
