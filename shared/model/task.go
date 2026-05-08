package model

import (
	"context"
	"time"

	"launchs/shared/database"

	"gorm.io/gorm"
)

type Task struct {
	ID           string     `gorm:"primaryKey;type:varchar(255)" json:"id"`
	TaskType     string     `gorm:"type:varchar(50);not null" json:"task_type"`
	Status       string     `gorm:"type:varchar(20);not null;default:pending" json:"status"`
	Payload      string     `gorm:"type:jsonb;not null" json:"payload"`
	TimeoutAt    time.Time  `gorm:"not null" json:"timeout_at"`
	CreatedAt    time.Time  `gorm:"default:now()" json:"created_at"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	ErrorMessage string     `gorm:"type:text" json:"error_message"`
}

func CreateTask(task *Task) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		tx.Exec("SELECT pg_notify('task_created', ?)", task.TaskType)
		return nil
	})
}

func ClaimTask(ctx context.Context, taskType string) (*Task, error) {
	var task Task
	result := database.DB.WithContext(ctx).Raw(`
		UPDATE tasks SET status = 'running', started_at = now()
		WHERE id = (
			SELECT id FROM tasks
			WHERE status = 'pending' AND task_type = ?
			ORDER BY created_at
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING *`, taskType).Scan(&task)
	if result.Error != nil {
		return nil, result.Error
	}
	if task.ID == "" {
		return nil, nil
	}
	return &task, nil
}

func CompleteTask(id string) error {
	now := time.Now()
	return database.DB.Model(&Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      "done",
		"finished_at": now,
	}).Error
}

func FailTask(id string, errMsg string) error {
	now := time.Now()
	return database.DB.Model(&Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        "failed",
		"finished_at":   now,
		"error_message": errMsg,
	}).Error
}

func CancelTask(id string, errMsg string) error {
	now := time.Now()
	return database.DB.Model(&Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        "cancelled",
		"finished_at":   now,
		"error_message": errMsg,
	}).Error
}

func GetTimedOutRunningTasks(ctx context.Context, taskType string) ([]Task, error) {
	var tasks []Task
	err := database.DB.WithContext(ctx).Raw(`
		SELECT * FROM tasks
		WHERE status = 'running' AND task_type = ? AND timeout_at < now()
		FOR UPDATE SKIP LOCKED`, taskType).Scan(&tasks).Error
	return tasks, err
}
