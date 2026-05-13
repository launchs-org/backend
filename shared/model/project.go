package model

import (
	"launchs/shared/database"
	"time"

	"gorm.io/gorm"
)

type Project struct {
	ID              string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Name            string         `gorm:"type:varchar(255);not null" json:"name"`
	K8sResourceName string         `gorm:"type:varchar(255);not null" json:"k8s_resource_name"`
	Namespace       string         `gorm:"type:varchar(255);not null" json:"namespace"`
	OwnerID         string         `gorm:"type:varchar(255);not null;index" json:"owner_id"`
	Status          string         `gorm:"type:varchar(255);default:'Pending'" json:"status"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	Containers      []Container    `json:"containers" gorm:"foreignKey:ProjectID"`
}

func CreateProject(project *Project) error {
	return database.DB.Create(project).Error
}

func GetProjectByName(name string) (*Project, error) {
	var project Project
	err := database.DB.Where("name = ?", name).First(&project).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func GetProjectByID(id string) (*Project, error) {
	var project Project
	err := database.DB.Preload("Containers.Service").
		Preload("Containers.Ingress").
		Preload("Containers.Volumes").
		Where("id = ?", id).First(&project).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func GetProjectsByOwnerID(ownerID string) ([]Project, error) {
	var projects []Project
	err := database.DB.Where("owner_id = ?", ownerID).Find(&projects).Error
	if err != nil {
		return nil, err
	}
	return projects, nil
}

func UpdateProjectStatus(id, status string) error {
	return database.DB.Model(&Project{}).Where("id = ?", id).Update("status", status).Error
}

func DeleteProject(id string) error {
	return database.DB.Where("id = ?", id).Delete(&Project{}).Error
}
