package models

import "time"

// IngressRouteRouteStatus はルートエントリのステータス
type IngressRouteRouteStatus string

const (
	IngressRouteRouteStatusPending  IngressRouteRouteStatus = "pending"  // 未 apply
	IngressRouteRouteStatusActive   IngressRouteRouteStatus = "active"   // apply 済み
	IngressRouteRouteStatusDeleting IngressRouteRouteStatus = "deleting" // 削除待ち（apply 後に物理削除）
)

// IngressRouteRoute は IngressRoute のパスルーティングエントリを表す中間テーブル
type IngressRouteRoute struct {
	ID             string                  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`               // UUID 主キー
	IngressRouteID string                  `gorm:"type:uuid;not null;index"                        json:"ingress_route_id"` // 親 IngressRoute の ID
	DeploymentID   string                  `gorm:"type:uuid;not null"                              json:"deployment_id"`    // ルーティング先 Deployment の ID
	PathPrefix     string                  `gorm:"type:varchar(255);not null"                      json:"path_prefix"`      // パスプレフィックス（例: /api）
	Port           int                     `gorm:"not null"                                        json:"port"`             // 転送先ポート番号
	Status         IngressRouteRouteStatus `gorm:"type:varchar(32);not null;default:'pending'"     json:"status"`           // ステータス（pending/active/deleting）
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

func (IngressRouteRoute) TableName() string { return "ingress_route_routes" }
