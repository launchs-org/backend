package models

import (
	"time"

	"gorm.io/gorm"
)

// Container は実行単位を管理するモデルです
type Container struct {
	ID            string         `gorm:"primaryKey" json:"id"`
	ProjectID     string         `gorm:"index" json:"project_id"`
	Name          string         `gorm:"index" json:"name"`

	// ソース・イメージ情報
	ImageID       string         `gorm:"index" json:"image_id"`
	RepositoryURL string         `json:"repository_url"` // GitHub URL
	Branch        string         `gorm:"default:'main'" json:"branch"`
	Directory     string         `gorm:"default:'/'" json:"directory"`
	Version       string         `json:"version"` // 適用中のタグ/バージョン

	// 実行設定
	Replicas      int            `gorm:"default:1" json:"replicas"`
	EnvVars       string         `gorm:"type:text" json:"env_vars"` // JSON: {"KEY": "VALUE"}

	// リソース制限
	Resources     string         `gorm:"type:text" json:"resources"`
	// JSON: {"requests":{"cpu":"100m","memory":"128Mi"},"limits":{"cpu":"500m","memory":"512Mi"}}

	// ステータス
	// "Building" | "Running" | "Failed" | "Stopped"
	Status        string         `gorm:"default:'Stopped'" json:"status"`

	Service       Service        `gorm:"foreignKey:ContainerID" json:"service"`
	Ingress       *Ingress       `gorm:"foreignKey:ContainerID" json:"ingress"`
	CreatedAt     time.Time      `json:"created_at"`
}

// Service はコンテナのネットワークサービスを管理するモデルです
type Service struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	ContainerID string    `gorm:"uniqueIndex" json:"container_id"`
	Type        string    `json:"type"` // "ClusterIP", "NodePort", "LoadBalancer"
	Ports       string    `gorm:"type:text" json:"ports"` // JSON: [{"name":"http","port":80,"target":3000}]
	InternalIP  string    `json:"internal_ip"`
	ExternalIP  string    `json:"external_ip"`
	CreatedAt   time.Time `json:"created_at"`
}

// Ingress は外部公開設定を管理するモデルです
type Ingress struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	ContainerID string    `gorm:"uniqueIndex" json:"container_id"`
	Subdomain   string    `gorm:"uniqueIndex" json:"subdomain"` // 自動生成: {proj-name}-{cont-name}.launchs.org
	HttpPort    int       `json:"http_port"`
	TlsEnabled  bool      `gorm:"default:false" json:"tls_enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// Create はコンテナを新規作成します
func (container *Container) Create(db *gorm.DB) error {
	return db.Create(container).Error
}

// FindByID はコンテナを取得します
func (container *Container) FindByID(db *gorm.DB, id string) error {
	return db.Preload("Service").Preload("Ingress").First(container, "id = ?", id).Error
}

// Update はコンテナを更新します
func (container *Container) Update(db *gorm.DB, updates map[string]interface{}) error {
	return db.Model(container).Updates(updates).Error
}

// Delete はコンテナを物理削除します
func (container *Container) Delete(db *gorm.DB) error {
	return db.Unscoped().Delete(container).Error
}

// DeleteAllByProjectID はプロジェクトに属する全コンテナを削除します
func (container *Container) DeleteAllByProjectID(db *gorm.DB, projectID string) error {
	return db.Where("project_id = ?", projectID).Delete(&Container{}).Error
}
