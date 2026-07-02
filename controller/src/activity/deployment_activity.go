package activity

import (
	"app/shared/models"
	"app/shared/repository"
	"context"
	"fmt"

	"controller/k8s"

	k8sclient "k8s.io/client-go/kubernetes"
)

// DeploymentActivities は Deployment 削除 Workflow で使われる Activity 群を保持する構造体
type DeploymentActivities struct {
	k8sClient      k8sclient.Interface                // k8s クライアント
	deploymentRepo repository.DeploymentRepository    // deployment リポジトリ
	projectRepo    repository.ProjectRepository       // project リポジトリ
	volumeMountRepo repository.VolumeMountRepository  // volume_mount リポジトリ
}

// NewDeploymentActivities は DeploymentActivities を生成して返す
func NewDeploymentActivities(
	k8sClient k8sclient.Interface,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
	volumeMountRepo repository.VolumeMountRepository,
) *DeploymentActivities {
	return &DeploymentActivities{ // 依存を注入して返す
		k8sClient:       k8sClient,
		deploymentRepo:  deploymentRepo,
		projectRepo:     projectRepo,
		volumeMountRepo: volumeMountRepo,
	}
}

// DeleteDeploymentInput は DeleteDeployment Activity への入力
type DeleteDeploymentInput struct {
	DeploymentID string // 削除対象デプロイメントのID
}

// SetDeploymentDeletingActivity は DB の deployment ステータスを deleting に更新する Activity
func (activities *DeploymentActivities) SetDeploymentDeletingActivity(ctx context.Context, input DeleteDeploymentInput) error {
	deploymentData, err := activities.deploymentRepo.FindByID(ctx, input.DeploymentID) // deployment を取得する
	if err != nil {
		return fmt.Errorf("deployment not found: %w", err) // 取得エラーを返す
	}

	deploymentData.Status = models.DeploymentStatusDeleting       // ステータスを deleting に変更する
	deploymentData.DeleteProgress = "k8s リソースを削除中"          // 初期進捗を設定する
	if err := activities.deploymentRepo.Save(ctx, deploymentData); err != nil { // リポジトリ経由で保存する
		return fmt.Errorf("deployment status update: %w", err) // 保存エラーを返す
	}
	return nil // 正常終了を返す
}

// DeleteK8sDeploymentActivityInput は k8s Deployment 削除 Activity への入力
type DeleteK8sDeploymentActivityInput struct {
	DeploymentID string // 削除対象デプロイメントのID
}

// DeleteK8sDeploymentActivity は k8s から Deployment・Service・ConfigMap・Secret を削除する Activity
func (activities *DeploymentActivities) DeleteK8sDeploymentActivity(ctx context.Context, input DeleteK8sDeploymentActivityInput) error {
	deploymentData, err := activities.deploymentRepo.FindByID(ctx, input.DeploymentID) // deployment を取得する
	if err != nil {
		return fmt.Errorf("deployment not found: %w", err) // 取得エラーを返す
	}

	projectData, err := activities.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
	if err != nil {
		return fmt.Errorf("project not found: %w", err) // 取得エラーを返す
	}
	namespace := projectData.Namespace // namespace を取得する

	// k8s リソースを順に削除する（エラーは無視して継続する）
	_ = activities.deploymentRepo.UpdateDeleteProgress(ctx, deploymentData.ID, "k8s Deployment を削除中") // 進捗を記録する
	_ = k8s.DeleteDeployment(ctx, activities.k8sClient, namespace, deploymentData.Name)               // k8s Deployment を削除する

	_ = activities.deploymentRepo.UpdateDeleteProgress(ctx, deploymentData.ID, "k8s Service を削除中") // 進捗を記録する
	_ = k8s.DeleteService(ctx, activities.k8sClient, namespace, deploymentData.Name)                // k8s Service を削除する

	_ = activities.deploymentRepo.UpdateDeleteProgress(ctx, deploymentData.ID, "k8s ConfigMap を削除中") // 進捗を記録する
	_ = k8s.DeleteConfigMap(ctx, activities.k8sClient, namespace, deploymentData.Name)               // k8s ConfigMap を削除する

	_ = activities.deploymentRepo.UpdateDeleteProgress(ctx, deploymentData.ID, "k8s Secret を削除中") // 進捗を記録する
	_ = k8s.DeleteSecret(ctx, activities.k8sClient, namespace, deploymentData.Name)                // k8s Secret を削除する

	_ = activities.deploymentRepo.UpdateDeleteProgress(ctx, deploymentData.ID, "k8s リソース削除完了 / DB クリーンアップ待ち") // 進捗を記録する

	return nil // 正常終了を返す
}

// DeleteDeploymentRecordActivity は DB から deployment レコードを削除する Activity
func (activities *DeploymentActivities) DeleteDeploymentRecordActivity(ctx context.Context, input DeleteDeploymentInput) error {
	// current_build_id を NULL にして外部キー制約を解除する
	if err := activities.deploymentRepo.ClearCurrentBuildID(ctx, input.DeploymentID); err != nil { // current_build_id をクリアする
		return fmt.Errorf("clear current_build_id: %w", err) // エラーを返す
	}

	// volume_mount レコードを削除する
	if err := activities.volumeMountRepo.DeleteAllByDeploymentID(ctx, nil, input.DeploymentID); err != nil { // volume_mount を削除する
		return fmt.Errorf("delete volume_mounts: %w", err) // 削除エラーを返す
	}

	// deployment レコードを削除する
	if err := activities.deploymentRepo.Delete(ctx, input.DeploymentID); err != nil { // deployment を削除する
		return fmt.Errorf("delete deployment: %w", err) // 削除エラーを返す
	}

	return nil // 正常終了を返す
}
