package models

import (
	sharedModels "shared/models"
)

// CreateContainer 新しいコンテナをデータベースに保存します
func CreateContainer(container *sharedModels.Container) error {
	return sharedModels.Instance.Create(container).Error
}

// GetContainerByID IDに一致するコンテナをデータベースから取得します
func GetContainerByID(id string) (*sharedModels.Container, error) {
	var container sharedModels.Container
	if err := sharedModels.Instance.Where("id = ?", id).First(&container).Error; err != nil {
		return nil, err
	}
	return &container, nil
}

// GetContainersByDeployment デプロイメントに紐づくコンテナ一覧を取得します
func GetContainersByDeployment(deploymentID string) ([]sharedModels.Container, error) {
	var containers []sharedModels.Container
	if err := sharedModels.Instance.Where("deployment_id = ?", deploymentID).Find(&containers).Error; err != nil {
		return nil, err
	}
	return containers, nil
}
