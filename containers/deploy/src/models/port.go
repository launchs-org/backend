package models

// Port Deployment に紐づく公開ポート
type Port struct {
	ID           string `gorm:"primaryKey" json:"id"`
	DeploymentID string `gorm:"index" json:"deployment_id"`
	Port         int    `json:"port"`
	Protocol     string `gorm:"default:TCP" json:"protocol"` // TCP / UDP
	Description  string `json:"description"`                 // 任意メモ
}
