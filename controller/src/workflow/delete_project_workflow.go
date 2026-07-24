package workflow

import (
	"controller/activity"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DeleteProjectWorkflowInput は DeleteProjectWorkflow への入力
type DeleteProjectWorkflowInput struct {
	ProjectID string // 削除対象プロジェクトのID
}

// DeleteProjectWorkflow は Project の削除処理を Temporal Workflow として実装する
// WorkflowID は "delete-project-{projectID}" で冪等性を保証する
func DeleteProjectWorkflow(ctx workflow.Context, input DeleteProjectWorkflowInput) error {
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute, // Activity 実行タイムアウト
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3, // 最大リトライ回数
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions) // Activity オプションをコンテキストに設定する

	deleteInput := activity.DeleteProjectInput{ProjectID: input.ProjectID} // Activity 入力を構築する

	// 1. k8s Namespace を削除する（PVC・Deployment 含む）
	if err := workflow.ExecuteActivity(ctx, (*activity.ProjectActivities).DeleteK8sNamespaceActivity, deleteInput).Get(ctx, nil); err != nil { // Namespace 削除 Activity を実行する
		return err // エラーを返す
	}

	// 2. Harbor プロジェクトを削除する
	if err := workflow.ExecuteActivity(ctx, (*activity.ProjectActivities).DeleteHarborProjectActivity, deleteInput).Get(ctx, nil); err != nil { // Harbor 削除 Activity を実行する
		return err // エラーを返す
	}

	// 3. DB レコードを削除する
	if err := workflow.ExecuteActivity(ctx, (*activity.ProjectActivities).DeleteProjectRecordActivity, deleteInput).Get(ctx, nil); err != nil { // DB 削除 Activity を実行する
		return err // エラーを返す
	}

	return nil // 正常終了を返す
}
