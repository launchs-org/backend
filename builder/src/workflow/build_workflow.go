package workflow

import (
	"builder/activity"
	"app/shared/models"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// BuildWorkflow はビルド処理を担う Temporal Workflow
// Activity 連鎖: VerifyHarborCredential → CreateBuildJob → StreamBuildLogs → SetPendingImage → UpdateBuildStatus
func BuildWorkflow(ctx workflow.Context, input activity.BuildWorkflowInput) error {
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: buildActivityTimeout, // Activity タイムアウトを設定する
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3, // 最大3回リトライする
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions) // Activity オプションをコンテキストに設定する

	// 1. Harbor 認証情報の存在を確認する
	if err := workflow.ExecuteActivity(ctx, "VerifyHarborCredentialActivity", input).Get(ctx, nil); err != nil {
		return updateBuildStatusOnError(ctx, input) // エラー時にビルドステータスを failed に更新する
	}

	// 2. K8s ビルドジョブを作成する
	if err := workflow.ExecuteActivity(ctx, "CreateBuildJobActivity", input).Get(ctx, nil); err != nil {
		return updateBuildStatusOnError(ctx, input) // エラー時にビルドステータスを failed に更新する
	}

	// 3. ビルドログをストリームして DB に保存する（長時間実行 Activity: Heartbeat 付き）
	longRunningOptions := workflow.ActivityOptions{
		StartToCloseTimeout:    buildLogStreamTimeout, // ログストリームは長時間かかるため長めに設定する
		HeartbeatTimeout:       heartbeatTimeout,      // Heartbeat タイムアウトを設定する
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1, // ログストリームはリトライしない（べき等でないため）
		},
	}
	longRunningCtx := workflow.WithActivityOptions(ctx, longRunningOptions) // 長時間実行用オプションを設定する
	if err := workflow.ExecuteActivity(longRunningCtx, "StreamBuildLogsActivity", input).Get(ctx, nil); err != nil {
		return updateBuildStatusOnError(ctx, input) // エラー時にビルドステータスを failed に更新する
	}

	// 4. ビルド成功時: Image レコードを作成し pending_image_id と pending_github_* フィールドを更新し、imageID を受け取る
	var imageID string                                                                                      // 作成した Image ID を格納する変数
	if err := workflow.ExecuteActivity(ctx, "SetPendingImageActivity", input).Get(ctx, &imageID); err != nil { // imageID を戻り値として受け取る
		return updateBuildStatusOnError(ctx, input) // エラー時にビルドステータスを failed に更新する
	}

	// 5. ビルドステータスを succeeded に更新し Deployment ステータスを pending に遷移する（成功時のみ imageID を渡す）
	if err := workflow.ExecuteActivity(ctx, "UpdateBuildStatusActivity", input, models.BuildStatusSucceeded, imageID).Get(ctx, nil); err != nil {
		return err // 更新エラーを返す
	}

	return nil // Workflow 正常終了を返す
}

// updateBuildStatusOnError はビルド失敗時にステータスを failed に更新する（ベストエフォート）
func updateBuildStatusOnError(ctx workflow.Context, input activity.BuildWorkflowInput) error {
	_ = workflow.ExecuteActivity(ctx, "UpdateBuildStatusActivity", input, models.BuildStatusFailed, "").Get(ctx, nil) // failed に更新する（builtImageURL は空文字、エラーは無視する）
	return temporal.NewApplicationError("ビルドに失敗しました", "BuildFailed")                                               // ビルド失敗エラーを返す
}
