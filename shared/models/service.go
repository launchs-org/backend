package models

import (
	"time"

	"gorm.io/datatypes"
)

type ServiceStatus string

const (
	ServiceStatusPending  ServiceStatus = "pending"
	ServiceStatusActive   ServiceStatus = "active"
	ServiceStatusDeleting ServiceStatus = "deleting"
)

// ServicePort は ports カラム（jsonb）の要素型
// 例: [{"protocol": "TCP", "port": 8080}, {"protocol": "UDP", "port": 9090}]
type ServicePort struct {
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
}

// ServiceType は k8s Service の type を表す型
type ServiceType string

const (
	ServiceTypeClusterIP    ServiceType = "ClusterIP"    // クラスター内部のみアクセス可能
	ServiceTypeNodePort     ServiceType = "NodePort"     // ノードのポートで外部公開
	ServiceTypeLoadBalancer ServiceType = "LoadBalancer" // ロードバランサーで外部公開
)

type Service struct {
	ID                string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`              // UUID 主キー
	DeploymentID      string         `gorm:"type:uuid;not null;uniqueIndex"                  json:"deployment_id"`   // デプロイメント ID
	Port              int            `gorm:"type:int"                                        json:"port"`            // 現在のポート番号
	TargetPort        int            `gorm:"type:int"                                        json:"target_port"`     // 現在のターゲットポート番号
	Type              ServiceType    `gorm:"type:varchar(32)"                                json:"type"`            // 現在の Service タイプ
	PendingPort       int            `gorm:"type:int"                                        json:"pending_port"`    // 未 apply のポート番号
	PendingTargetPort int            `gorm:"type:int"                                        json:"pending_target_port"` // 未 apply のターゲットポート番号
	Ports             datatypes.JSON `gorm:"type:jsonb"                                      json:"ports"`           // 現在稼働中のポート一覧（jsonb）
	PendingPorts      datatypes.JSON `gorm:"type:jsonb"                                      json:"pending_ports"`   // 未 apply のポート一覧
	Status            ServiceStatus  `gorm:"type:varchar(32);not null;default:'pending'"     json:"status"`          // サービスのステータス
	ClusterIP         string         `gorm:"type:varchar(64)"                                json:"cluster_ip"`      // k8s が割り当てた ClusterIP
	K8sStatus         datatypes.JSON `gorm:"type:jsonb"                                      json:"k8s_status"`      // null = 未同期
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

func (Service) TableName() string { return "services" }
