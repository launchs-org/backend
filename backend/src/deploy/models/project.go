package models

import (
	"time"

	"gorm.io/gorm"
)

// Project はプロジェクト全体を管理するモデルです
type Project struct {
	ID              string           `gorm:"primaryKey" json:"id"`
	Name            string           `gorm:"uniqueIndex" json:"name"`
	K8sResourceName string           `gorm:"uniqueIndex" json:"k8s_resource_name"`
	Namespace       string           `gorm:"uniqueIndex" json:"namespace"`
	OwnerID         string           `gorm:"index" json:"owner_id"`
	Containers      []Container      `gorm:"foreignKey:ProjectID" json:"containers"`
	BuildJobs       []BuildJob       `gorm:"foreignKey:ProjectID" json:"build_jobs"`
	Histories       []ProjectHistory `gorm:"foreignKey:ProjectID" json:"histories"`
	CreatedAt       time.Time        `json:"created_at"`
	DeletedAt       gorm.DeletedAt   `gorm:"index" json:"-"`
}

// FindAllByOwner は指定されたオーナーの全プロジェクトを取得します
func (project *Project) FindAllByOwner(db *gorm.DB, ownerID string) ([]Project, error) {
	var projects []Project
	err := db.Where("owner_id = ?", ownerID).Preload("Containers").Find(&projects).Error
	return projects, err
}

// FindByID は指定された ID のプロジェクトを取得します
func (project *Project) FindByID(db *gorm.DB, id string, ownerID string) error {
	return db.Where("id = ? AND owner_id = ?", id, ownerID).
		Preload("Containers").
		Preload("Containers.Service").
		Preload("Containers.Ingress").
		First(project).Error
}

// Create はプロジェクトを新規作成します
func (project *Project) Create(db *gorm.DB) error {
	return db.Create(project).Error
}

// Delete はプロジェクトをハードデリートします
func (project *Project) Delete(db *gorm.DB) error {
	return db.Unscoped().Delete(project).Error
}
