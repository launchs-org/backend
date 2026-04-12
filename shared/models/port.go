package models

// Port Deployment に紐づく開放ポート設定
type Port struct {
	ID           string `gorm:"primaryKey" json:"id"`
	DeploymentID string `gorm:"index" json:"deployment_id"`
	Protocol     string `json:"protocol"` // TCP / UDP
	InternalPort int    `json:"internal_port"`
	ExternalPort int    `json:"external_port"`
}
