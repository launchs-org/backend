package k8s

import (
	"app/shared/models"
	"app/shared/repository"
	"context"
	"sync"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ---- モック定義 ----

// mockDeploymentRepository は DeploymentRepository のテスト用モック
type mockDeploymentRepository struct {
	findByIDFunc             func(ctx context.Context, deploymentID string) (*models.Deployment, error)
	updateAppStatusFunc      func(ctx context.Context, deploymentID string, appStatus models.AppStatus) error
	updateK8sStatusFunc      func(ctx context.Context, deploymentID string, k8sStatus datatypes.JSON) error
	updateDeleteProgressFunc func(ctx context.Context, deploymentID string, progress string) error
	deleteFunc               func(ctx context.Context, deploymentID string) error
	findAllRunningFunc       func(ctx context.Context) ([]models.Deployment, error)
}

func (mock *mockDeploymentRepository) Create(ctx context.Context, deployment *models.Deployment) error { return nil }
func (mock *mockDeploymentRepository) CreateWithTx(ctx context.Context, tx *gorm.DB, deployment *models.Deployment) error { return nil }
func (mock *mockDeploymentRepository) FindByID(ctx context.Context, deploymentID string) (*models.Deployment, error) {
	if mock.findByIDFunc != nil {
		return mock.findByIDFunc(ctx, deploymentID)
	}
	return nil, nil
}
func (mock *mockDeploymentRepository) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, deploymentID string) (*models.Deployment, error) { return nil, nil }
func (mock *mockDeploymentRepository) FindAllByProjectID(ctx context.Context, projectID string) ([]models.Deployment, error) { return nil, nil }
func (mock *mockDeploymentRepository) FindAllRunning(ctx context.Context) ([]models.Deployment, error) {
	if mock.findAllRunningFunc != nil {
		return mock.findAllRunningFunc(ctx)
	}
	return nil, nil
}
func (mock *mockDeploymentRepository) Save(ctx context.Context, deployment *models.Deployment) error { return nil }
func (mock *mockDeploymentRepository) Updates(ctx context.Context, tx *gorm.DB, deployment *models.Deployment, values map[string]interface{}) error { return nil }
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
func (mock *mockDeploymentRepository) UpdatePendingImageID(ctx context.Context, deploymentID string, imageID string) error { return nil }
func (mock *mockDeploymentRepository) UpdatePendingGithubCommitSHA(ctx context.Context, deploymentID string, commitSHA string) error { return nil }
func (mock *mockDeploymentRepository) UpdatePendingGithubBuildFields(ctx context.Context, deploymentID string, repoURL string, branch string, commitSHA string, directory string) error { return nil }
func (mock *mockDeploymentRepository) UpdateDeploymentStatus(ctx context.Context, deploymentID string, status models.DeploymentStatus) error { return nil }
func (mock *mockDeploymentRepository) UpdateCurrentBuildID(ctx context.Context, deploymentID string, buildID string) error { return nil }
func (mock *mockDeploymentRepository) ClearCurrentBuildID(ctx context.Context, deploymentID string) error { return nil }
func (mock *mockDeploymentRepository) Delete(ctx context.Context, deploymentID string) error {
	if mock.deleteFunc != nil {
		return mock.deleteFunc(ctx, deploymentID)
	}
	return nil
}

// mockEnvVarMountRepository は EnvVarMountRepository のテスト用最小モック
type mockEnvVarMountRepository struct{}

func (mock *mockEnvVarMountRepository) Create(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error { return nil }
func (mock *mockEnvVarMountRepository) FindByID(ctx context.Context, mountID string) (*models.EnvVarMount, error) { return nil, nil }
func (mock *mockEnvVarMountRepository) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.EnvVarMount, error) { return nil, nil }
func (mock *mockEnvVarMountRepository) FindByDeploymentIDAndEnvVarID(ctx context.Context, deploymentID string, envVarID string) (*models.EnvVarMount, error) { return nil, nil }
func (mock *mockEnvVarMountRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount, status models.EnvVarMountStatus) error { return nil }
func (mock *mockEnvVarMountRepository) Delete(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error { return nil }
func (mock *mockEnvVarMountRepository) DeleteAllByDeploymentID(ctx context.Context, tx *gorm.DB, deploymentID string) error { return nil }
func (mock *mockEnvVarMountRepository) CountByEnvVarID(ctx context.Context, envVarID string) (int64, error) { return 0, nil }

// mockVolumeMountRepository は VolumeMountRepository のテスト用最小モック
type mockVolumeMountRepository struct{}

func (mock *mockVolumeMountRepository) Create(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error { return nil }
func (mock *mockVolumeMountRepository) FindByID(ctx context.Context, mountID string) (*models.VolumeMount, error) { return nil, nil }
func (mock *mockVolumeMountRepository) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.VolumeMount, error) { return nil, nil }
func (mock *mockVolumeMountRepository) FindAllByVolumeID(ctx context.Context, volumeID string) ([]*models.VolumeMount, error) { return nil, nil }
func (mock *mockVolumeMountRepository) FindByDeploymentIDAndMountPath(ctx context.Context, deploymentID string, mountPath string) (*models.VolumeMount, error) { return nil, nil }
func (mock *mockVolumeMountRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount, status models.VolumeMountStatus) error { return nil }
func (mock *mockVolumeMountRepository) Delete(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error { return nil }
func (mock *mockVolumeMountRepository) DeleteAllByDeploymentID(ctx context.Context, tx *gorm.DB, deploymentID string) error { return nil }

// mockApplyHistoryRepository は ApplyHistoryRepository のテスト用最小モック
type mockApplyHistoryRepository struct {
	deleteAllByDeploymentIDFunc func(ctx context.Context, deploymentID string) error
}

func (mock *mockApplyHistoryRepository) Create(ctx context.Context, tx *gorm.DB, history *models.ApplyHistory) error { return nil }
func (mock *mockApplyHistoryRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, history *models.ApplyHistory, status models.ApplyStatus) error { return nil }
func (mock *mockApplyHistoryRepository) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.ApplyHistory, error) { return nil, nil }
func (mock *mockApplyHistoryRepository) DeleteAllByDeploymentID(ctx context.Context, deploymentID string) error {
	if mock.deleteAllByDeploymentIDFunc != nil {
		return mock.deleteAllByDeploymentIDFunc(ctx, deploymentID)
	}
	return nil
}

// mockPodLogChunkRepository は PodLogChunkRepository のテスト用最小モック
type mockPodLogChunkRepository struct{}

func (mock *mockPodLogChunkRepository) Create(ctx context.Context, chunk *models.PodLogChunk) error { return nil }
func (mock *mockPodLogChunkRepository) FindByDeploymentID(ctx context.Context, deploymentID string) ([]models.PodLogChunk, error) { return nil, nil }
func (mock *mockPodLogChunkRepository) FindByDeploymentIDSince(ctx context.Context, deploymentID string, since time.Time) ([]models.PodLogChunk, error) { return nil, nil }
func (mock *mockPodLogChunkRepository) DeleteByDeploymentIDAndPodNameNotIn(ctx context.Context, deploymentID string, activePodNames []string) error { return nil }
func (mock *mockPodLogChunkRepository) DeleteByPodName(ctx context.Context, deploymentID string, podName string) error { return nil }

// mockProjectRepository は ProjectRepository のテスト用最小モック
type mockProjectRepository struct {
	findByIDNoTxFunc func(ctx context.Context, projectID string) (*models.Project, error)
}

func (mock *mockProjectRepository) Create(ctx context.Context, tx *gorm.DB, project *models.Project) error { return nil }
func (mock *mockProjectRepository) FindByID(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error) { return nil, nil }
func (mock *mockProjectRepository) FindByIDNoTx(ctx context.Context, projectID string) (*models.Project, error) {
	if mock.findByIDNoTxFunc != nil {
		return mock.findByIDNoTxFunc(ctx, projectID)
	}
	return nil, nil
}
func (mock *mockProjectRepository) FindByNamespace(ctx context.Context, namespace string) (*models.Project, error) { return nil, nil }
func (mock *mockProjectRepository) FindAllByUserID(ctx context.Context, userID string) ([]*models.Project, error) { return nil, nil }
func (mock *mockProjectRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, project *models.Project, status models.ProjectStatus) error { return nil }
func (mock *mockProjectRepository) UpdateStatusNoTx(ctx context.Context, project *models.Project, status models.ProjectStatus) error { return nil }
func (mock *mockProjectRepository) Save(ctx context.Context, project *models.Project) error { return nil }
func (mock *mockProjectRepository) Delete(ctx context.Context, tx *gorm.DB, project *models.Project) error { return nil }
func (mock *mockProjectRepository) DeleteNoTx(ctx context.Context, project *models.Project) error { return nil }

// インターフェース実装を静的に確認する
var _ repository.DeploymentRepository = (*mockDeploymentRepository)(nil)
var _ repository.EnvVarMountRepository = (*mockEnvVarMountRepository)(nil)
var _ repository.VolumeMountRepository = (*mockVolumeMountRepository)(nil)
var _ repository.ApplyHistoryRepository = (*mockApplyHistoryRepository)(nil)
var _ repository.PodLogChunkRepository = (*mockPodLogChunkRepository)(nil)
var _ repository.ProjectRepository = (*mockProjectRepository)(nil)

// ---- テスト ----

// makeRunningDeployment は Running 状態の k8s Deployment オブジェクトを生成するヘルパー
// calcAppStatus は DeploymentAvailable Condition が True であることを要求するため、Conditions を設定する
func makeRunningDeployment(deploymentID string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy",
			Namespace: "test-ns",
			Labels:    map[string]string{"launchs.org/deployment-id": deploymentID}, // deployment-id ラベルを設定する
		},
		Status: appsv1.DeploymentStatus{
			Replicas:            replicas, // 総レプリカ数を設定する
			ReadyReplicas:       replicas, // 準備完了レプリカ数を設定する
			UpdatedReplicas:     replicas, // 更新済みレプリカ数を設定する
			AvailableReplicas:   replicas, // 利用可能レプリカ数を設定する
			UnavailableReplicas: 0,        // 利用不可レプリカ数を設定する
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable, // Available 条件を設定する
					Status: corev1.ConditionTrue,        // True を設定して running 判定にする
				},
			},
		},
	}
}

// TestHandleDeploymentEvent_ModifiedイベントでappStatusがrunningに更新される はすべてのレプリカが Ready のとき app_status が running になることを確認する
func TestHandleDeploymentEvent_ModifiedイベントでappStatusがrunningに更新される(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	var updatedAppStatus models.AppStatus // 記録用変数を定義する

	deploymentRepo := &mockDeploymentRepository{
		updateAppStatusFunc: func(ctx context.Context, deploymentID string, appStatus models.AppStatus) error {
			updatedAppStatus = appStatus // app_status を記録する
			return nil                   // 成功を返す
		},
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-1"}, nil // ログストリーム開始用に deployment を返す
		},
	}

	// running 状態になるとログストリーム開始のため projectRepo.FindByIDNoTx が呼ばれる
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, Namespace: "test-ns"}, nil // namespace 解決用に project を返す
		},
	}

	fakeK8sClient := k8sfake.NewSimpleClientset() // fake k8s クライアントを生成する
	streamCancelMap := make(map[string]podStreamState) // ストリーム管理マップを生成する
	var streamCancelMu sync.Mutex                       // ミューテックスを生成する

	k8sDeployment := makeRunningDeployment("deployment-1", 1) // Running 状態の k8s Deployment を生成する
	event := watch.Event{
		Type:   watch.Modified,
		Object: k8sDeployment, // Modified イベントを構築する
	}

	handleDeploymentEvent(ctx, event, fakeK8sClient, deploymentRepo, &mockEnvVarMountRepository{}, &mockVolumeMountRepository{}, &mockApplyHistoryRepository{}, &mockPodLogChunkRepository{}, projectRepo, streamCancelMap, &streamCancelMu) // イベントを処理する

	if updatedAppStatus != models.AppStatusRunning { // app_status が running になったことを確認する
		t.Errorf("期待する app_status: %s、実際: %s", models.AppStatusRunning, updatedAppStatus)
	}
}

// TestHandleDeploymentEvent_ラベルなしDeploymentはスキップされる は deployment-id ラベルがない Deployment イベントを無視することを確認する
func TestHandleDeploymentEvent_ラベルなしDeploymentはスキップされる(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	updateCalled := false // UpdateAppStatus が呼ばれたかを記録する変数を定義する

	deploymentRepo := &mockDeploymentRepository{
		updateAppStatusFunc: func(ctx context.Context, deploymentID string, appStatus models.AppStatus) error {
			updateCalled = true // 呼ばれたことを記録する
			return nil
		},
	}

	fakeK8sClient := k8sfake.NewSimpleClientset()      // fake k8s クライアントを生成する
	streamCancelMap := make(map[string]podStreamState) // ストリーム管理マップを生成する
	var streamCancelMu sync.Mutex                       // ミューテックスを生成する

	// ラベルのない k8s Deployment を生成する
	k8sDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-label-deploy",
			Namespace: "test-ns",
			Labels:    map[string]string{}, // deployment-id ラベルなし
		},
	}
	event := watch.Event{Type: watch.Modified, Object: k8sDeployment} // Modified イベントを構築する

	handleDeploymentEvent(ctx, event, fakeK8sClient, deploymentRepo, &mockEnvVarMountRepository{}, &mockVolumeMountRepository{}, &mockApplyHistoryRepository{}, &mockPodLogChunkRepository{}, &mockProjectRepository{}, streamCancelMap, &streamCancelMu) // イベントを処理する

	if updateCalled { // UpdateAppStatus が呼ばれていないことを確認する
		t.Error("ラベルなし Deployment でも UpdateAppStatus が呼ばれました")
	}
}

// TestHandleDeploymentEvent_DeletingステータスのDeployment削除はDBレコードを削除する は status=deleting の Deployment 削除時に DB レコードが削除されることを確認する
func TestHandleDeploymentEvent_DeletingステータスのDeployment削除はDBレコードを削除する(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	deleteCalled := false // Delete が呼ばれたかを記録する変数を定義する

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{
				ID:     deploymentID,
				Status: models.DeploymentStatusDeleting, // deleting 状態を返す
			}, nil
		},
		deleteFunc: func(ctx context.Context, deploymentID string) error {
			deleteCalled = true // 呼ばれたことを記録する
			return nil
		},
	}

	fakeK8sClient := k8sfake.NewSimpleClientset()      // fake k8s クライアントを生成する
	streamCancelMap := make(map[string]podStreamState) // ストリーム管理マップを生成する
	var streamCancelMu sync.Mutex                       // ミューテックスを生成する

	k8sDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-deploy",
			Namespace:         "test-ns",
			Labels:            map[string]string{"launchs.org/deployment-id": "deployment-del"},
			DeletionTimestamp: nil, // Terminating 中ではない（完全削除済み）
		},
	}
	event := watch.Event{Type: watch.Deleted, Object: k8sDeployment} // Deleted イベントを構築する

	handleDeploymentEvent(ctx, event, fakeK8sClient, deploymentRepo, &mockEnvVarMountRepository{}, &mockVolumeMountRepository{}, &mockApplyHistoryRepository{}, &mockPodLogChunkRepository{}, &mockProjectRepository{}, streamCancelMap, &streamCancelMu) // イベントを処理する

	if !deleteCalled { // Delete が呼ばれたことを確認する
		t.Error("deleting 状態の Deployment 削除で Delete が呼ばれませんでした")
	}
}
