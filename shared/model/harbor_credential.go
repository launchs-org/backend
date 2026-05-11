package model

import (
	"time"

	"launchs/shared/database"

	"gorm.io/gorm"
)

// HarborCredential はプロジェクトID に紐づく Harbor の robot アカウント情報です。
// ビルド時にプロジェクトごとの push 権限を持つ robot を使い回します。
type HarborCredential struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	ProjectID     string         `gorm:"uniqueIndex;not null;type:varchar(255)" json:"project_id"`
	HarborProject string         `gorm:"not null;type:varchar(255)" json:"harbor_project"`
	RobotID       int64          `gorm:"not null" json:"robot_id"`
	RobotName     string         `gorm:"not null;type:varchar(255)" json:"robot_name"`
	RobotSecret   string         `gorm:"not null;type:varchar(255)" json:"robot_secret"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func GetHarborCredentialByProjectID(projectID string) (*HarborCredential, error) {
	var cred HarborCredential
	result := database.DB.Where("project_id = ?", projectID).First(&cred)
	if result.Error == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &cred, result.Error
}

func SaveHarborCredential(cred *HarborCredential) error {
	return database.DB.Save(cred).Error
}

func DeleteHarborCredentialByProjectID(projectID string) error {
	return database.DB.Where("project_id = ?", projectID).Delete(&HarborCredential{}).Error
}
