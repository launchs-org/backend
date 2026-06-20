package models

import "time"

type PathRuleStatus string

const (
	PathRuleStatusPending  PathRuleStatus = "pending"
	PathRuleStatusActive   PathRuleStatus = "active"
	PathRuleStatusDeleting PathRuleStatus = "deleting"
)

type PathRule struct {
	ID             string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`               // UUID 主キー
	IngressRouteID string         `gorm:"type:uuid;not null;index"                       json:"ingress_route_id"` // 親 IngressRoute ID
	PathPrefix     string         `gorm:"type:varchar(255);not null"                     json:"path_prefix"`      // ルーティング対象パス
	ServiceID      string         `gorm:"type:uuid;not null"                             json:"service_id"`       // 対象 Service の ID
	Status         PathRuleStatus `gorm:"type:varchar(32);not null;default:'pending'"    json:"status"`           // ステータス
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func (PathRule) TableName() string { return "path_rules" } // テーブル名を明示する
