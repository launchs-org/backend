package activity

import (
	"app/shared/models"
	"app/shared/repository"
	"context"
	"fmt"

	"controller/k8s"

	k8sclient "k8s.io/client-go/kubernetes"
)

// ProjectActivities は Project 系 Workflow で使われる Activity 群を保持する構造体
type ProjectActivities struct {
	k8sClient            k8sclient.Interface                       // k8s クライアント
	projectRepo          repository.ProjectRepository              // project リポジトリ
	harborCredentialRepo repository.HarborCredentialRepository    // harbor credential リポジトリ
	deploymentRepo       repository.DeploymentRepository          // deployment リポジトリ
	harborClient         *k8s.HarborClient                        // Harbor API クライアント
	harborStorageLimit   int64                                     // Harbor ストレージ制限（バイト）
}

// NewProjectActivities は ProjectActivities を生成して返す
func NewProjectActivities(
	k8sClient k8sclient.Interface,
	projectRepo repository.ProjectRepository,
	harborCredentialRepo repository.HarborCredentialRepository,
	deploymentRepo repository.DeploymentRepository,
	harborClient *k8s.HarborClient,
	harborStorageLimit int64,
) *ProjectActivities {
	return &ProjectActivities{ // 依存を注入して返す
		k8sClient:            k8sClient,
		projectRepo:          projectRepo,
		harborCredentialRepo: harborCredentialRepo,
		deploymentRepo:       deploymentRepo,
		harborClient:         harborClient,
		harborStorageLimit:   harborStorageLimit,
	}
}

// CreateProjectWorkflowInput は CreateProject Workflow への入力
type CreateProjectWorkflowInput struct {
	ProjectID string // 対象プロジェクトのID（あらかじめ DB に provisioning 状態で作成済み）
}

// CreateHarborProjectActivity は Harbor プロジェクトを作成する Activity
func (activities *ProjectActivities) CreateHarborProjectActivity(ctx context.Context, input CreateProjectWorkflowInput) error {
	if err := activities.harborClient.CreateHarborProject(ctx, input.ProjectID, activities.harborStorageLimit); err != nil { // Harbor プロジェクトを作成する
		return fmt.Errorf("harbor project の作成に失敗しました: %w", err) // 作成エラーを返す
	}
	return nil // 正常終了を返す
}

// CreateHarborRobotActivity は Harbor Robot アカウントを作成して DB に保存する Activity
func (activities *ProjectActivities) CreateHarborRobotActivity(ctx context.Context, input CreateProjectWorkflowInput) error {
	robotCredential, err := activities.harborClient.CreateHarborRobotAccount(ctx, input.ProjectID) // Harbor Robot アカウントを作成する
	if err != nil {
		return fmt.Errorf("harbor robot account の作成に失敗しました: %w", err) // 作成エラーを返す
	}

	credentialData := &models.HarborCredential{ // HarborCredential レコードを構築する
		ProjectID:      input.ProjectID,                          // プロジェクト ID を設定する
		RobotName:      robotCredential.Name,                     // robot アカウント名を設定する
		RobotSecret:    robotCredential.Secret,                   // シークレットを設定する
		HarborEndpoint: activities.harborClient.Endpoint(),       // エンドポイントを設定する
	}
	if err := activities.harborCredentialRepo.CreateNoTx(ctx, credentialData); err != nil { // DB に保存する（トランザクション外）
		return fmt.Errorf("harbor credential レコードの保存に失敗しました: %w", err) // 保存エラーを返す
	}
	return nil // 正常終了を返す
}

// CreateK8sNamespaceActivity は k8s Namespace を作成する Activity
func (activities *ProjectActivities) CreateK8sNamespaceActivity(ctx context.Context, input CreateProjectWorkflowInput) error {
	projectData, err := activities.projectRepo.FindByIDNoTx(ctx, input.ProjectID) // project を取得する
	if err != nil {
		return fmt.Errorf("project not found: %w", err) // 取得エラーを返す
	}

	if err := k8s.CreateNamespace(ctx, activities.k8sClient, projectData.Namespace); err != nil { // k8s Namespace を作成する
		return fmt.Errorf("k8s namespace の作成に失敗しました: %w", err) // 作成エラーを返す
	}
	return nil // 正常終了を返す
}

// ActivateProjectActivity は DB の project status を active に更新する Activity
func (activities *ProjectActivities) ActivateProjectActivity(ctx context.Context, input CreateProjectWorkflowInput) error {
	projectData, err := activities.projectRepo.FindByIDNoTx(ctx, input.ProjectID) // project を取得する
	if err != nil {
		return fmt.Errorf("project not found: %w", err) // 取得エラーを返す
	}

	if err := activities.projectRepo.UpdateStatusNoTx(ctx, projectData, models.ProjectStatusActive); err != nil { // status を active に更新する（トランザクション外）
		return fmt.Errorf("project status update: %w", err) // 更新エラーを返す
	}
	return nil // 正常終了を返す
}

// DeleteProjectInput は DeleteProject Workflow への入力
type DeleteProjectInput struct {
	ProjectID string // 削除対象プロジェクトのID
}

// DeleteK8sNamespaceActivity は k8s Namespace を削除する Activity
func (activities *ProjectActivities) DeleteK8sNamespaceActivity(ctx context.Context, input DeleteProjectInput) error {
	projectData, err := activities.projectRepo.FindByIDNoTx(ctx, input.ProjectID) // project を取得する
	if err != nil {
		return fmt.Errorf("project not found: %w", err) // 取得エラーを返す
	}

	if err := k8s.DeleteNamespace(ctx, activities.k8sClient, projectData.Namespace); err != nil { // k8s Namespace を削除する
		return fmt.Errorf("k8s namespace の削除に失敗しました: %w", err) // 削除エラーを返す
	}
	return nil // 正常終了を返す
}

// DeleteHarborProjectActivity は Harbor プロジェクトを削除する Activity
func (activities *ProjectActivities) DeleteHarborProjectActivity(ctx context.Context, input DeleteProjectInput) error {
	credentialData, err := activities.harborCredentialRepo.FindByProjectIDNoTx(ctx, input.ProjectID) // Harbor 認証情報を取得する
	if err != nil {
		return fmt.Errorf("harbor credential not found: %w", err) // 取得エラーを返す
	}

	robotCredential := k8s.HarborRobotCredential{ // robot 認証情報を構築する
		Name:   credentialData.RobotName,   // robot アカウント名を設定する
		Secret: credentialData.RobotSecret, // シークレットを設定する
	}
	if err := activities.harborClient.DeleteHarborProject(ctx, input.ProjectID, robotCredential); err != nil { // Harbor プロジェクトを削除する
		return fmt.Errorf("harbor project の削除に失敗しました: %w", err) // 削除エラーを返す
	}
	return nil // 正常終了を返す
}

// DeleteProjectRecordActivity は DB から project レコードを削除する Activity
func (activities *ProjectActivities) DeleteProjectRecordActivity(ctx context.Context, input DeleteProjectInput) error {
	projectData, err := activities.projectRepo.FindByIDNoTx(ctx, input.ProjectID) // project を取得する
	if err != nil {
		return fmt.Errorf("project not found: %w", err) // 取得エラーを返す
	}

	if err := activities.projectRepo.DeleteNoTx(ctx, projectData); err != nil { // project レコードを削除する
		return fmt.Errorf("project record delete: %w", err) // 削除エラーを返す
	}
	return nil // 正常終了を返す
}
