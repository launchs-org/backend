package activity

import (
	"app/shared/models"
	"app/shared/repository"
	"context"
	"errors"
	"fmt"

	"controller/k8s"

	"gorm.io/gorm"
	k8sclient "k8s.io/client-go/kubernetes"
)

// DeploymentActivities は Deployment 削除 Workflow で使われる Activity 群を保持する構造体
type DeploymentActivities struct {
	k8sClient       k8sclient.Interface               // k8s クライアント
	deploymentRepo  repository.DeploymentRepository   // deployment リポジトリ
	projectRepo     repository.ProjectRepository      // project リポジトリ
	volumeMountRepo repository.VolumeMountRepository  // volume_mount リポジトリ
	envVarRepo      repository.EnvVarRepository       // env_var リポジトリ（削除時の後始末用）
	envVarMountRepo repository.EnvVarMountRepository  // env_var_mount リポジトリ（削除時の後始末用）
}

// NewDeploymentActivities は DeploymentActivities を生成して返す
func NewDeploymentActivities(
	k8sClient k8sclient.Interface,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
	volumeMountRepo repository.VolumeMountRepository,
	envVarRepo repository.EnvVarRepository,
	envVarMountRepo repository.EnvVarMountRepository,
) *DeploymentActivities {
	return &DeploymentActivities{ // 依存を注入して返す
		k8sClient:       k8sClient,
		deploymentRepo:  deploymentRepo,
		projectRepo:     projectRepo,
		volumeMountRepo: volumeMountRepo,
		envVarRepo:      envVarRepo,
		envVarMountRepo: envVarMountRepo,
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

	// env_var_mount と、他のデプロイメントから参照されなくなった env_var 本体を削除する
	if err := activities.deleteEnvVarsByDeploymentID(ctx, input.DeploymentID); err != nil {
		return fmt.Errorf("delete env_vars: %w", err) // 削除エラーを返す
	}

	// deployment レコードを削除する
	if err := activities.deploymentRepo.Delete(ctx, input.DeploymentID); err != nil { // deployment を削除する
		return fmt.Errorf("delete deployment: %w", err) // 削除エラーを返す
	}

	return nil // 正常終了を返す
}

// deleteEnvVarsByDeploymentID は deploymentID に紐づく env_var_mount を全件削除し、
// 他のデプロイメントから参照されなくなった env_var 本体も削除する
func (activities *DeploymentActivities) deleteEnvVarsByDeploymentID(ctx context.Context, deploymentID string) error {
	envVarMountList, err := activities.envVarMountRepo.FindAllByDeploymentID(ctx, deploymentID) // 紐づくマウント設定一覧を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if err := activities.envVarMountRepo.DeleteAllByDeploymentID(ctx, nil, deploymentID); err != nil { // マウント設定を一括削除する
		return err // 削除エラーを返す
	}
	for _, mountItem := range envVarMountList { // 各マウント設定が参照していた env_var を確認する
		remainingCount, err := activities.envVarMountRepo.CountByEnvVarID(ctx, mountItem.EnvVarID) // 残存参照数を確認する
		if err != nil {
			return err // 確認エラーを返す
		}
		if remainingCount > 0 { // 他のデプロイメントからまだ参照されている場合は残す
			continue
		}
		envVarData, err := activities.envVarRepo.FindByID(ctx, mountItem.EnvVarID) // env_var 本体を取得する
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) { // 既に削除済みなら何もしない
				continue
			}
			return err // 取得エラーを返す
		}
		if err := activities.envVarRepo.Delete(ctx, nil, envVarData); err != nil { // env_var 本体を削除する
			return err // 削除エラーを返す
		}
	}
	return nil // 全て成功
}
