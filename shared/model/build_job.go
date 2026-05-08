package model

import (
	"context"
	"time"

	"launchs/shared/database"

	"gorm.io/gorm"
)

type BuildJob struct {
	ID            string     `gorm:"primaryKey;type:varchar(255)" json:"id"`
	ProjectID     string     `gorm:"index;type:varchar(255)" json:"project_id"`
	ContainerID   string     `gorm:"index;type:varchar(255)" json:"container_id"`
	RepositoryURL string     `json:"repository_url"`
	Branch        string     `json:"branch"`
	Directory     string     `json:"directory"`
	Status        string     `json:"status"`
	BuildLog      []byte     `gorm:"type:bytea" json:"-"`
	StartedAt     *time.Time `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func UpdateBuildJobStatus(id string, updates map[string]interface{}) error {
	return database.DB.Model(&BuildJob{}).Where("id = ?", id).Updates(updates).Error
}

func GetBuildJobsByContainerID(containerID string) ([]BuildJob, error) {
	var jobs []BuildJob
	err := database.DB.Where("container_id = ?", containerID).Order("created_at desc").Find(&jobs).Error
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

func GetBuildJobByID(id string) (*BuildJob, error) {
	var job BuildJob
	err := database.DB.Where("id = ?", id).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func AppendBuildLog(id string, log []byte) error {
	err := database.DB.Model(&BuildJob{}).Where("id = ?", id).Update("build_log", gorm.Expr("COALESCE(build_log, '\\x'::bytea) || ?", log)).Error
	if err != nil {
		database.DB.Logger.Error(context.Background(), "failed to append build log: %v", err)
	}
	return err
}

func DeleteBuildJobsByContainerID(containerID string) error {
	return database.DB.Where("container_id = ?", containerID).Delete(&BuildJob{}).Error
}

func GetBuildJobLog(id string) ([]byte, error) {
	var job BuildJob
	err := database.DB.Select("build_log").Where("id = ?", id).First(&job).Error
	if err != nil {
		return nil, err
	}
	return job.BuildLog, nil
}
