package models

import (
	sharedModels "shared/models"
)

// ListPortsByDeployment デプロイメントに紐づくポート設定一覧を取得します
func ListPortsByDeployment(deploymentID string) ([]sharedModels.Port, error) {
	var ports []sharedModels.Port
	if err := sharedModels.Instance.Where("deployment_id = ?", deploymentID).Find(&ports).Error; err != nil {
		return nil, err
	}
	return ports, nil
}

// SavePortsByDeployment デプロイメントのポート設定を保存します
func SavePortsByDeployment(ports []sharedModels.Port) error {
	if len(ports) == 0 {
		return nil
	}
	return sharedModels.Instance.Save(&ports).Error
}
