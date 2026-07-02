package workflow

import (
	"controller/activity"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CreateProjectWorkflowInput は CreateProjectWorkflow への入力
type CreateProjectWorkflowInput struct {
	ProjectID string // 対象プロジェクトのID（あらかじめ DB に provisioning 状態で作成済み）
}

// CreateProjectWorkflow は Project の作成処理を Temporal Workflow として実装する
// WorkflowID は "create-project-{projectID}" で冪等性を保証する
func CreateProjectWorkflow(ctx workflow.Context, input CreateProjectWorkflowInput) error {
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute, // Activity 実行タイムアウト
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3, // 最大リトライ回数
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions) // Activity オプションをコンテキストに設定する

	activityInput := activity.CreateProjectWorkflowInput{ProjectID: input.ProjectID} // Activity 入力を構築する

	// 1. Harbor プロジェクトを作成する
	if err := workflow.ExecuteActivity(ctx, (*activity.ProjectActivities).CreateHarborProjectActivity, activityInput).Get(ctx, nil); err != nil { // Harbor プロジェクト作成 Activity を実行する
		return err // エラーを返す
	}

	// 2. Harbor Robot アカウントを作成する
	if err := workflow.ExecuteActivity(ctx, (*activity.ProjectActivities).CreateHarborRobotActivity, activityInput).Get(ctx, nil); err != nil { // Harbor Robot 作成 Activity を実行する
		return err // エラーを返す
	}

	// 3. k8s Namespace を作成する
	if err := workflow.ExecuteActivity(ctx, (*activity.ProjectActivities).CreateK8sNamespaceActivity, activityInput).Get(ctx, nil); err != nil { // Namespace 作成 Activity を実行する
		return err // エラーを返す
	}

	// 4. DB の project status を active に更新する
	if err := workflow.ExecuteActivity(ctx, (*activity.ProjectActivities).ActivateProjectActivity, activityInput).Get(ctx, nil); err != nil { // ステータス更新 Activity を実行する
		return err // エラーを返す
	}

	return nil // 正常終了を返す
}
