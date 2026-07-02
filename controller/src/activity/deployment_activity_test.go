package activity

import (
	"app/shared/models"
	"app/shared/repository"
	"context"
	"errors"
	"testing"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// mockDeploymentRepository は DeploymentRepository のテスト用モック
type mockDeploymentRepository struct {
	findByIDFunc             func(ctx context.Context, deploymentID string) (*models.Deployment, error)
	findByIDForUpdateFunc    func(ctx context.Context, tx *gorm.DB, deploymentID string) (*models.Deployment, error)
	findAllByProjectIDFunc   func(ctx context.Context, projectID string) ([]models.Deployment, error)
	findAllRunningFunc       func(ctx context.Context) ([]models.Deployment, error)
	saveFunc                 func(ctx context.Context, deployment *models.Deployment) error
	updatesFunc              func(ctx context.Context, tx *gorm.DB, deployment *models.Deployment, values map[string]interface{}) error
	updateAppStatusFunc      func(ctx context.Context, deploymentID string, appStatus models.AppStatus) error
	updateK8sStatusFunc      func(ctx context.Context, deploymentID string, k8sStatus datatypes.JSON) error
	updateDeleteProgressFunc func(ctx context.Context, deploymentID string, progress string) error
	updatePendingImageURLFunc func(ctx context.Context, deploymentID string, imageURL string) error
	updatePendingGithubCommitSHAFunc func(ctx context.Context, deploymentID string, commitSHA string) error
	updatePendingGithubBuildFieldsFunc func(ctx context.Context, deploymentID string, repoURL string, branch string, commitSHA string, directory string) error
	updateDeploymentStatusFunc func(ctx context.Context, deploymentID string, status models.DeploymentStatus) error
	updateCurrentBuildIDFunc   func(ctx context.Context, deploymentID string, buildID string) error
	clearCurrentBuildIDFunc    func(ctx context.Context, deploymentID string) error
	deleteFunc                 func(ctx context.Context, deploymentID string) error
	createFunc                 func(ctx context.Context, deployment *models.Deployment) error
	createWithTxFunc           func(ctx context.Context, tx *gorm.DB, deployment *models.Deployment) error
}

func (mock *mockDeploymentRepository) Create(ctx context.Context, deployment *models.Deployment) error {
	if mock.createFunc != nil {
		return mock.createFunc(ctx, deployment)
	}
	return nil
}
func (mock *mockDeploymentRepository) CreateWithTx(ctx context.Context, tx *gorm.DB, deployment *models.Deployment) error {
	if mock.createWithTxFunc != nil {
		return mock.createWithTxFunc(ctx, tx, deployment)
	}
	return nil
}
func (mock *mockDeploymentRepository) FindByID(ctx context.Context, deploymentID string) (*models.Deployment, error) {
	if mock.findByIDFunc != nil {
		return mock.findByIDFunc(ctx, deploymentID)
	}
	return nil, nil
}
func (mock *mockDeploymentRepository) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, deploymentID string) (*models.Deployment, error) {
	if mock.findByIDForUpdateFunc != nil {
		return mock.findByIDForUpdateFunc(ctx, tx, deploymentID)
	}
	return nil, nil
}
func (mock *mockDeploymentRepository) FindAllByProjectID(ctx context.Context, projectID string) ([]models.Deployment, error) {
	if mock.findAllByProjectIDFunc != nil {
		return mock.findAllByProjectIDFunc(ctx, projectID)
	}
	return nil, nil
}
func (mock *mockDeploymentRepository) FindAllRunning(ctx context.Context) ([]models.Deployment, error) {
	if mock.findAllRunningFunc != nil {
		return mock.findAllRunningFunc(ctx)
	}
	return nil, nil
}
func (mock *mockDeploymentRepository) Save(ctx context.Context, deployment *models.Deployment) error {
	if mock.saveFunc != nil {
		return mock.saveFunc(ctx, deployment)
	}
	return nil
}
func (mock *mockDeploymentRepository) Updates(ctx context.Context, tx *gorm.DB, deployment *models.Deployment, values map[string]interface{}) error {
	if mock.updatesFunc != nil {
		return mock.updatesFunc(ctx, tx, deployment, values)
	}
	return nil
}
func (mock *mockDeploymentRepository) UpdateAppStatus(ctx context.Context, deploymentID string, appStatus models.AppStatus) error {
	if mock.updateAppStatusFunc != nil {
		return mock.updateAppStatusFunc(ctx, deploymentID, appStatus)
	}
	return nil
}
func (mock *mockDeploymentRepository) UpdateK8sStatus(ctx context.Context, deploymentID string, k8sStatus datatypes.JSON) error {
	if mock.updateK8sStatusFunc != nil {
		return mock.updateK8sStatusFunc(ctx, deploymentID, k8sStatus)
	}
	return nil
}
func (mock *mockDeploymentRepository) UpdateDeleteProgress(ctx context.Context, deploymentID string, progress string) error {
	if mock.updateDeleteProgressFunc != nil {
		return mock.updateDeleteProgressFunc(ctx, deploymentID, progress)
	}
	return nil
}
func (mock *mockDeploymentRepository) UpdatePendingImageURL(ctx context.Context, deploymentID string, imageURL string) error {
	if mock.updatePendingImageURLFunc != nil {
		return mock.updatePendingImageURLFunc(ctx, deploymentID, imageURL)
	}
	return nil
}
func (mock *mockDeploymentRepository) UpdatePendingGithubCommitSHA(ctx context.Context, deploymentID string, commitSHA string) error {
	if mock.updatePendingGithubCommitSHAFunc != nil {
		return mock.updatePendingGithubCommitSHAFunc(ctx, deploymentID, commitSHA)
	}
	return nil
}
func (mock *mockDeploymentRepository) UpdatePendingGithubBuildFields(ctx context.Context, deploymentID string, repoURL string, branch string, commitSHA string, directory string) error {
	if mock.updatePendingGithubBuildFieldsFunc != nil {
		return mock.updatePendingGithubBuildFieldsFunc(ctx, deploymentID, repoURL, branch, commitSHA, directory)
	}
	return nil
}
func (mock *mockDeploymentRepository) UpdateDeploymentStatus(ctx context.Context, deploymentID string, status models.DeploymentStatus) error {
	if mock.updateDeploymentStatusFunc != nil {
		return mock.updateDeploymentStatusFunc(ctx, deploymentID, status)
	}
	return nil
}
func (mock *mockDeploymentRepository) UpdateCurrentBuildID(ctx context.Context, deploymentID string, buildID string) error {
	if mock.updateCurrentBuildIDFunc != nil {
		return mock.updateCurrentBuildIDFunc(ctx, deploymentID, buildID)
	}
	return nil
}
func (mock *mockDeploymentRepository) ClearCurrentBuildID(ctx context.Context, deploymentID string) error {
	if mock.clearCurrentBuildIDFunc != nil {
		return mock.clearCurrentBuildIDFunc(ctx, deploymentID)
	}
	return nil
}
func (mock *mockDeploymentRepository) Delete(ctx context.Context, deploymentID string) error {
	if mock.deleteFunc != nil {
		return mock.deleteFunc(ctx, deploymentID)
	}
	return nil
}

// mockProjectRepository は ProjectRepository のテスト用モック（controller activity 用）
type mockProjectRepository struct {
	findByIDNoTxFunc    func(ctx context.Context, projectID string) (*models.Project, error)
	findByIDFunc        func(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error)
	findByNamespaceFunc func(ctx context.Context, namespace string) (*models.Project, error)
	findAllByUserIDFunc func(ctx context.Context, userID string) ([]*models.Project, error)
	updateStatusFunc    func(ctx context.Context, tx *gorm.DB, project *models.Project, status models.ProjectStatus) error
	updateStatusNoTxFunc func(ctx context.Context, project *models.Project, status models.ProjectStatus) error
	saveFunc            func(ctx context.Context, project *models.Project) error
	createFunc          func(ctx context.Context, tx *gorm.DB, project *models.Project) error
	deleteFunc          func(ctx context.Context, tx *gorm.DB, project *models.Project) error
	deleteNoTxFunc      func(ctx context.Context, project *models.Project) error
}

func (mock *mockProjectRepository) FindByIDNoTx(ctx context.Context, projectID string) (*models.Project, error) {
	if mock.findByIDNoTxFunc != nil {
		return mock.findByIDNoTxFunc(ctx, projectID)
	}
	return nil, nil
}
func (mock *mockProjectRepository) FindByID(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error) {
	if mock.findByIDFunc != nil {
		return mock.findByIDFunc(ctx, tx, projectID)
	}
	return nil, nil
}
func (mock *mockProjectRepository) FindByNamespace(ctx context.Context, namespace string) (*models.Project, error) {
	if mock.findByNamespaceFunc != nil {
		return mock.findByNamespaceFunc(ctx, namespace)
	}
	return nil, nil
}
func (mock *mockProjectRepository) FindAllByUserID(ctx context.Context, userID string) ([]*models.Project, error) {
	if mock.findAllByUserIDFunc != nil {
		return mock.findAllByUserIDFunc(ctx, userID)
	}
	return nil, nil
}
func (mock *mockProjectRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, project *models.Project, status models.ProjectStatus) error {
	if mock.updateStatusFunc != nil {
		return mock.updateStatusFunc(ctx, tx, project, status)
	}
	return nil
}
func (mock *mockProjectRepository) UpdateStatusNoTx(ctx context.Context, project *models.Project, status models.ProjectStatus) error {
	if mock.updateStatusNoTxFunc != nil {
		return mock.updateStatusNoTxFunc(ctx, project, status)
	}
	return nil
}
func (mock *mockProjectRepository) Save(ctx context.Context, project *models.Project) error {
	if mock.saveFunc != nil {
		return mock.saveFunc(ctx, project)
	}
	return nil
}
func (mock *mockProjectRepository) Create(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	if mock.createFunc != nil {
		return mock.createFunc(ctx, tx, project)
	}
	return nil
}
func (mock *mockProjectRepository) Delete(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	if mock.deleteFunc != nil {
		return mock.deleteFunc(ctx, tx, project)
	}
	return nil
}
func (mock *mockProjectRepository) DeleteNoTx(ctx context.Context, project *models.Project) error {
	if mock.deleteNoTxFunc != nil {
		return mock.deleteNoTxFunc(ctx, project)
	}
	return nil
}

// mockVolumeMountRepository は VolumeMountRepository のテスト用モック（controller activity 用）
type mockVolumeMountRepository struct {
	deleteAllByDeploymentIDFunc func(ctx context.Context, tx *gorm.DB, deploymentID string) error
}

func (mock *mockVolumeMountRepository) Create(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error {
	return nil
}
func (mock *mockVolumeMountRepository) FindByID(ctx context.Context, mountID string) (*models.VolumeMount, error) {
	return nil, nil
}
func (mock *mockVolumeMountRepository) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.VolumeMount, error) {
	return nil, nil
}
func (mock *mockVolumeMountRepository) FindAllByVolumeID(ctx context.Context, volumeID string) ([]*models.VolumeMount, error) {
	return nil, nil
}
func (mock *mockVolumeMountRepository) FindByDeploymentIDAndMountPath(ctx context.Context, deploymentID string, mountPath string) (*models.VolumeMount, error) {
	return nil, nil
}
func (mock *mockVolumeMountRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount, status models.VolumeMountStatus) error {
	return nil
}
func (mock *mockVolumeMountRepository) Delete(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error {
	return nil
}
func (mock *mockVolumeMountRepository) DeleteAllByDeploymentID(ctx context.Context, tx *gorm.DB, deploymentID string) error {
	if mock.deleteAllByDeploymentIDFunc != nil {
		return mock.deleteAllByDeploymentIDFunc(ctx, tx, deploymentID)
	}
	return nil
}

// インターフェース実装を静的に確認する
var _ repository.DeploymentRepository = (*mockDeploymentRepository)(nil)
var _ repository.ProjectRepository = (*mockProjectRepository)(nil)
var _ repository.VolumeMountRepository = (*mockVolumeMountRepository)(nil)

// TestSetDeploymentDeletingActivity_正常にステータスがdeletingになる は deployment ステータスが deleting に変更されることを確認する
func TestSetDeploymentDeletingActivity_正常にステータスがdeletingになる(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	var savedDeployment *models.Deployment // 保存された deployment を記録する変数を定義する

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{
				ID:     deploymentID,           // テスト用 deployment を返す
				Status: models.DeploymentStatusPending, // 初期ステータスを設定する
			}, nil
		},
		saveFunc: func(ctx context.Context, deployment *models.Deployment) error {
			savedDeployment = deployment // 保存された deployment を記録する
			return nil                  // 保存成功を返す
		},
	}

	activities := &DeploymentActivities{
		k8sClient:       k8sfake.NewSimpleClientset(), // fake k8s クライアントを生成する
		deploymentRepo:  deploymentRepo,               // モックリポジトリを注入する
		projectRepo:     &mockProjectRepository{},     // 空のモックを注入する
		volumeMountRepo: &mockVolumeMountRepository{}, // 空のモックを注入する
	}

	err := activities.SetDeploymentDeletingActivity(ctx, DeleteDeploymentInput{DeploymentID: "deployment-1"}) // Activity を実行する
	if err != nil {
		t.Fatalf("SetDeploymentDeletingActivity がエラーを返しました: %v", err) // エラーが発生した場合はテスト失敗
	}
	if savedDeployment == nil { // deployment が保存されたことを確認する
		t.Fatal("deployment が保存されていません")
	}
	if savedDeployment.Status != models.DeploymentStatusDeleting { // ステータスが deleting になったことを確認する
		t.Errorf("期待するステータス %s、実際のステータス %s", models.DeploymentStatusDeleting, savedDeployment.Status)
	}
}

// TestSetDeploymentDeletingActivity_deploymentが見つからない場合はエラーを返す は deployment が見つからない場合エラーを返すことを確認する
func TestSetDeploymentDeletingActivity_deploymentが見つからない場合はエラーを返す(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return nil, errors.New("not found") // 取得エラーを返す
		},
	}

	activities := &DeploymentActivities{
		k8sClient:       k8sfake.NewSimpleClientset(), // fake k8s クライアントを生成する
		deploymentRepo:  deploymentRepo,               // モックリポジトリを注入する
		projectRepo:     &mockProjectRepository{},     // 空のモックを注入する
		volumeMountRepo: &mockVolumeMountRepository{}, // 空のモックを注入する
	}

	err := activities.SetDeploymentDeletingActivity(ctx, DeleteDeploymentInput{DeploymentID: "not-exists"}) // Activity を実行する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("エラーが返されるべきですが、nil が返りました")
	}
}

// TestDeleteDeploymentRecordActivity_正常にDBレコードが削除される は deployment レコードが正常に削除されることを確認する
func TestDeleteDeploymentRecordActivity_正常にDBレコードが削除される(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	clearCalled := false  // clearCurrentBuildID が呼ばれたかを記録する
	deleteCalled := false // Delete が呼ばれたかを記録する

	deploymentRepo := &mockDeploymentRepository{
		clearCurrentBuildIDFunc: func(ctx context.Context, deploymentID string) error {
			clearCalled = true // 呼び出しを記録する
			return nil         // 成功を返す
		},
		deleteFunc: func(ctx context.Context, deploymentID string) error {
			deleteCalled = true // 呼び出しを記録する
			return nil          // 成功を返す
		},
	}

	activities := &DeploymentActivities{
		k8sClient:       k8sfake.NewSimpleClientset(), // fake k8s クライアントを生成する
		deploymentRepo:  deploymentRepo,               // モックリポジトリを注入する
		projectRepo:     &mockProjectRepository{},     // 空のモックを注入する
		volumeMountRepo: &mockVolumeMountRepository{}, // 空のモックを注入する
	}

	err := activities.DeleteDeploymentRecordActivity(ctx, DeleteDeploymentInput{DeploymentID: "deployment-1"}) // Activity を実行する
	if err != nil {
		t.Fatalf("DeleteDeploymentRecordActivity がエラーを返しました: %v", err) // エラーが発生した場合はテスト失敗
	}
	if !clearCalled { // ClearCurrentBuildID が呼ばれていない場合はテスト失敗
		t.Error("ClearCurrentBuildID が呼ばれていません")
	}
	if !deleteCalled { // Delete が呼ばれていない場合はテスト失敗
		t.Error("Delete が呼ばれていません")
	}
}
