package models

import "time"

// DeploymentMetrics は Deployment に属する Pod のリソース使用量を時系列で記録するモデル
type DeploymentMetrics struct {
	ID            string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`             // UUID 主キー
	DeploymentID  string    `gorm:"type:uuid;not null;index"                       json:"deployment_id"` // 紐づく Deployment の ID
	PodName       string    `gorm:"type:varchar(255);not null"                     json:"pod_name"`      // Pod 名
	CPUMillicores int64     `gorm:"not null"                                       json:"cpu_millicores"` // CPU 使用量（ミリコア単位）
	MemoryBytes   int64     `gorm:"not null"                                       json:"memory_bytes"`  // メモリ使用量（バイト単位）
	ReadyReplicas int32     `gorm:"not null"                                       json:"ready_replicas"` // Ready 状態のレプリカ数
	TotalReplicas int32     `gorm:"not null"                                       json:"total_replicas"` // 合計レプリカ数
	RecordedAt    time.Time `gorm:"not null;index"                                 json:"recorded_at"`  // 記録日時
	CreatedAt     time.Time `json:"created_at"`                                                          // レコード作成日時
}

// TableName はテーブル名を明示する
func (deploymentMetrics *DeploymentMetrics) TableName() string {
	return "deployment_metrics" // テーブル名を返す
}
