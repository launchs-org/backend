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
	ID           string      `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ProjectID    string      `gorm:"type:uuid;not null;index"                       json:"project_id"`    // 親プロジェクトID（Deployment削除後もビルドを保持するため）
	DeploymentID *string     `gorm:"type:uuid;index"                                json:"deployment_id"` // ビルド元DeploymentID（nullable: Deployment削除時にNULLになる）
	BuildType    BuildType   `gorm:"type:varchar(32);not null"                      json:"build_type"`    // ビルドタイプ（dockerfile / railpack）
	Status       BuildStatus `gorm:"type:varchar(32);not null;default:'pending'"    json:"status"`

	K8sJobName string `gorm:"type:varchar(63)" json:"k8s_job_name"` // k8s Job 名。キャンセル時に Job を削除するために使用

	BuiltImageURL    string `gorm:"type:text"    json:"built_image_url"`    // ビルド成功時の push 先 URL
	ImageSizeBytes   int64  `gorm:"default:0"    json:"image_size_bytes"`   // Harbor に格納されたイメージサイズ（バイト単位）

	// ビルド時点のソーススナップショット（HEAD は解決済みの実 SHA）
	GithubRepoURL  string `gorm:"type:text"          json:"github_repo_url"`  // GitHub リポジトリ URL スナップショット
	CommitSHA      string `gorm:"type:varchar(40)"   json:"commit_sha"`
	CommitMessage  string `gorm:"type:text"          json:"commit_message"`
	Branch         string `gorm:"type:varchar(255)"  json:"branch"`
	Author         string `gorm:"type:varchar(255)"  json:"author"`
	Directory      string `gorm:"type:varchar(255)"  json:"directory"` // build_directory スナップショット
	DockerfilePath string `gorm:"type:varchar(255)"  json:"dockerfile_path"`

	BuildLog string `gorm:"type:text" json:"build_log"` // k8s Job の Pod ログを収集して保存

	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (DeploymentBuild) TableName() string { return "deployment_builds" }
