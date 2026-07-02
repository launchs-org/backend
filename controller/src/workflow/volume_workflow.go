package workflow

import (
	"controller/activity"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CreateVolumeWorkflowInput は CreateVolumeWorkflow への入力
type CreateVolumeWorkflowInput struct {
	VolumeID string // 対象ボリュームのID（あらかじめ DB に pending 状態で作成済み）
}

// CreateVolumeWorkflow は Volume の作成処理を Temporal Workflow として実装する
// WorkflowID は "create-volume-{volumeID}" で冪等性を保証する
func CreateVolumeWorkflow(ctx workflow.Context, input CreateVolumeWorkflowInput) error {
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute, // Activity 実行タイムアウト
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3, // 最大リトライ回数
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions) // Activity オプションをコンテキストに設定する

	volumeInput := activity.VolumeWorkflowInput{VolumeID: input.VolumeID} // Activity 入力を構築する

	// 1. k8s PVC を作成する
	if err := workflow.ExecuteActivity(ctx, (*activity.VolumeActivities).CreateK8sPVCActivity, volumeInput).Get(ctx, nil); err != nil { // PVC 作成 Activity を実行する
		return err // エラーを返す
	}

	return nil // 正常終了を返す
}

// DeleteVolumeWorkflowInput は DeleteVolumeWorkflow への入力
type DeleteVolumeWorkflowInput struct {
	VolumeID string // 削除対象ボリュームのID
}

// DeleteVolumeWorkflow は Volume の削除処理を Temporal Workflow として実装する
// WorkflowID は "delete-volume-{volumeID}" で冪等性を保証する
func DeleteVolumeWorkflow(ctx workflow.Context, input DeleteVolumeWorkflowInput) error {
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute, // Activity 実行タイムアウト
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3, // 最大リトライ回数
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions) // Activity オプションをコンテキストに設定する

	volumeInput := activity.VolumeWorkflowInput{VolumeID: input.VolumeID} // Activity 入力を構築する

	// 1. k8s PVC を削除する
	if err := workflow.ExecuteActivity(ctx, (*activity.VolumeActivities).DeleteK8sPVCActivity, volumeInput).Get(ctx, nil); err != nil { // PVC 削除 Activity を実行する
		return err // エラーを返す
	}

	// 2. DB レコードを削除する
	if err := workflow.ExecuteActivity(ctx, (*activity.VolumeActivities).DeleteVolumeRecordActivity, volumeInput).Get(ctx, nil); err != nil { // DB 削除 Activity を実行する
		return err // エラーを返す
	}

	return nil // 正常終了を返す
}
