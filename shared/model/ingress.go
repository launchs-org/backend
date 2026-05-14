package model

import (
	"time"

	"launchs/shared/database"

	"gorm.io/gorm"
)

type Ingress struct {
	ID                  string    `gorm:"primaryKey;type:varchar(255)" json:"id"`
	ContainerID         string    `gorm:"uniqueIndex;type:varchar(255)" json:"container_id"`
	Subdomain           string    `json:"subdomain"`
	HttpPort            int       `json:"http_port"`
	TlsEnabled          bool      `json:"tls_enabled"`
	CustomDomain        string    `json:"custom_domain"`
	CustomDomainEnabled bool      `json:"custom_domain_enabled"`
	Status              string    `gorm:"type:varchar(32);default:'pending'" json:"status"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func GetIngressByContainerID(containerID string) (*Ingress, error) {
	var ingress Ingress
	err := database.DB.Where("container_id = ?", containerID).First(&ingress).Error
	if err != nil {
		return nil, err
	}
	return &ingress, nil
}

func CreateIngress(ingress *Ingress) error {
	return database.DB.Create(ingress).Error
}

func UpdateIngress(ingress *Ingress) error {
	return database.DB.Save(ingress).Error
}

func SetIngressStatus(containerID, status string) error {
	return database.DB.Model(&Ingress{}).Where("container_id = ?", containerID).Update("status", status).Error
}

func DeleteIngress(tx *gorm.DB, containerID string) error {
	return tx.Where("container_id = ?", containerID).Delete(&Ingress{}).Error
}
