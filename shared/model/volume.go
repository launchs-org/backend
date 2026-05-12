package model

import (
	"launchs/shared/database"
	"time"

	"gorm.io/gorm"
)

type Volume struct {
	ID          string    `gorm:"primaryKey;type:varchar(255)" json:"id"`
	ProjectID   string    `gorm:"index;type:varchar(255);not null" json:"project_id"`
	ContainerID string    `gorm:"index;type:varchar(255)" json:"container_id"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	SizeMB      int       `gorm:"not null" json:"size_mb"`
	MountPath   string    `gorm:"type:varchar(255);not null" json:"mount_path"`
	Status      string    `gorm:"type:varchar(50);default:'Available'" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func CreateVolume(volume *Volume) error {
	return database.DB.Create(volume).Error
}

func GetVolumesByProjectID(projectID string) ([]Volume, error) {
	var volumes []Volume
	err := database.DB.Where("project_id = ?", projectID).Find(&volumes).Error
	return volumes, err
}

func GetVolumesByContainerID(containerID string) ([]Volume, error) {
	var volumes []Volume
	err := database.DB.Where("container_id = ?", containerID).Find(&volumes).Error
	return volumes, err
}

func GetVolumeByID(id string) (*Volume, error) {
	var volume Volume
	err := database.DB.Where("id = ?", id).First(&volume).Error
	if err != nil {
		return nil, err
	}
	return &volume, nil
}

func UpdateVolume(volume *Volume) error {
	return database.DB.Save(volume).Error
}

func DeleteVolume(tx *gorm.DB,id string) error {
	return database.DB.Where("id = ?", id).Delete(&Volume{}).Error
}
