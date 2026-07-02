package workflow

import (
	"builder/activity"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// タイムアウト定数
const (
	buildActivityTimeout  = 5 * time.Minute   // 通常 Activity のタイムアウト
	buildLogStreamTimeout = 60 * time.Minute  // ログストリーム Activity のタイムアウト（長時間ビルドに対応）
	heartbeatTimeout      = 30 * time.Second  // Heartbeat タイムアウト
	cancelActivityTimeout = 2 * time.Minute   // キャンセル Activity のタイムアウト
)

// CancelBuildWorkflow はビルドキャンセル処理を担う Temporal Workflow
// Activity 連鎖: DeleteBuildJob → SetBuildCancelled
func CancelBuildWorkflow(ctx workflow.Context, input activity.CancelBuildWorkflowInput) error {
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: cancelActivityTimeout, // Activity タイムアウトを設定する
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3, // 最大3回リトライする
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions) // Activity オプションをコンテキストに設定する

	// 1. K8s Job を削除する（Job 名が設定されている場合のみ）
	if err := workflow.ExecuteActivity(ctx, "DeleteBuildJobActivity", input).Get(ctx, nil); err != nil {
		return err // Job 削除エラーを返す
	}

	// 2. ビルドステータスを cancelled に更新する
	if err := workflow.ExecuteActivity(ctx, "SetBuildCancelledActivity", input).Get(ctx, nil); err != nil {
		return err // ステータス更新エラーを返す
	}

	return nil // Workflow 正常終了を返す
}
