package models

import (
	sharedModels "shared/models"
)

// CreateDeployment 新しいデプロイメントをデータベースに保存します
func CreateDeployment(deployment *sharedModels.Deployment) error {
	return sharedModels.Instance.Create(deployment).Error
}

// GetDeploymentByID IDに一致するデプロイメントをデータベースから取得します
func GetDeploymentByID(id string) (*sharedModels.Deployment, error) {
	var deployment sharedModels.Deployment
	if err := sharedModels.Instance.Preload("Containers").Preload("EnvVars").Preload("Ports").
		Where("id = ?", id).First(&deployment).Error; err != nil {
		return nil, err
	}
	return &deployment, nil
}

// GetDeploymentsByProject プロジェクトに紐づくデプロイメント一覧を取得します
func GetDeploymentsByProject(projectID string) ([]sharedModels.Deployment, error) {
	var deployments []sharedModels.Deployment
	if err := sharedModels.Instance.Where("project_id = ?", projectID).Find(&deployments).Error; err != nil {
		return nil, err
	}
	return deployments, nil
}

// UpdateDeployment デプロイメント情報を更新します
func UpdateDeployment(deployment *sharedModels.Deployment) error {
	return sharedModels.Instance.Save(deployment).Error
}

// DeleteDeployment デプロイメントをデータベースから削除します
func DeleteDeployment(id string) error {
	return sharedModels.Instance.Where("id = ?", id).Delete(&sharedModels.Deployment{}).Error
}
