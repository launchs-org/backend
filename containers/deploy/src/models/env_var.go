package models

import (
	sharedModels "shared/models"
)

// ListEnvVarsByDeployment デプロイメントに紐づく環境変数一覧を取得します
func ListEnvVarsByDeployment(deploymentID string) ([]sharedModels.EnvVar, error) {
	var envVars []sharedModels.EnvVar
	if err := sharedModels.Instance.Where("deployment_id = ?", deploymentID).Find(&envVars).Error; err != nil {
		return nil, err
	}
	return envVars, nil
}

// SaveEnvVarsByDeployment デプロイメントの環境変数を保存します
func SaveEnvVarsByDeployment(envVars []sharedModels.EnvVar) error {
	if len(envVars) == 0 {
		return nil
	}
	return sharedModels.Instance.Save(&envVars).Error
}
