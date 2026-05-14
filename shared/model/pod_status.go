package model

import (
	"time"

	"launchs/shared/database"
)

// PodStatus は K8s Pod 1つ分のステータスを保持します。
// watcher が Deployment イベントのたびに upsert します。
type PodStatus struct {
	ID          string    `gorm:"primaryKey;type:varchar(255)" json:"id"` // Pod name
	ContainerID string    `gorm:"index;type:varchar(255)" json:"container_id"`
	Name        string    `gorm:"type:varchar(255)" json:"name"`
	Phase       string    `gorm:"type:varchar(64)" json:"phase"`  // Running, Pending, Failed, Succeeded, Unknown
	Ready       bool      `json:"ready"`
	Restarts    int32     `json:"restarts"`
	Message     string    `gorm:"type:text" json:"message"` // 失敗時のメッセージ
	StartedAt   *time.Time `json:"started_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func UpsertPodStatuses(pods []PodStatus) error {
	if len(pods) == 0 {
		return nil
	}
	return database.DB.Save(&pods).Error
}

func DeleteStalePodStatuses(containerID string, activePodIDs []string) error {
	if len(activePodIDs) == 0 {
		return database.DB.Where("container_id = ?", containerID).Delete(&PodStatus{}).Error
	}
	return database.DB.
		Where("container_id = ? AND id NOT IN ?", containerID, activePodIDs).
		Delete(&PodStatus{}).Error
}

func GetPodStatusesByContainerID(containerID string) ([]PodStatus, error) {
	var pods []PodStatus
	err := database.DB.Where("container_id = ?", containerID).
		Order("name ASC").Find(&pods).Error
	return pods, err
}
