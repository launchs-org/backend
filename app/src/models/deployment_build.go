package models

import "time"

type BuildStatus string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusBuilding  BuildStatus = "building"
	BuildStatusSucceeded BuildStatus = "succeeded"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusCancelled BuildStatus = "cancelled"
)

type BuildType string

const (
	BuildTypeDockerfile BuildType = "dockerfile" // Dockerfile を使ったビルド
	BuildTypeRailpack   BuildType = "railpack"   // Railpack を使ったビルド
)

type DeploymentBuild struct {
	ID           string      `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	DeploymentID string      `gorm:"type:uuid;not null;index"`
	BuildType    BuildType   `gorm:"type:varchar(32);not null"`           // ビルドタイプ（dockerfile / railpack）
	Status       BuildStatus `gorm:"type:varchar(32);not null;default:'pending'"`

	K8sJobName string `gorm:"type:varchar(63)"` // k8s Job 名。キャンセル時に Job を削除するために使用

	BuiltImageURL string `gorm:"type:text"` // ビルド成功時の push 先 URL

	// ビルド時点のソーススナップショット（HEAD は解決済みの実 SHA）
	CommitSHA      string `gorm:"type:varchar(40)"`
	CommitMessage  string `gorm:"type:text"`
	Branch         string `gorm:"type:varchar(255)"`
	Author         string `gorm:"type:varchar(255)"`
	Directory      string `gorm:"type:varchar(255)"` // build_directory スナップショット
	DockerfilePath string `gorm:"type:varchar(255)"`

	BuildLog string `gorm:"type:text"` // k8s Job の Pod ログを収集して保存

	StartedAt  *time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
}

func (DeploymentBuild) TableName() string { return "deployment_builds" }
