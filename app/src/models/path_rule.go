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
	IngressRouteID string         `gorm:"type:uuid;not null;uniqueIndex:idx_path_rules_unique_path" json:"ingress_route_id"` // 親 IngressRoute ID（path_prefix との複合ユニーク）
	PathPrefix     string         `gorm:"type:varchar(255);not null;uniqueIndex:idx_path_rules_unique_path" json:"path_prefix"`      // ルーティング対象パス（同一 IngressRoute 内で重複不可）
	ServiceID      string         `gorm:"type:uuid;not null"                             json:"service_id"`       // 対象 Service の ID
	Status         PathRuleStatus `gorm:"type:varchar(32);not null;default:'pending'"    json:"status"`           // ステータス
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func (PathRule) TableName() string { return "path_rules" } // テーブル名を明示する
