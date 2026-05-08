package model

import (
	"time"

	"launchs/shared/database"
)

type Image struct {
	ID          string    `gorm:"primaryKey;type:varchar(255)" json:"id"`
	ContainerID string    `gorm:"index;type:varchar(255)" json:"container_id"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	Registry    string    `json:"registry"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func DeleteImagesByContainerID(containerID string) error {
	return database.DB.Where("container_id = ?", containerID).Delete(&Image{}).Error
}

func GetImagesByContainerID(containerID string) ([]Image, error) {
	var images []Image
	err := database.DB.Where("container_id = ?", containerID).Find(&images).Error
	return images, err
}
