package model

import (
	"time"

	"launchs/shared/database"

	"gorm.io/gorm"
)

type Service struct {
	ID          string    `gorm:"primaryKey;type:varchar(255)" json:"id"`
	ContainerID string    `gorm:"uniqueIndex;type:varchar(255)" json:"container_id"`
	Type        string    `json:"type"`
	Ports       string    `gorm:"type:text" json:"ports"`
	IsActive    bool      `json:"is_active"`
	InternalIP  string    `json:"internal_ip"`
	ExternalIP  string    `json:"external_ip"`
	Status      string    `gorm:"type:varchar(32);default:'pending'" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func GetServiceByContainerID(containerID string) (*Service, error) {
	var service Service
	err := database.DB.Where("container_id = ?", containerID).First(&service).Error
	if err != nil {
		return nil, err
	}
	return &service, nil
}

func SetServiceStatus(containerID, status string) error {
	return database.DB.Model(&Service{}).Where("container_id = ?", containerID).Update("status", status).Error
}

func DeleteServiceByContainerID(tx *gorm.DB, containerID string) error {
	return tx.Where("container_id = ?", containerID).Delete(&Service{}).Error
}

func UpdateService(service *Service) error {
	service.UpdatedAt = time.Now()
	return database.DB.Save(service).Error
}
