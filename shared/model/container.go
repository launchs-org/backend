package model

import (
	"time"

	"launchs/shared/database"

	"gorm.io/gorm"
)

type Container struct {
	ID            string    `gorm:"primaryKey;type:varchar(255)" json:"id"`
	ProjectID     string    `gorm:"index;type:varchar(255)" json:"project_id"`
	Name          string    `gorm:"index;type:varchar(255)" json:"name"`
	ImageID       string    `gorm:"index;type:varchar(255)" json:"image_id"`
	RepositoryURL string    `json:"repository_url"`
	Branch        string    `gorm:"default:'main'" json:"branch"`
	Directory     string    `gorm:"default:'/'" json:"directory"`
	Version       string    `json:"version"`
	Replicas      int       `gorm:"default:1" json:"replicas"`
	EnvVars       string    `gorm:"type:text" json:"env_vars"`
	Resources     string    `gorm:"type:text" json:"resources"`
	Status        string    `gorm:"default:'Stopped'" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Service       *Service  `gorm:"foreignKey:ContainerID" json:"service"`
	Ingress       *Ingress  `gorm:"foreignKey:ContainerID" json:"ingress"`
	Volumes       []Volume  `gorm:"foreignKey:ContainerID" json:"volumes"`
}

func GetContainerCountByProjectIDAndName(projectID, name string) (int64, error) {
	var count int64
	err := database.DB.Model(&Container{}).Where("project_id = ? AND name = ?", projectID, name).Count(&count).Error
	return count, err
}

func CreateContainerWithRelatedRecords(image *Image, container *Container, service *Service, buildJob *BuildJob) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(image).Error; err != nil {
			return err
		}
		if err := tx.Create(container).Error; err != nil {
			return err
		}
		if err := tx.Create(service).Error; err != nil {
			return err
		}
		if err := tx.Create(buildJob).Error; err != nil {
			return err
		}
		return nil
	})
}

func UpdateContainerStatus(id, status string) error {
	return database.DB.Model(&Container{}).Where("id = ?", id).Update("status", status).Error
}

func DeleteContainer(id string) error {
	return database.DB.Where("id = ?", id).Delete(&Container{}).Error
}

func GetContainerByID(id string) (*Container, error) {
	var container Container
	err := database.DB.Preload("Service").Preload("Ingress").Preload("Volumes").Where("id = ?", id).First(&container).Error
	if err != nil {
		return nil, err
	}
	return &container, nil
}
