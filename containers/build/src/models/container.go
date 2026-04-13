package models

import (
	sharedModels "shared/models"
)

// CreateContainer 新しいコンテナ（ビルド対象）を保存します
func CreateContainer(container *sharedModels.Container) error {
	return sharedModels.Instance.Create(container).Error
}

// GetContainerByID IDでコンテナ情報を取得します
func GetContainerByID(id string) (*sharedModels.Container, error) {
	var container sharedModels.Container
	if err := sharedModels.Instance.Where("id = ?", id).First(&container).Error; err != nil {
		return nil, err
	}
	return &container, nil
}

// UpdateContainerStatus コンテナのビルドステータスを更新します
func UpdateContainerStatus(id string, status string) error {
	return sharedModels.Instance.Model(&sharedModels.Container{}).Where("id = ?", id).Update("status", status).Error
}

// SaveBuildLog ビルドログを保存します
func SaveBuildLog(id string, logData []byte) error {
	return sharedModels.Instance.Model(&sharedModels.Container{}).Where("id = ?", id).Update("build_log", logData).Error
}
