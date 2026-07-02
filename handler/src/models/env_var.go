package models

import "time"

type EnvVar struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ProjectID string    `gorm:"type:uuid;not null;uniqueIndex:idx_env_var_project_key" json:"project_id"`
	Key       string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_env_var_project_key" json:"key"`
	Value     string    `gorm:"type:text"                                      json:"value"`    // 即時更新。k8s 反映は apply 時
	IsSecret  bool      `gorm:"not null;default:false"                         json:"is_secret"` // true → k8s Secret / UI マスク
	Status    string    `gorm:"type:varchar(32);not null;default:'active'"     json:"status"` // active / deleting
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (EnvVar) TableName() string { return "env_vars" }
