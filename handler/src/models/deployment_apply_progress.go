package models

import "time"

// ApplyProgressStepStatus は apply 進捗ステップの状態を表す
type ApplyProgressStepStatus string

const (
	ApplyProgressStepStatusPending    ApplyProgressStepStatus = "pending"     // 未着手
	ApplyProgressStepStatusInProgress ApplyProgressStepStatus = "in_progress" // 実行中
	ApplyProgressStepStatusDone       ApplyProgressStepStatus = "done"        // 完了
	ApplyProgressStepStatusFailed     ApplyProgressStepStatus = "failed"      // 失敗
	ApplyProgressStepStatusSkipped    ApplyProgressStepStatus = "skipped"     // 対象リソースがなくスキップ
)

// ApplyProgressStepName は apply 進捗ステップの種別を表す
type ApplyProgressStepName string

const (
	ApplyProgressStepVolume       ApplyProgressStepName = "volume"        // ボリュームをプロビジョニングしています
	ApplyProgressStepEnvVar       ApplyProgressStepName = "env_var"       // 環境変数を適用しています
	ApplyProgressStepImage        ApplyProgressStepName = "image"         // イメージを取得しています
	ApplyProgressStepContainer    ApplyProgressStepName = "container"     // コンテナを作成しています
	ApplyProgressStepPodScheduled ApplyProgressStepName = "pod_scheduled" // コンテナを起動しています
	ApplyProgressStepPodRunning   ApplyProgressStepName = "pod_running"   // コンテナの起動を確認しています
	ApplyProgressStepService      ApplyProgressStepName = "service"       // ポートを開いています
	ApplyProgressStepNetwork      ApplyProgressStepName = "network"       // ネットワーク接続を確認しています
	ApplyProgressStepReadiness    ApplyProgressStepName = "readiness"     // 起動状態を確認しています
)

// ApplyProgressStepOrder は apply 進捗ステップの表示順を定義する
var ApplyProgressStepOrder = []ApplyProgressStepName{
	ApplyProgressStepVolume,
	ApplyProgressStepEnvVar,
	ApplyProgressStepImage,
	ApplyProgressStepContainer,
	ApplyProgressStepPodScheduled,
	ApplyProgressStepPodRunning,
	ApplyProgressStepService,
	ApplyProgressStepNetwork,
	ApplyProgressStepReadiness,
}

// DeploymentApplyProgress は1回の apply 実行（workflow_id）における1ステップの進捗を表す
// workflow_id ごとに9レコード（step_no=1〜9）を保持し、履歴として残す
type DeploymentApplyProgress struct {
	ID           string                  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"                     json:"id"`
	WorkflowID   string                  `gorm:"type:varchar(255);not null;uniqueIndex:idx_progress_workflow_step" json:"workflow_id"`
	DeploymentID string                  `gorm:"type:uuid;not null;index"                                           json:"deployment_id"`
	StepNo       int                     `gorm:"not null"                                                           json:"step_no"`
	StepName     ApplyProgressStepName   `gorm:"type:varchar(32);not null;uniqueIndex:idx_progress_workflow_step"  json:"step_name"`
	Status       ApplyProgressStepStatus `gorm:"type:varchar(16);not null;default:'pending'"                       json:"status"`
	ErrorMessage string                  `gorm:"type:text"                                                          json:"error_message"`
	StartedAt    *time.Time              `json:"started_at"`
	FinishedAt   *time.Time              `json:"finished_at"`
	CreatedAt    time.Time               `json:"created_at"`
	UpdatedAt    time.Time               `json:"updated_at"`
}

func (DeploymentApplyProgress) TableName() string { return "deployment_apply_progress" }
