package models

import (
	sharedModels "shared/models"
)

// CreateUser 新しいユーザーをデータベースに保存します
func CreateUser(user *sharedModels.User) error {
	return sharedModels.Instance.Create(user).Error
}

// GetUserByUsername ユーザー名でユーザーを取得します
func GetUserByUsername(username string) (*sharedModels.User, error) {
	var user sharedModels.User
	if err := sharedModels.Instance.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID IDでユーザーを取得します
func GetUserByID(id uint) (*sharedModels.User, error) {
	var user sharedModels.User
	if err := sharedModels.Instance.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
