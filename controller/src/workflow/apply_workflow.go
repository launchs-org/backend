package workflow

import (
	"controller/activity"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ApplyWorkflowInput は ApplyWorkflow への入力
type ApplyWorkflowInput struct {
	DeploymentID string // 対象デプロイメントのID
	BaseDomain   string // ベースドメイン
	ProjectID    string // 対象プロジェクトのID（IngressRoute apply に使用）
}

// ApplyWorkflow は Deployment の Apply 処理を Temporal Workflow として実装する
// WorkflowID は "apply-{deploymentID}" で冪等性を保証する
func ApplyWorkflow(ctx workflow.Context, input ApplyWorkflowInput) error {
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute, // Activity 実行タイムアウト
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3, // 最大リトライ回数
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions) // Activity オプションをコンテキストに設定する

	// pending→current 昇格・Manifest生成・k8s Apply・ApplyHistory記録を実行する
	var applyResult *activity.ApplyResultData                            // 結果を格納する変数を定義する
	if err := workflow.ExecuteActivity(ctx, (*activity.ApplyActivities).ExecuteApply, activity.ApplyActivityInput{
		DeploymentID: input.DeploymentID,
		BaseDomain:   input.BaseDomain,
	}).Get(ctx, &applyResult); err != nil { // Apply Activity を実行する
		return err // エラーを返す
	}

	// IngressRoute を apply する
	if input.ProjectID != "" { // ProjectID が設定されている場合のみ IngressRoute を apply する
		if err := workflow.ExecuteActivity(ctx, (*activity.ApplyActivities).ApplyIngressRoutes, activity.ApplyIngressRoutesInput{
			ProjectID:  input.ProjectID,
			BaseDomain: input.BaseDomain,
		}).Get(ctx, nil); err != nil { // IngressRoute apply Activity を実行する
			return err // エラーを返す
		}
	}

	return nil // 正常終了を返す
}
