package models

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
)

type DeploymentType string

const (
	DeploymentTypeImageURL   DeploymentType = "image_url"
	DeploymentTypeDockerfile DeploymentType = "dockerfile"
	DeploymentTypeRailpack   DeploymentType = "railpack"
	DeploymentTypeArchive    DeploymentType = "archive" // zip/tar.gzアップロードからのRailpackビルド
)

type DeploymentStatus string

const (
	DeploymentStatusNotInit  DeploymentStatus = "not_init"  // 初回ビルド未完了（Githubビルド専用）
	DeploymentStatusPending  DeploymentStatus = "pending"   // 初回作成・未 apply
	DeploymentStatusRunning  DeploymentStatus = "running"
	DeploymentStatusFailed   DeploymentStatus = "failed"
	DeploymentStatusDeleting DeploymentStatus = "deleting"
)

type AppStatus string

const (
	AppStatusPending   AppStatus = "pending"
	AppStatusBuilding  AppStatus = "building"
	AppStatusDeploying AppStatus = "deploying"
	AppStatusRunning   AppStatus = "running"
	AppStatusError     AppStatus = "error"
)

type Deployment struct {
	ID        string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ProjectID string         `gorm:"type:uuid;not null;index"                       json:"project_id"`
	Name      string         `gorm:"type:varchar(63);not null"                      json:"name"`
	Type      DeploymentType `gorm:"type:varchar(32);not null"                      json:"type"` // 作成後変更不可

	// --- イメージ参照（image_url / railpack / dockerfile 共通）---
	// nil = イメージ未設定。apply 時に Image レコードを引いて実URLをk8sに適用する
	ImageID        *string `gorm:"type:uuid" json:"image_id"`
	Image          *Image  `gorm:"foreignKey:ImageID"        json:"image,omitempty"`
	PendingImageID *string `gorm:"type:uuid" json:"pending_image_id"`
	PendingImage   *Image  `gorm:"foreignKey:PendingImageID" json:"pending_image,omitempty"`

	// --- dockerfile / railpack 共通（GitHub）---
	GithubRepoURL        string `gorm:"type:text"          json:"github_repo_url"`
	PendingGithubRepoURL string `gorm:"type:text"          json:"pending_github_repo_url"`

	GithubBranch        string `gorm:"type:varchar(255)"  json:"github_branch"`
	PendingGithubBranch string `gorm:"type:varchar(255)"  json:"pending_github_branch"`

	// "HEAD" 指定可。apply 時に最新 SHA を取得して上書き
	GithubCommitSHA        string `gorm:"type:varchar(40)"   json:"github_commit_sha"`
	PendingGithubCommitSHA string `gorm:"type:varchar(40)"   json:"pending_github_commit_sha"`

	// ビルド作業ディレクトリ。このディレクトリに CD した状態でビルドを開始する
	GithubRepoDirectory        string `gorm:"type:varchar(255);default:'./'"  json:"github_repo_directory"`
	PendingGithubRepoDirectory string `gorm:"type:varchar(255)"              json:"pending_github_repo_directory"`

	// --- dockerfile 専用 ---
	DockerfilePath        string `gorm:"type:varchar(255);default:'./Dockerfile'" json:"dockerfile_path"`
	PendingDockerfilePath string `gorm:"type:varchar(255)"                        json:"pending_dockerfile_path"`

	// --- ビルド管理 ---
	// nil = ビルドなし。完了時に build_id をセット
	CurrentBuildID *string          `gorm:"type:uuid"                    json:"current_build_id"`
	CurrentBuild   *DeploymentBuild `gorm:"foreignKey:CurrentBuildID"    json:"current_build,omitempty"`

	// --- デプロイ設定 ---
	InstanceSize        string `gorm:"type:varchar(16);not null;default:'small'" json:"instance_size"`
	PendingInstanceSize string `gorm:"type:varchar(16)"                          json:"pending_instance_size"`

	Replicas        int32 `gorm:"not null;default:1" json:"replicas"`
	PendingReplicas int32 `json:"pending_replicas"`

	// --- 起動設定 ---
	Command        pq.StringArray `gorm:"type:text[]" json:"command"`  // k8s command（ENTRYPOINT 上書き）
	PendingCommand pq.StringArray `gorm:"type:text[]" json:"pending_command"`

	Args        pq.StringArray `gorm:"type:text[]" json:"args"`  // k8s args（CMD 上書き）
	PendingArgs pq.StringArray `gorm:"type:text[]" json:"pending_args"`

	// --- ステータス ---
	Status         DeploymentStatus `gorm:"type:varchar(32);not null;default:'pending'" json:"status"`
	AppStatus      AppStatus        `gorm:"type:varchar(32);not null;default:'pending'" json:"app_status"`
	K8sStatus      datatypes.JSON   `gorm:"type:jsonb"                                  json:"k8s_status"` // null = 未同期
	DeleteProgress string           `gorm:"type:varchar(128)"                           json:"delete_progress"` // 削除中のステップ名（deleting 時のみ使用）
	AppliedAt      *time.Time       `json:"applied_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Deployment) TableName() string { return "deployments" }
