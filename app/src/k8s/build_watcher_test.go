package k8s

import (
	"app/models"
	"app/repository"
	"context"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// --- モック実装 ---

// mockBuildRepo は DeploymentBuildRepository のテスト用モック
type mockBuildRepo struct {
	findByIDFn       func(ctx context.Context, buildID string) (*models.DeploymentBuild, error)
	findAllBuildingFn func(ctx context.Context) ([]models.DeploymentBuild, error)
	updateStatusFn   func(ctx context.Context, buildID string, status models.BuildStatus) error
	updateBuildResultFn func(ctx context.Context, buildID string, status models.BuildStatus, builtImageURL string, finishedAt time.Time) error
}

func (mock *mockBuildRepo) Create(ctx context.Context, build *models.DeploymentBuild) error {
	return nil // テストでは使用しない
}
func (mock *mockBuildRepo) FindByID(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
	return mock.findByIDFn(ctx, buildID) // モック関数を呼び出す
}
func (mock *mockBuildRepo) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]models.DeploymentBuild, error) {
	return nil, nil // テストでは使用しない
}
func (mock *mockBuildRepo) FindAllBuilding(ctx context.Context) ([]models.DeploymentBuild, error) {
	if mock.findAllBuildingFn != nil {
		return mock.findAllBuildingFn(ctx) // モック関数を呼び出す
	}
	return nil, nil // デフォルトは空リストを返す
}
func (mock *mockBuildRepo) UpdateStatus(ctx context.Context, buildID string, status models.BuildStatus) error {
	if mock.updateStatusFn != nil {
		return mock.updateStatusFn(ctx, buildID, status) // モック関数を呼び出す
	}
	return nil // デフォルトは正常終了を返す
}
func (mock *mockBuildRepo) UpdateK8sJobName(ctx context.Context, buildID string, jobName string) error {
	return nil // テストでは使用しない
}
func (mock *mockBuildRepo) UpdateBuildResult(ctx context.Context, buildID string, status models.BuildStatus, builtImageURL string, finishedAt time.Time) error {
	if mock.updateBuildResultFn != nil {
		return mock.updateBuildResultFn(ctx, buildID, status, builtImageURL, finishedAt) // モック関数を呼び出す
	}
	return nil // デフォルトは正常終了を返す
}

func (mock *mockBuildRepo) DeleteAllByDeploymentID(ctx context.Context, deploymentID string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

// mockLogChunkRepo は BuildLogChunkRepository のテスト用モック
type mockLogChunkRepo struct {
	createFn func(ctx context.Context, chunk *models.BuildLogChunk) error
}

func (mock *mockLogChunkRepo) Create(ctx context.Context, chunk *models.BuildLogChunk) error {
	if mock.createFn != nil {
		return mock.createFn(ctx, chunk) // モック関数を呼び出す
	}
	return nil // デフォルトは正常終了を返す
}
func (mock *mockLogChunkRepo) FindByBuildID(ctx context.Context, buildID string) ([]models.BuildLogChunk, error) {
	return nil, nil // テストでは使用しない
}
func (mock *mockLogChunkRepo) FindByBuildIDSince(ctx context.Context, buildID string, since time.Time) ([]models.BuildLogChunk, error) {
	return nil, nil // テストでは使用しない
}

// mockDeploymentRepo は DeploymentRepository のテスト用モック
type mockDeploymentRepoForBuild struct {
	findByIDFn              func(ctx context.Context, deploymentID string) (*models.Deployment, error)
	updatePendingImageURLFn func(ctx context.Context, deploymentID string, imageURL string) error
}

func (mock *mockDeploymentRepoForBuild) Create(ctx context.Context, deployment *models.Deployment) error {
	return nil // テストでは使用しない
}
func (mock *mockDeploymentRepoForBuild) FindByID(ctx context.Context, deploymentID string) (*models.Deployment, error) {
	if mock.findByIDFn != nil {
		return mock.findByIDFn(ctx, deploymentID) // モック関数を呼び出す
	}
	return nil, gorm.ErrRecordNotFound // デフォルトはレコードなしエラーを返す
}
func (mock *mockDeploymentRepoForBuild) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, deploymentID string) (*models.Deployment, error) {
	return nil, nil // テストでは使用しない
}
func (mock *mockDeploymentRepoForBuild) FindAllByProjectID(ctx context.Context, projectID string) ([]models.Deployment, error) {
	return nil, nil // テストでは使用しない
}
func (mock *mockDeploymentRepoForBuild) Save(ctx context.Context, deployment *models.Deployment) error {
	return nil // テストでは使用しない
}
func (mock *mockDeploymentRepoForBuild) Updates(ctx context.Context, tx *gorm.DB, deployment *models.Deployment, values map[string]interface{}) error {
	return nil // テストでは使用しない
}
func (mock *mockDeploymentRepoForBuild) UpdateAppStatus(ctx context.Context, deploymentID string, appStatus models.AppStatus) error {
	return nil // テストでは使用しない
}
func (mock *mockDeploymentRepoForBuild) UpdateK8sStatus(ctx context.Context, deploymentID string, k8sStatus datatypes.JSON) error {
	return nil // テストでは使用しない
}
func (mock *mockDeploymentRepoForBuild) UpdatePendingImageURL(ctx context.Context, deploymentID string, imageURL string) error {
	if mock.updatePendingImageURLFn != nil {
		return mock.updatePendingImageURLFn(ctx, deploymentID, imageURL) // モック関数を呼び出す
	}
	return nil // デフォルトは正常終了を返す
}

func (mock *mockDeploymentRepoForBuild) UpdatePendingGithubCommitSHA(ctx context.Context, deploymentID string, commitSHA string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}
func (mock *mockDeploymentRepoForBuild) UpdateDeleteProgress(ctx context.Context, deploymentID string, progress string) error {
	return nil // テストでは使用しない
}
func (mock *mockDeploymentRepoForBuild) Delete(ctx context.Context, deploymentID string) error {
	return nil // テストでは使用しない
}
func (mock *mockDeploymentRepoForBuild) FindAllRunning(ctx context.Context) ([]models.Deployment, error) {
	return nil, nil // テストでは使用しない
}

// mockProjectRepoForBuild は ProjectRepository のテスト用モック
type mockProjectRepoForBuild struct {
	findByIDNoTxFn func(ctx context.Context, projectID string) (*models.Project, error)
}

func (mock *mockProjectRepoForBuild) Create(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil // テストでは使用しない
}
func (mock *mockProjectRepoForBuild) FindByID(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error) {
	return nil, nil // テストでは使用しない
}
func (mock *mockProjectRepoForBuild) FindByIDNoTx(ctx context.Context, projectID string) (*models.Project, error) {
	if mock.findByIDNoTxFn != nil {
		return mock.findByIDNoTxFn(ctx, projectID) // モック関数を呼び出す
	}
	return nil, gorm.ErrRecordNotFound // デフォルトはレコードなしエラーを返す
}
func (mock *mockProjectRepoForBuild) FindByNamespace(ctx context.Context, namespace string) (*models.Project, error) {
	return nil, nil // テストでは使用しない
}
func (mock *mockProjectRepoForBuild) FindAllByUserID(ctx context.Context, userID string) ([]*models.Project, error) {
	return nil, nil // テストでは使用しない
}
func (mock *mockProjectRepoForBuild) UpdateStatus(ctx context.Context, tx *gorm.DB, project *models.Project, status models.ProjectStatus) error {
	return nil // テストでは使用しない
}
func (mock *mockProjectRepoForBuild) Save(ctx context.Context, project *models.Project) error {
	return nil // テストでは使用しない
}
func (mock *mockProjectRepoForBuild) Delete(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil // テストでは使用しない
}
func (mock *mockProjectRepoForBuild) DeleteNoTx(ctx context.Context, project *models.Project) error {
	return nil // テストでは使用しない
}

// mockHarborCredentialRepoForBuild は HarborCredentialRepository のテスト用モック
type mockHarborCredentialRepoForBuild struct {
	findByProjectIDNoTxFn func(ctx context.Context, projectID string) (*models.HarborCredential, error)
}

func (mock *mockHarborCredentialRepoForBuild) Create(ctx context.Context, tx *gorm.DB, credential *models.HarborCredential) error {
	return nil // テストでは使用しない
}
func (mock *mockHarborCredentialRepoForBuild) FindByProjectID(ctx context.Context, tx *gorm.DB, projectID string) (*models.HarborCredential, error) {
	return nil, nil // テストでは使用しない
}
func (mock *mockHarborCredentialRepoForBuild) FindByProjectIDNoTx(ctx context.Context, projectID string) (*models.HarborCredential, error) {
	if mock.findByProjectIDNoTxFn != nil {
		return mock.findByProjectIDNoTxFn(ctx, projectID) // モック関数を呼び出す
	}
	return nil, gorm.ErrRecordNotFound // デフォルトはレコードなしエラーを返す
}
func (mock *mockHarborCredentialRepoForBuild) DeleteByProjectID(ctx context.Context, tx *gorm.DB, projectID string) error {
	return nil // テストでは使用しない
}

// --- ヘルパー ---

// newTestJobWithStatus はテスト用の Job を生成する
func newTestJobWithStatus(buildID, namespace string, active, succeeded, failed int32) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "railpack-" + buildID, // Job 名を設定する
			Namespace: namespace,             // namespace を設定する
			Labels: map[string]string{
				"build-job-id": buildID, // build-job-id ラベルを設定する
			},
		},
		Status: batchv1.JobStatus{
			Active:    active,    // Active Pod 数を設定する
			Succeeded: succeeded, // 成功 Pod 数を設定する
			Failed:    failed,    // 失敗 Pod 数を設定する
		},
	}
}

// newTestRepositories はテスト用リポジトリ群を生成するヘルパー
func newTestRepositories(buildData *models.DeploymentBuild, deploymentData *models.Deployment, projectData *models.Project, harborCred *models.HarborCredential) (
	*mockBuildRepo,
	*mockLogChunkRepo,
	*mockDeploymentRepoForBuild,
	*mockProjectRepoForBuild,
	*mockHarborCredentialRepoForBuild,
) {
	buildRepo := &mockBuildRepo{
		findByIDFn: func(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
			return buildData, nil // テスト用ビルドレコードを返す
		},
	}
	logChunkRepo := &mockLogChunkRepo{}
	deploymentRepo := &mockDeploymentRepoForBuild{
		findByIDFn: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return deploymentData, nil // テスト用 deployment を返す
		},
	}
	projectRepo := &mockProjectRepoForBuild{
		findByIDNoTxFn: func(ctx context.Context, projectID string) (*models.Project, error) {
			return projectData, nil // テスト用 project を返す
		},
	}
	harborCredentialRepo := &mockHarborCredentialRepoForBuild{
		findByProjectIDNoTxFn: func(ctx context.Context, projectID string) (*models.HarborCredential, error) {
			return harborCred, nil // テスト用 harbor credential を返す
		},
	}
	return buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo
}

// --- テスト ---

// TestHandleBuildJobEvent_ActiveJob_StatusBecomesBuilding は Job が Active になると status が building になることを確認する
func TestHandleBuildJobEvent_ActiveJob_StatusBecomesBuilding(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background()) // キャンセル可能なコンテキストを生成する
	defer cancel()

	buildID := "build-id-001"       // テスト用ビルドID
	namespace := "test-namespace"   // テスト用 namespace

	buildData := &models.DeploymentBuild{
		ID:           buildID,                     // ビルドIDを設定する
		DeploymentID: "deployment-id-001",         // デプロイメントIDを設定する
		Status:       models.BuildStatusPending,   // 初期ステータスを pending に設定する
		BuildType:    models.BuildTypeRailpack,    // ビルドタイプを設定する
	}

	updatedStatus := models.BuildStatusPending // 更新後のステータスを追跡する
	buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo := newTestRepositories(
		buildData,
		&models.Deployment{ID: "deployment-id-001", ProjectID: "project-id-001", Name: "my-app"},
		&models.Project{ID: "project-id-001", Name: "my-project", Namespace: namespace},
		&models.HarborCredential{HarborEndpoint: "harbor.example.com"},
	)
	buildRepo.updateStatusFn = func(ctx context.Context, buildID string, status models.BuildStatus) error {
		updatedStatus = status // 更新されたステータスを記録する
		buildData.Status = status
		return nil
	}

	fakeClient := fake.NewSimpleClientset() // フェイク k8s クライアントを生成する
	activeJob := newTestJobWithStatus(buildID, namespace, 1, 0, 0) // Active 状態の Job を生成する

	event := watch.Event{
		Type:   watch.Modified,    // Modified イベントを設定する
		Object: activeJob,         // Job オブジェクトを設定する
	}

	handleBuildJobEvent(ctx, event, fakeClient, buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo, "") // イベントを処理する

	if updatedStatus != models.BuildStatusBuilding { // ステータスが building になっていることを確認する
		t.Errorf("Job Active 後のステータスは building であるべきですが、%s でした", updatedStatus)
	}
}

// TestHandleBuildJobEvent_SucceededJob_StatusBecomesSucceeded は Job が Succeeded になると status が succeeded になることを確認する
func TestHandleBuildJobEvent_SucceededJob_StatusBecomesSucceeded(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	buildID := "build-id-002"       // テスト用ビルドID
	namespace := "test-namespace"   // テスト用 namespace

	buildData := &models.DeploymentBuild{
		ID:           buildID,                     // ビルドIDを設定する
		DeploymentID: "deployment-id-002",         // デプロイメントIDを設定する
		Status:       models.BuildStatusBuilding,  // building 状態を設定する
		BuildType:    models.BuildTypeRailpack,    // ビルドタイプを設定する
	}

	var resultStatus models.BuildStatus // 更新後のステータスを追跡する
	var resultImageURL string           // 更新後のイメージURLを追跡する

	buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo := newTestRepositories(
		buildData,
		&models.Deployment{ID: "deployment-id-002", ProjectID: "project-id-002", Name: "my-app"},
		&models.Project{ID: "project-id-002", Name: "my-project", Namespace: namespace},
		&models.HarborCredential{HarborEndpoint: "harbor.example.com"},
	)
	buildRepo.updateBuildResultFn = func(ctx context.Context, buildID string, status models.BuildStatus, builtImageURL string, finishedAt time.Time) error {
		resultStatus = status       // 更新されたステータスを記録する
		resultImageURL = builtImageURL // 更新されたイメージURLを記録する
		return nil
	}

	fakeClient := fake.NewSimpleClientset()                              // フェイク k8s クライアントを生成する
	succeededJob := newTestJobWithStatus(buildID, namespace, 0, 1, 0) // Succeeded 状態の Job を生成する

	event := watch.Event{
		Type:   watch.Modified,    // Modified イベントを設定する
		Object: succeededJob,      // Job オブジェクトを設定する
	}

	handleBuildJobEvent(ctx, event, fakeClient, buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo, "") // イベントを処理する

	if resultStatus != models.BuildStatusSucceeded { // ステータスが succeeded になっていることを確認する
		t.Errorf("Job Succeeded 後のステータスは succeeded であるべきですが、%s でした", resultStatus)
	}

	expectedImageURL := "/project-id-002/deployment-id-002:" + buildID // 期待するイメージURLを組み立てる（registryHost="" のため先頭スラッシュ）
	if resultImageURL != expectedImageURL {                              // イメージURLが正しいことを確認する
		t.Errorf("BuiltImageURL: 期待=%s, 実際=%s", expectedImageURL, resultImageURL)
	}
}

// TestHandleBuildJobEvent_SucceededJob_PendingImageURLUpdated はビルド成功時に Deployment の pending_image_url が更新されることを確認する
func TestHandleBuildJobEvent_SucceededJob_PendingImageURLUpdated(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	buildID := "build-id-003"       // テスト用ビルドID
	namespace := "test-namespace"   // テスト用 namespace

	buildData := &models.DeploymentBuild{
		ID:           buildID,                     // ビルドIDを設定する
		DeploymentID: "deployment-id-003",         // デプロイメントIDを設定する
		Status:       models.BuildStatusBuilding,  // building 状態を設定する
		BuildType:    models.BuildTypeRailpack,    // ビルドタイプを設定する
	}

	var updatedPendingImageURL string // 更新後の pending_image_url を追跡する

	buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo := newTestRepositories(
		buildData,
		&models.Deployment{ID: "deployment-id-003", ProjectID: "project-id-003", Name: "my-app"},
		&models.Project{ID: "project-id-003", Name: "my-project", Namespace: namespace},
		&models.HarborCredential{HarborEndpoint: "harbor.example.com"},
	)
	deploymentRepo.updatePendingImageURLFn = func(ctx context.Context, deploymentID string, imageURL string) error {
		updatedPendingImageURL = imageURL // 更新された pending_image_url を記録する
		return nil
	}

	fakeClient := fake.NewSimpleClientset()                              // フェイク k8s クライアントを生成する
	succeededJob := newTestJobWithStatus(buildID, namespace, 0, 1, 0) // Succeeded 状態の Job を生成する

	event := watch.Event{
		Type:   watch.Modified,    // Modified イベントを設定する
		Object: succeededJob,      // Job オブジェクトを設定する
	}

	handleBuildJobEvent(ctx, event, fakeClient, buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo, "") // イベントを処理する

	expectedImageURL := "/project-id-003/deployment-id-003:" + buildID // 期待するイメージURLを組み立てる（registryHost="" のため先頭スラッシュ）
	if updatedPendingImageURL != expectedImageURL {                      // pending_image_url が正しく更新されていることを確認する
		t.Errorf("pending_image_url: 期待=%s, 実際=%s", expectedImageURL, updatedPendingImageURL)
	}
}

// TestHandleBuildJobEvent_FailedJob_StatusBecomesFailed は Job が Failed になると status が failed になることを確認する
func TestHandleBuildJobEvent_FailedJob_StatusBecomesFailed(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	buildID := "build-id-004"       // テスト用ビルドID
	namespace := "test-namespace"   // テスト用 namespace

	buildData := &models.DeploymentBuild{
		ID:           buildID,                     // ビルドIDを設定する
		DeploymentID: "deployment-id-004",         // デプロイメントIDを設定する
		Status:       models.BuildStatusBuilding,  // building 状態を設定する
		BuildType:    models.BuildTypeRailpack,    // ビルドタイプを設定する
	}

	var resultStatus models.BuildStatus // 更新後のステータスを追跡する

	buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo := newTestRepositories(
		buildData,
		&models.Deployment{ID: "deployment-id-004", ProjectID: "project-id-004", Name: "my-app"},
		&models.Project{ID: "project-id-004", Name: "my-project", Namespace: namespace},
		&models.HarborCredential{HarborEndpoint: "harbor.example.com"},
	)
	buildRepo.updateBuildResultFn = func(ctx context.Context, buildID string, status models.BuildStatus, builtImageURL string, finishedAt time.Time) error {
		resultStatus = status // 更新されたステータスを記録する
		return nil
	}

	fakeClient := fake.NewSimpleClientset()                           // フェイク k8s クライアントを生成する
	failedJob := newTestJobWithStatus(buildID, namespace, 0, 0, 1) // Failed 状態の Job を生成する

	event := watch.Event{
		Type:   watch.Modified,    // Modified イベントを設定する
		Object: failedJob,         // Job オブジェクトを設定する
	}

	handleBuildJobEvent(ctx, event, fakeClient, buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo, "") // イベントを処理する

	if resultStatus != models.BuildStatusFailed { // ステータスが failed になっていることを確認する
		t.Errorf("Job Failed 後のステータスは failed であるべきですが、%s でした", resultStatus)
	}
}

// TestWatchBuildJobs_RecoversInProgressBuilds は Watcher 起動時に building 状態のビルドが検出されることを確認する
func TestWatchBuildJobs_RecoversInProgressBuilds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background()) // キャンセル可能なコンテキストを生成する

	buildID := "build-id-005"       // テスト用ビルドID
	namespace := "test-namespace"   // テスト用 namespace

	buildData := models.DeploymentBuild{
		ID:           buildID,                     // ビルドIDを設定する
		DeploymentID: "deployment-id-005",         // デプロイメントIDを設定する
		Status:       models.BuildStatusBuilding,  // building 状態を設定する
		BuildType:    models.BuildTypeDockerfile,  // Dockerfile ビルドは未実装のためスキップされる
	}

	findAllBuildingCalled := false // FindAllBuilding が呼ばれたかを追跡する

	buildRepo := &mockBuildRepo{
		findByIDFn: func(ctx context.Context, id string) (*models.DeploymentBuild, error) {
			return &buildData, nil // テスト用ビルドレコードを返す
		},
		findAllBuildingFn: func(ctx context.Context) ([]models.DeploymentBuild, error) {
			findAllBuildingCalled = true    // 呼び出しを記録する
			cancel()                        // リカバリ確認後すぐにキャンセルする
			return []models.DeploymentBuild{buildData}, nil // building 状態のビルドを返す
		},
	}
	logChunkRepo := &mockLogChunkRepo{}
	deploymentRepo := &mockDeploymentRepoForBuild{
		findByIDFn: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: "deployment-id-005", ProjectID: "project-id-005", Name: "my-app"}, nil // テスト用 deployment を返す
		},
	}
	projectRepo := &mockProjectRepoForBuild{
		findByIDNoTxFn: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: "project-id-005", Name: "my-project", Namespace: namespace}, nil // テスト用 project を返す
		},
	}
	harborCredentialRepo := &mockHarborCredentialRepoForBuild{}

	fakeClient := fake.NewSimpleClientset() // フェイク k8s クライアントを生成する
	fakeWatcher := watch.NewFake()          // フェイク Watcher を生成する
	fakeClient.PrependWatchReactor("jobs", func(action k8stesting.Action) (bool, watch.Interface, error) {
		return true, fakeWatcher, nil // フェイク Watcher を返す
	})

	go WatchBuildJobs(ctx, fakeClient, buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo, "") // Watcher をバックグラウンドで起動する

	// リカバリが実行されるまで少し待機する
	waitDeadline := time.After(2 * time.Second) // 2秒のタイムアウトを設定する
	for !findAllBuildingCalled {
		select {
		case <-waitDeadline: // タイムアウトした場合はテスト失敗
			t.Fatal("WatchBuildJobs 起動時に FindAllBuilding が呼ばれませんでした")
		default:
			time.Sleep(10 * time.Millisecond) // 10ms 待機して再確認する
		}
	}

	if !findAllBuildingCalled { // FindAllBuilding が呼ばれていない場合はテスト失敗
		t.Error("WatchBuildJobs 起動時に building 状態のビルドを検索する FindAllBuilding が呼ばれるべきです")
	}
}

// 未使用インポートを防ぐためのコンパイル確認用変数
var _ repository.DeploymentBuildRepository = (*mockBuildRepo)(nil)
var _ repository.BuildLogChunkRepository = (*mockLogChunkRepo)(nil)
var _ repository.DeploymentRepository = (*mockDeploymentRepoForBuild)(nil)
var _ repository.ProjectRepository = (*mockProjectRepoForBuild)(nil)
var _ repository.HarborCredentialRepository = (*mockHarborCredentialRepoForBuild)(nil)
