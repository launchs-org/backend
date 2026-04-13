package models

import (
	sharedModels "shared/models"
)

// CreateProject 新しいプロジェクトをデータベースに保存します
func CreateProject(project *sharedModels.Project) error {
	return sharedModels.Instance.Create(project).Error
}

// GetProjectByID IDに一致するプロジェクトをデータベースから取得します
func GetProjectByID(id string) (*sharedModels.Project, error) {
	var project sharedModels.Project
	if err := sharedModels.Instance.Where("id = ?", id).First(&project).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

// GetProjectsByOwner 所有者に紐づくプロジェクト一覧を取得します
func GetProjectsByOwner(ownerID string) ([]sharedModels.Project, error) {
	var projects []sharedModels.Project
	if err := sharedModels.Instance.Where("owner_id = ?", ownerID).Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

// DeleteProject プロジェクトをデータベースから削除します
func DeleteProject(id string) error {
	return sharedModels.Instance.Where("id = ?", id).Delete(&sharedModels.Project{}).Error
}
