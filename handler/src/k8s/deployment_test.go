package k8s

import (
	"handler/models"
	"handler/repository"
	"context"
	"sync"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// テスト用の Deployment manifest を生成するヘルパー関数
func newTestDeploymentManifest(name, namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name}, // セレクターを設定する
			},
		},
	}
}

// TestApplyDeployment_正常にDeploymentが作成される は ApplyDeployment で k8s に Deployment が作成されることを確認する
func TestApplyDeployment_正常にDeploymentが作成される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	deploymentManifest := newTestDeploymentManifest("test-deploy", "test-namespace") // テスト用 manifest を生成する

	err := ApplyDeployment(ctx, fakeClient, deploymentManifest) // Deployment を apply する
	if err != nil {
		t.Fatalf("ApplyDeployment() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	createdDeployment, err := fakeClient.AppsV1().Deployments("test-namespace").Get(ctx, "test-deploy", metav1.GetOptions{}) // 作成された Deployment を取得する
	if err != nil {
		t.Fatalf("Deployment の取得に失敗しました: %v", err) // 取得失敗時はテスト失敗とする
	}
	if createdDeployment.Name != "test-deploy" { // Deployment 名を確認する
		t.Errorf("期待する Deployment 名: test-deploy, 実際: %s", createdDeployment.Name)
	}
}

// TestApplyDeployment_同名Deploymentを再applyすると更新される は同名の Deployment を再度 apply すると更新されることを確認する
func TestApplyDeployment_同名Deploymentを再applyすると更新される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	deploymentManifest := newTestDeploymentManifest("update-deploy", "test-namespace") // テスト用 manifest を生成する

	err := ApplyDeployment(ctx, fakeClient, deploymentManifest) // 1回目の apply（作成）
	if err != nil {
		t.Fatalf("1回目の ApplyDeployment() がエラーを返しました: %v", err) // 1回目は成功するべきなのでテスト失敗とする
	}

	updatedManifest := newTestDeploymentManifest("update-deploy", "test-namespace") // 更新用 manifest を生成する
	updatedManifest.Labels = map[string]string{"updated": "true"}                  // ラベルを追加して更新内容を確認できるようにする

	err = ApplyDeployment(ctx, fakeClient, updatedManifest) // 2回目の apply（更新）
	if err != nil {
		t.Fatalf("2回目の ApplyDeployment() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	updatedDeployment, err := fakeClient.AppsV1().Deployments("test-namespace").Get(ctx, "update-deploy", metav1.GetOptions{}) // 更新後の Deployment を取得する
	if err != nil {
		t.Fatalf("更新後の Deployment 取得に失敗しました: %v", err) // 取得失敗時はテスト失敗とする
	}
	if updatedDeployment.Labels["updated"] != "true" { // 更新が反映されていることを確認する
		t.Errorf("Deployment の更新が反映されていません。Labels: %v", updatedDeployment.Labels)
	}
}

// TestDeleteDeployment_正常にDeploymentが削除される は DeleteDeployment で k8s から Deployment が削除されることを確認する
func TestDeleteDeployment_正常にDeploymentが削除される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	deploymentManifest := newTestDeploymentManifest("delete-deploy", "test-namespace") // テスト用 manifest を生成する

	err := ApplyDeployment(ctx, fakeClient, deploymentManifest) // 削除対象の Deployment を作成する
	if err != nil {
		t.Fatalf("事前の ApplyDeployment() がエラーを返しました: %v", err) // 前提条件の作成失敗時はテスト失敗とする
	}

	err = DeleteDeployment(ctx, fakeClient, "test-namespace", "delete-deploy") // Deployment を削除する
	if err != nil {
		t.Fatalf("DeleteDeployment() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	_, err = fakeClient.AppsV1().Deployments("test-namespace").Get(ctx, "delete-deploy", metav1.GetOptions{}) // 削除後に取得を試みる
	if err == nil {
		t.Fatal("削除後も Deployment が存在しています") // 削除後に取得できた場合はテスト失敗とする
	}
}

// mockDeploymentRepo は DeploymentRepository のテスト用モック
type mockDeploymentRepo struct {
	deletedIDs       []string                                                                     // 削除された deployment ID を記録する
	findByIDFunc     func(ctx context.Context, deploymentID string) (*models.Deployment, error)   // FindByID のモック関数
	updatedK8sStatus datatypes.JSON                                                               // UpdateK8sStatus で更新された値を記録する
}

func (mock *mockDeploymentRepo) Create(ctx context.Context, deployment *models.Deployment) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepo) FindByID(ctx context.Context, deploymentID string) (*models.Deployment, error) {
	if mock.findByIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDFunc(ctx, deploymentID)
	}
	return &models.Deployment{ID: deploymentID, Status: models.DeploymentStatusRunning}, nil // デフォルトは running を返す
}
func (mock *mockDeploymentRepo) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, deploymentID string) (*models.Deployment, error) {
	return nil, nil // 使用しない
}
func (mock *mockDeploymentRepo) FindAllByProjectID(ctx context.Context, projectID string) ([]models.Deployment, error) {
	return nil, nil // 使用しない
}
func (mock *mockDeploymentRepo) FindAllRunning(ctx context.Context) ([]models.Deployment, error) {
	return nil, nil // テストでは使用しない
}
func (mock *mockDeploymentRepo) Save(ctx context.Context, deployment *models.Deployment) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepo) Updates(ctx context.Context, tx *gorm.DB, deployment *models.Deployment, values map[string]interface{}) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepo) UpdateAppStatus(ctx context.Context, deploymentID string, appStatus models.AppStatus) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepo) UpdateK8sStatus(ctx context.Context, deploymentID string, k8sStatus datatypes.JSON) error {
	mock.updatedK8sStatus = k8sStatus // 更新された k8s_status を記録する
	return nil
}
func (mock *mockDeploymentRepo) UpdatePendingImageID(ctx context.Context, deploymentID string, imageID string) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepo) UpdatePendingGithubCommitSHA(ctx context.Context, deploymentID string, commitSHA string) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepo) UpdatePendingGithubBuildFields(ctx context.Context, deploymentID string, repoURL string, branch string, commitSHA string, directory string) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepo) UpdateDeleteProgress(ctx context.Context, deploymentID string, progress string) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepo) Delete(ctx context.Context, deploymentID string) error {
	mock.deletedIDs = append(mock.deletedIDs, deploymentID) // 削除 ID を記録する
	return nil
}

func (mock *mockDeploymentRepo) UpdateDeploymentStatus(ctx context.Context, deploymentID string, status models.DeploymentStatus) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *mockDeploymentRepo) UpdateCurrentBuildID(ctx context.Context, deploymentID string, buildID string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *mockDeploymentRepo) ClearCurrentBuildID(ctx context.Context, deploymentID string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}
func (mock *mockDeploymentRepo) CreateWithTx(ctx context.Context, tx *gorm.DB, deployment *models.Deployment) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

// mockEnvVarMountRepoForDeployment は EnvVarMountRepository のテスト用モック
type mockEnvVarMountRepoForDeployment struct {
	findAllFunc func(ctx context.Context, deploymentID string) ([]*models.EnvVarMount, error)
	deletedIDs  []string // 削除されたマウント ID を記録する
}

func (mock *mockEnvVarMountRepoForDeployment) Create(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error {
	return nil // 使用しない
}
func (mock *mockEnvVarMountRepoForDeployment) FindByID(ctx context.Context, mountID string) (*models.EnvVarMount, error) {
	return nil, nil // 使用しない
}
func (mock *mockEnvVarMountRepoForDeployment) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.EnvVarMount, error) {
	if mock.findAllFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findAllFunc(ctx, deploymentID)
	}
	return []*models.EnvVarMount{}, nil // デフォルトは空スライスを返す
}
func (mock *mockEnvVarMountRepoForDeployment) FindByDeploymentIDAndEnvVarID(ctx context.Context, deploymentID string, envVarID string) (*models.EnvVarMount, error) {
	return nil, nil // 使用しない
}
func (mock *mockEnvVarMountRepoForDeployment) UpdateStatus(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount, status models.EnvVarMountStatus) error {
	return nil // 使用しない
}
func (mock *mockEnvVarMountRepoForDeployment) Delete(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error {
	mock.deletedIDs = append(mock.deletedIDs, mount.ID) // 削除 ID を記録する
	return nil
}
func (mock *mockEnvVarMountRepoForDeployment) DeleteAllByDeploymentID(ctx context.Context, tx *gorm.DB, deploymentID string) error {
	return nil // 使用しない
}
func (mock *mockEnvVarMountRepoForDeployment) CountByEnvVarID(ctx context.Context, envVarID string) (int64, error) {
	return 0, nil // 使用しない
}

// mockVolumeMountRepoForDeployment は VolumeMountRepository のテスト用モック
type mockVolumeMountRepoForDeployment struct{}

func (mock *mockVolumeMountRepoForDeployment) Create(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error {
	return nil // 使用しない
}
func (mock *mockVolumeMountRepoForDeployment) FindByID(ctx context.Context, mountID string) (*models.VolumeMount, error) {
	return nil, nil // 使用しない
}
func (mock *mockVolumeMountRepoForDeployment) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.VolumeMount, error) {
	return []*models.VolumeMount{}, nil // 空スライスを返す
}
func (mock *mockVolumeMountRepoForDeployment) FindAllByVolumeID(ctx context.Context, volumeID string) ([]*models.VolumeMount, error) {
	return []*models.VolumeMount{}, nil // 使用しない
}
func (mock *mockVolumeMountRepoForDeployment) FindByDeploymentIDAndMountPath(ctx context.Context, deploymentID string, mountPath string) (*models.VolumeMount, error) {
	return nil, nil // 使用しない
}
func (mock *mockVolumeMountRepoForDeployment) UpdateStatus(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount, status models.VolumeMountStatus) error {
	return nil // 使用しない
}
func (mock *mockVolumeMountRepoForDeployment) DeleteAllByDeploymentID(ctx context.Context, tx *gorm.DB, deploymentID string) error {
	return nil // 使用しない
}
func (mock *mockVolumeMountRepoForDeployment) Delete(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error {
	return nil // 使用しない
}

// mockApplyHistoryRepoForDeployment は ApplyHistoryRepository のテスト用モック
type mockApplyHistoryRepoForDeployment struct {
	deletedDeploymentIDs []string // DeleteAllByDeploymentID で削除された ID を記録する
}

func (mock *mockApplyHistoryRepoForDeployment) Create(ctx context.Context, tx *gorm.DB, history *models.ApplyHistory) error {
	return nil // 使用しない
}
func (mock *mockApplyHistoryRepoForDeployment) UpdateStatus(ctx context.Context, tx *gorm.DB, history *models.ApplyHistory, status models.ApplyStatus) error {
	return nil // 使用しない
}
func (mock *mockApplyHistoryRepoForDeployment) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.ApplyHistory, error) {
	return nil, nil // 使用しない
}
func (mock *mockApplyHistoryRepoForDeployment) DeleteAllByDeploymentID(ctx context.Context, deploymentID string) error {
	mock.deletedDeploymentIDs = append(mock.deletedDeploymentIDs, deploymentID) // 削除 ID を記録する
	return nil
}

// mockBuildRepoForDeployment は DeploymentBuildRepository のテスト用モック
type mockBuildRepoForDeployment struct {
	deletedDeploymentIDs []string // DeleteAllByDeploymentID で削除された ID を記録する
}

func (mock *mockBuildRepoForDeployment) Create(ctx context.Context, build *models.DeploymentBuild) error {
	return nil // 使用しない
}
func (mock *mockBuildRepoForDeployment) FindByID(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
	return nil, nil // 使用しない
}
func (mock *mockBuildRepoForDeployment) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]models.DeploymentBuild, error) {
	return nil, nil // 使用しない
}
func (mock *mockBuildRepoForDeployment) FindAllBuilding(ctx context.Context) ([]models.DeploymentBuild, error) {
	return nil, nil // 使用しない
}
func (mock *mockBuildRepoForDeployment) UpdateStatus(ctx context.Context, buildID string, status models.BuildStatus) error {
	return nil // 使用しない
}
func (mock *mockBuildRepoForDeployment) UpdateK8sJobName(ctx context.Context, buildID string, jobName string) error {
	return nil // 使用しない
}
func (mock *mockBuildRepoForDeployment) UpdateBuildResult(ctx context.Context, buildID string, status models.BuildStatus, finishedAt time.Time) error {
	return nil // 使用しない
}
func (mock *mockBuildRepoForDeployment) Delete(ctx context.Context, build *models.DeploymentBuild) error {
	return nil // 使用しない
}

func (mock *mockBuildRepoForDeployment) DeleteAllByDeploymentID(ctx context.Context, deploymentID string) error {
	mock.deletedDeploymentIDs = append(mock.deletedDeploymentIDs, deploymentID) // 削除 ID を記録する
	return nil
}

func (mock *mockBuildRepoForDeployment) FindAllByProjectID(ctx context.Context, projectID string) ([]models.DeploymentBuild, error) {
	return nil, nil // 使用しない
}

func (mock *mockBuildRepoForDeployment) DeleteAllByProjectID(ctx context.Context, db *gorm.DB, projectID string) error {
	return nil // 使用しない
}

// mockPodLogChunkRepoForDeployment は PodLogChunkRepository のテスト用モック
type mockPodLogChunkRepoForDeployment struct{}

func (mock *mockPodLogChunkRepoForDeployment) Create(ctx context.Context, chunk *models.PodLogChunk) error {
	return nil // 使用しない
}
func (mock *mockPodLogChunkRepoForDeployment) FindByDeploymentID(ctx context.Context, deploymentID string) ([]models.PodLogChunk, error) {
	return nil, nil // 使用しない
}
func (mock *mockPodLogChunkRepoForDeployment) FindByDeploymentIDSince(ctx context.Context, deploymentID string, since time.Time) ([]models.PodLogChunk, error) {
	return nil, nil // 使用しない
}
func (mock *mockPodLogChunkRepoForDeployment) DeleteByDeploymentIDAndPodNameNotIn(ctx context.Context, deploymentID string, activePodNames []string) error {
	return nil // 使用しない
}
func (mock *mockPodLogChunkRepoForDeployment) DeleteByPodName(ctx context.Context, deploymentID string, podName string) error {
	return nil // 使用しない
}

// mockProjectRepoForDeployment は ProjectRepository のテスト用モック
type mockProjectRepoForDeployment struct{}

func (mock *mockProjectRepoForDeployment) Create(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil // 使用しない
}
func (mock *mockProjectRepoForDeployment) FindByID(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error) {
	return nil, nil // 使用しない
}
func (mock *mockProjectRepoForDeployment) FindByIDNoTx(ctx context.Context, projectID string) (*models.Project, error) {
	return &models.Project{ID: projectID, Namespace: "test-namespace"}, nil // テスト用 project を返す
}
func (mock *mockProjectRepoForDeployment) FindAllByUserID(ctx context.Context, userID string) ([]*models.Project, error) {
	return nil, nil // 使用しない
}
func (mock *mockProjectRepoForDeployment) UpdateStatus(ctx context.Context, tx *gorm.DB, project *models.Project, status models.ProjectStatus) error {
	return nil // 使用しない
}
func (mock *mockProjectRepoForDeployment) Save(ctx context.Context, project *models.Project) error {
	return nil // 使用しない
}
func (mock *mockProjectRepoForDeployment) Delete(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil // 使用しない
}
func (mock *mockProjectRepoForDeployment) DeleteNoTx(ctx context.Context, project *models.Project) error {
	return nil // 使用しない
}
func (mock *mockProjectRepoForDeployment) FindByNamespace(ctx context.Context, namespace string) (*models.Project, error) {
	return nil, nil // 使用しない
}

// 型アサーションで interface を実装していることを確認する
var _ repository.DeploymentRepository = &mockDeploymentRepo{}
var _ repository.EnvVarMountRepository = &mockEnvVarMountRepoForDeployment{}
var _ repository.VolumeMountRepository = &mockVolumeMountRepoForDeployment{}
var _ repository.ApplyHistoryRepository = &mockApplyHistoryRepoForDeployment{}
var _ repository.DeploymentBuildRepository = &mockBuildRepoForDeployment{}
var _ repository.PodLogChunkRepository = &mockPodLogChunkRepoForDeployment{}
var _ repository.ProjectRepository = &mockProjectRepoForDeployment{}

// newTestHandleDeploymentEventArgs はテスト用のデフォルト引数を返すヘルパー関数
func newTestHandleDeploymentEventArgs() (
	fakeK8sClient *fake.Clientset,
	deploymentRepo *mockDeploymentRepo,
	envVarMountRepo *mockEnvVarMountRepoForDeployment,
	volumeMountRepo *mockVolumeMountRepoForDeployment,
	applyHistoryRepo *mockApplyHistoryRepoForDeployment,
	podLogChunkRepo *mockPodLogChunkRepoForDeployment,
	projectRepo *mockProjectRepoForDeployment,
	streamCancelMap map[string]podStreamState,
	streamCancelMu *sync.Mutex,
) {
	fakeK8sClient = fake.NewSimpleClientset()               // fake k8s クライアントを生成する
	deploymentRepo = &mockDeploymentRepo{}                  // deployment リポジトリのモックを生成する
	envVarMountRepo = &mockEnvVarMountRepoForDeployment{}   // env_var_mount リポジトリのモックを生成する
	volumeMountRepo = &mockVolumeMountRepoForDeployment{}   // volume_mount リポジトリのモックを生成する
	applyHistoryRepo = &mockApplyHistoryRepoForDeployment{} // apply_history リポジトリのモックを生成する
	podLogChunkRepo = &mockPodLogChunkRepoForDeployment{}   // pod_log_chunk リポジトリのモックを生成する
	projectRepo = &mockProjectRepoForDeployment{}           // project リポジトリのモックを生成する
	streamCancelMap = make(map[string]podStreamState)       // ストリーム状態マップを生成する
	streamCancelMu = &sync.Mutex{}                          // ミューテックスを生成する
	return
}

// TestHandleDeploymentEvent_statusがdeletingの場合に連鎖削除される はDB status が deleting の時にDeletedイベントで連鎖削除されることを確認する
func TestHandleDeploymentEvent_statusがdeletingの場合に連鎖削除される(t *testing.T) {
	deploymentID := "test-deployment-id" // テスト対象の deployment ID を定義する
	fakeK8sClient, deploymentRepo, envVarMountRepo, volumeMountRepo, applyHistoryRepo, podLogChunkRepo, projectRepo, streamCancelMap, streamCancelMu := newTestHandleDeploymentEventArgs()

	deploymentRepo.findByIDFunc = func(ctx context.Context, id string) (*models.Deployment, error) {
		return &models.Deployment{ID: id, Status: models.DeploymentStatusDeleting}, nil // status が deleting の deployment を返す
	}
	envVarMountMount := &models.EnvVarMount{ID: "mount-1"} // テスト用の EnvVarMount を用意する
	envVarMountRepo.findAllFunc = func(ctx context.Context, id string) ([]*models.EnvVarMount, error) {
		return []*models.EnvVarMount{envVarMountMount}, nil // 1件の EnvVarMount を返す
	}

	k8sDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-deploy",
			Labels: map[string]string{"launchs.org/deployment-id": deploymentID}, // deployment-id ラベルを設定する
		},
	}
	event := watch.Event{Type: watch.Deleted, Object: k8sDeployment} // Deleted イベントを生成する

	ctx := context.Background()                                                                                                                                                                                                         // テスト用コンテキストを生成する
	handleDeploymentEvent(ctx, event, fakeK8sClient, deploymentRepo, envVarMountRepo, volumeMountRepo, applyHistoryRepo, podLogChunkRepo, projectRepo, streamCancelMap, streamCancelMu) // イベントを処理する

	// Deployment レコードが削除されたことを確認する
	if len(deploymentRepo.deletedIDs) != 1 || deploymentRepo.deletedIDs[0] != deploymentID {
		t.Errorf("期待する削除 deploymentID: %s, 実際: %v", deploymentID, deploymentRepo.deletedIDs)
	}
	// EnvVarMount レコードが削除されたことを確認する
	if len(envVarMountRepo.deletedIDs) != 1 || envVarMountRepo.deletedIDs[0] != "mount-1" {
		t.Errorf("期待する削除 EnvVarMount ID: mount-1, 実際: %v", envVarMountRepo.deletedIDs)
	}
	// ApplyHistory が全件削除されたことを確認する
	if len(applyHistoryRepo.deletedDeploymentIDs) != 1 || applyHistoryRepo.deletedDeploymentIDs[0] != deploymentID {
		t.Errorf("期待する削除 deploymentID: %s, 実際: %v", deploymentID, applyHistoryRepo.deletedDeploymentIDs)
	}
	// DeploymentBuild はDeployment削除時に削除しない（Project単位で管理するため）
}

// TestHandleDeploymentEvent_statusがdeletingでない場合はk8sStatusをdeletedに更新する はDB status が deleting 以外の時にk8s_statusのみ更新されることを確認する
func TestHandleDeploymentEvent_statusがdeletingでない場合はk8sStatusをdeletedに更新する(t *testing.T) {
	deploymentID := "test-deployment-id-2" // テスト対象の deployment ID を定義する
	fakeK8sClient, deploymentRepo, envVarMountRepo, volumeMountRepo, applyHistoryRepo, podLogChunkRepo, projectRepo, streamCancelMap, streamCancelMu := newTestHandleDeploymentEventArgs()

	deploymentRepo.findByIDFunc = func(ctx context.Context, id string) (*models.Deployment, error) {
		return &models.Deployment{ID: id, Status: models.DeploymentStatusRunning}, nil // status が running の deployment を返す（意図しない削除）
	}

	k8sDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-deploy-2",
			Labels: map[string]string{"launchs.org/deployment-id": deploymentID}, // deployment-id ラベルを設定する
		},
	}
	event := watch.Event{Type: watch.Deleted, Object: k8sDeployment} // Deleted イベントを生成する

	ctx := context.Background()                                                                                                                                                                                                         // テスト用コンテキストを生成する
	handleDeploymentEvent(ctx, event, fakeK8sClient, deploymentRepo, envVarMountRepo, volumeMountRepo, applyHistoryRepo, podLogChunkRepo, projectRepo, streamCancelMap, streamCancelMu) // イベントを処理する

	// Deployment レコードが削除されていないことを確認する
	if len(deploymentRepo.deletedIDs) != 0 {
		t.Errorf("Deployment が削除されるべきではありませんが削除されました: %v", deploymentRepo.deletedIDs)
	}
	// k8s_status が {"deleted":true} に更新されていることを確認する
	if string(deploymentRepo.updatedK8sStatus) != `{"deleted":true}` {
		t.Errorf("期待する k8s_status: {\"deleted\":true}, 実際: %s", string(deploymentRepo.updatedK8sStatus))
	}
	// 連鎖削除が実行されていないことを確認する
	if len(applyHistoryRepo.deletedDeploymentIDs) != 0 {
		t.Errorf("ApplyHistory が削除されるべきではありませんが削除されました: %v", applyHistoryRepo.deletedDeploymentIDs)
	}
}
