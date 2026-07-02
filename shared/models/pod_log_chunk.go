package models

import "time"

// PodLogChunk は稼働中 Pod のログを chunk 単位で保存するモデル
type PodLogChunk struct {
	ID           string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`           // UUID主キー
	DeploymentID string    `gorm:"type:uuid;not null;index"                       json:"deployment_id"` // デプロイメントID
	PodName      string    `gorm:"type:text;not null;default:''"                  json:"pod_name"`      // ログの送信元 Pod 名
	Content      string    `gorm:"type:text;not null"                             json:"content"`       // ログの内容
	CreatedAt    time.Time `                                                      json:"created_at"`    // 作成日時（since差分取得に使用）
}

func (PodLogChunk) TableName() string { return "pod_log_chunks" } // テーブル名を明示する
