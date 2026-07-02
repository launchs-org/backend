package workflow

import (
	"controller/activity"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DeleteDeploymentWorkflowInput は DeleteDeploymentWorkflow への入力
type DeleteDeploymentWorkflowInput struct {
	DeploymentID string // 削除対象デプロイメントのID
}

// DeleteDeploymentWorkflow は Deployment の削除処理を Temporal Workflow として実装する
// WorkflowID は "delete-deployment-{deploymentID}" で冪等性を保証する
func DeleteDeploymentWorkflow(ctx workflow.Context, input DeleteDeploymentWorkflowInput) error {
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute, // Activity 実行タイムアウト
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3, // 最大リトライ回数
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions) // Activity オプションをコンテキストに設定する

	deleteInput := activity.DeleteDeploymentInput{DeploymentID: input.DeploymentID} // Activity 入力を構築する

	// 1. DB ステータスを deleting に更新する
	if err := workflow.ExecuteActivity(ctx, (*activity.DeploymentActivities).SetDeploymentDeletingActivity, deleteInput).Get(ctx, nil); err != nil { // ステータス更新 Activity を実行する
		return err // エラーを返す
	}

	// 2. k8s Deployment・Service・ConfigMap・Secret を削除する
	if err := workflow.ExecuteActivity(ctx, (*activity.DeploymentActivities).DeleteK8sDeploymentActivity, activity.DeleteK8sDeploymentActivityInput{DeploymentID: input.DeploymentID}).Get(ctx, nil); err != nil { // k8s 削除 Activity を実行する
		return err // エラーを返す
	}

	// 3. DB レコードを削除する
	if err := workflow.ExecuteActivity(ctx, (*activity.DeploymentActivities).DeleteDeploymentRecordActivity, deleteInput).Get(ctx, nil); err != nil { // DB 削除 Activity を実行する
		return err // エラーを返す
	}

	return nil // 正常終了を返す
}
