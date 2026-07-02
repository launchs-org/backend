package activity

import (
	"context"
	"fmt"

	"app/shared/models"
	"app/shared/repository"
	"builder/railpack"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CancelBuildActivities はビルドキャンセル関連の Temporal Activity をまとめた構造体
type CancelBuildActivities struct {
	K8sClient  kubernetes.Interface                  // k8s クライアント
	BuildRepo  repository.DeploymentBuildRepository // build リポジトリ
}

// CancelBuildWorkflowInput は CancelBuildWorkflow への入力
type CancelBuildWorkflowInput struct {
	BuildID string // キャンセル対象ビルドの ID
}

// DeleteBuildJobActivity は K8s Job を削除する（Job 名が設定されている場合のみ）
func (act *CancelBuildActivities) DeleteBuildJobActivity(ctx context.Context, input CancelBuildWorkflowInput) error {
	buildData, err := act.BuildRepo.FindByID(ctx, input.BuildID) // ビルドレコードを取得する
	if err != nil {
		return fmt.Errorf("ビルドレコードの取得に失敗しました: %w", err) // 取得エラーを返す
	}

	if buildData.K8sJobName == "" { // Job 名が空の場合は pending 状態で Job 未作成なのでスキップする
		return nil // スキップして正常終了を返す
	}

	// railpack の Job 名から jobID を抽出する（"railpack-{buildID}" の形式）
	jobID := buildData.ID                                                                                        // ビルド ID を Job ID として使用する
	if err := railpack.DeleteJob(ctx, act.K8sClient, buildkitNamespace, jobID); err != nil { // K8s Job を削除する
		// Job が既に存在しない場合（404）はエラーを無視する
		if isNotFoundError(ctx, act.K8sClient, buildkitNamespace, buildData.K8sJobName) {
			return nil // 既に存在しない場合はスキップする
		}
		return fmt.Errorf("K8s Job の削除に失敗しました: %w", err) // 削除エラーを返す
	}

	return nil // 削除成功を返す
}

// SetBuildCancelledActivity はビルドステータスを cancelled に更新する
func (act *CancelBuildActivities) SetBuildCancelledActivity(ctx context.Context, input CancelBuildWorkflowInput) error {
	if err := act.BuildRepo.UpdateStatus(ctx, input.BuildID, models.BuildStatusCancelled); err != nil { // ステータスを cancelled に更新する
		return fmt.Errorf("ビルドステータスの更新に失敗しました: %w", err) // 更新エラーを返す
	}
	return nil // 更新成功を返す
}

// isNotFoundError は指定した Job が存在しない場合に true を返す
func isNotFoundError(ctx context.Context, k8sClient kubernetes.Interface, namespace, jobName string) bool {
	_, err := k8sClient.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{}) // Job の存在確認をする
	return err != nil                                                                     // エラーが発生した場合は存在しないと判断する
}
