package models

// EnvVar Deployment に紐づく環境変数
type EnvVar struct {
	ID           string `gorm:"primaryKey" json:"id"`
	DeploymentID string `gorm:"index" json:"deployment_id"`
	Key          string `json:"key"`
	Value        string `json:"value"` // 平文保存 or 暗号化
}
