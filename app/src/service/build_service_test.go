package service

import (
	"app/models"
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// mockDeploymentBuildRepository は DeploymentBuildRepository のテスト用モック実装
type mockDeploymentBuildRepository struct {
	createFunc                func(ctx context.Context, build *models.DeploymentBuild) error
	findByIDFunc              func(ctx context.Context, buildID string) (*models.DeploymentBuild, error)
	findAllByDeploymentIDFunc func(ctx context.Context, deploymentID string) ([]models.DeploymentBuild, error)
	findAllByProjectIDFunc    func(ctx context.Context, projectID string) ([]models.DeploymentBuild, error)
	updateStatusFunc          func(ctx context.Context, buildID string, status models.BuildStatus) error
	updateK8sJobNameFunc      func(ctx context.Context, buildID string, jobName string) error
}

func (mock *mockDeploymentBuildRepository) Create(ctx context.Context, build *models.DeploymentBuild) error {
	return mock.createFunc(ctx, build) // モック関数を呼び出す
}

func (mock *mockDeploymentBuildRepository) FindByID(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
	if mock.findByIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDFunc(ctx, buildID)
	}
	return nil, nil // デフォルトは nil を返す
}

func (mock *mockDeploymentBuildRepository) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]models.DeploymentBuild, error) {
	return mock.findAllByDeploymentIDFunc(ctx, deploymentID) // モック関数を呼び出す
}

func (mock *mockDeploymentBuildRepository) UpdateStatus(ctx context.Context, buildID string, status models.BuildStatus) error {
	if mock.updateStatusFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updateStatusFunc(ctx, buildID, status)
	}
	return nil // デフォルトは nil を返す
}

func (mock *mockDeploymentBuildRepository) UpdateK8sJobName(ctx context.Context, buildID string, jobName string) error {
	if mock.updateK8sJobNameFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updateK8sJobNameFunc(ctx, buildID, jobName)
	}
	return nil // デフォルトは nil を返す
}

func (mock *mockDeploymentBuildRepository) FindAllBuilding(ctx context.Context) ([]models.DeploymentBuild, error) {
	return nil, nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *mockDeploymentBuildRepository) UpdateBuildResult(ctx context.Context, buildID string, status models.BuildStatus, builtImageURL string, imageSizeBytes int64, finishedAt time.Time) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *mockDeploymentBuildRepository) Delete(ctx context.Context, build *models.DeploymentBuild) error {
	return nil // テストでは使用しない
}

func (mock *mockDeploymentBuildRepository) DeleteAllByDeploymentID(ctx context.Context, deploymentID string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *mockDeploymentBuildRepository) FindAllByProjectID(ctx context.Context, projectID string) ([]models.DeploymentBuild, error) {
	if mock.findAllByProjectIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findAllByProjectIDFunc(ctx, projectID)
	}
	return []models.DeploymentBuild{}, nil // デフォルトは空リストを返す
}

func (mock *mockDeploymentBuildRepository) DeleteAllByProjectID(ctx context.Context, db *gorm.DB, projectID string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

// mockBuildLogChunkRepository は BuildLogChunkRepository のテスト用モック実装
type mockBuildLogChunkRepository struct {
	findByBuildIDFunc      func(ctx context.Context, buildID string) ([]models.BuildLogChunk, error)
	findByBuildIDSinceFunc func(ctx context.Context, buildID string, since time.Time) ([]models.BuildLogChunk, error)
}

func (mock *mockBuildLogChunkRepository) Create(ctx context.Context, chunk *models.BuildLogChunk) error {
	return nil // テストでは使用しない
}

func (mock *mockBuildLogChunkRepository) FindByBuildID(ctx context.Context, buildID string) ([]models.BuildLogChunk, error) {
	if mock.findByBuildIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByBuildIDFunc(ctx, buildID)
	}
	return nil, nil // デフォルトは nil を返す
}

func (mock *mockBuildLogChunkRepository) FindByBuildIDSince(ctx context.Context, buildID string, since time.Time) ([]models.BuildLogChunk, error) {
	if mock.findByBuildIDSinceFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByBuildIDSinceFunc(ctx, buildID, since)
	}
	return nil, nil // デフォルトは nil を返す
}

// mockHarborCredentialRepository は HarborCredentialRepository のテスト用モック実装（build service テスト用）
type mockHarborCredentialRepository struct {
	findByProjectIDNoTxFunc func(ctx context.Context, projectID string) (*models.HarborCredential, error)
}

func (mock *mockHarborCredentialRepository) Create(ctx context.Context, tx *gorm.DB, credential *models.HarborCredential) error {
	return nil // テストでは使用しない
}

func (mock *mockHarborCredentialRepository) FindByProjectID(ctx context.Context, tx *gorm.DB, projectID string) (*models.HarborCredential, error) {
	return nil, nil // テストでは使用しない
}

func (mock *mockHarborCredentialRepository) DeleteByProjectID(ctx context.Context, tx *gorm.DB, projectID string) error {
	return nil // テストでは使用しない
}

func (mock *mockHarborCredentialRepository) FindByProjectIDNoTx(ctx context.Context, projectID string) (*models.HarborCredential, error) {
	return mock.findByProjectIDNoTxFunc(ctx, projectID) // モック関数を呼び出す
}

// TestTriggerBuild_正常系 はビルドが正常にトリガーされることを確認する
func TestTriggerBuild_正常系(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	deploymentData := &models.Deployment{
		ID:                         "deployment-1",             // デプロイメント ID を設定する
		ProjectID:                  "project-1",                // プロジェクト ID を設定する
		Type:                       models.DeploymentTypeRailpack, // railpack タイプを設定する
		Name:                       "my-app",                   // デプロイメント名を設定する
		PendingGithubRepoURL:       "https://github.com/org/repo",
		PendingGithubBranch:        "main",
		PendingGithubCommitSHA:     "abc123",
		PendingGithubRepoDirectory: "./",
	}

	projectData := &models.Project{
		ID:        "project-1",  // プロジェクト ID を設定する
		UserID:    "user-1",     // ユーザー ID を設定する
		Name:      "my-project", // プロジェクト名を設定する
		Namespace: "ns-test",    // namespace を設定する
	}

	harborCredential := &models.HarborCredential{
		ProjectID:      "project-1",             // プロジェクト ID を設定する
		RobotName:      "robot-name",            // robot アカウント名を設定する
		RobotSecret:    "robot-secret",          // robot シークレットを設定する
		HarborEndpoint: "https://harbor.example", // Harbor エンドポイントを設定する
	}

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return deploymentData, nil // テスト用デプロイメントを返す
		},
	}

	buildRepo := &mockDeploymentBuildRepository{
		createFunc: func(ctx context.Context, build *models.DeploymentBuild) error {
			build.ID = "build-id-1" // テスト用ビルド ID を設定する
			return nil              // 作成成功を返す
		},
		findAllByDeploymentIDFunc: func(ctx context.Context, deploymentID string) ([]models.DeploymentBuild, error) {
			return []models.DeploymentBuild{}, nil // 進行中のビルドなしを返す
		},
	}

	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return projectData, nil // テスト用プロジェクトを返す
		},
	}

	harborCredRepo := &mockHarborCredentialRepository{
		findByProjectIDNoTxFunc: func(ctx context.Context, projectID string) (*models.HarborCredential, error) {
			return harborCredential, nil // テスト用 Harbor 認証情報を返す
		},
	}

	k8sClient := fake.NewSimpleClientset() // フェイク k8s クライアントを生成する

	buildSvc := NewBuildService(deploymentRepo, buildRepo, projectRepo, harborCredRepo, &mockBuildLogChunkRepository{}, k8sClient, nil, "") // サービスを生成する

	resultBuild, err := buildSvc.TriggerBuild(ctx, "user-1", "deployment-1") // ビルドをトリガーする
	if err != nil {
		t.Fatalf("TriggerBuild() がエラーを返しました: %v", err)
	}
	if resultBuild == nil { // 結果が nil でないことを確認する
		t.Fatal("TriggerBuild() が nil を返しました")
	}
	if resultBuild.Status != models.BuildStatusPending { // ステータスが pending であることを確認する
		t.Errorf("期待するステータス %s、実際のステータス %s", models.BuildStatusPending, resultBuild.Status)
	}
}

// TestTriggerBuild_403_他ユーザー は他ユーザーのデプロイメントにアクセスすると ErrForbidden を返すことを確認する
func TestTriggerBuild_403_他ユーザー(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	deploymentData := &models.Deployment{
		ID:        "deployment-1",  // デプロイメント ID を設定する
		ProjectID: "project-1",    // プロジェクト ID を設定する
		Type:      models.DeploymentTypeRailpack,
	}

	projectData := &models.Project{
		ID:     "project-1", // プロジェクト ID を設定する
		UserID: "other-user", // 異なるユーザー ID を設定する
	}

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return deploymentData, nil // テスト用デプロイメントを返す
		},
	}

	buildRepo := &mockDeploymentBuildRepository{
		findAllByDeploymentIDFunc: func(ctx context.Context, deploymentID string) ([]models.DeploymentBuild, error) {
			return []models.DeploymentBuild{}, nil // 進行中のビルドなしを返す
		},
	}

	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return projectData, nil // 異なるユーザーのプロジェクトを返す
		},
	}

	harborCredRepo := &mockHarborCredentialRepository{}     // harbor credential リポジトリのモックを生成する
	k8sClient := fake.NewSimpleClientset()                  // フェイク k8s クライアントを生成する

	buildSvc := NewBuildService(deploymentRepo, buildRepo, projectRepo, harborCredRepo, &mockBuildLogChunkRepository{}, k8sClient, nil, "") // サービスを生成する

	_, err := buildSvc.TriggerBuild(ctx, "user-1", "deployment-1") // ビルドをトリガーする
	if err == nil {                                                  // エラーが返ることを確認する
		t.Fatal("TriggerBuild() はエラーを返すべきです")
	}
	if err != ErrForbidden { // ErrForbidden が返ることを確認する
		t.Errorf("期待するエラー %v、実際のエラー %v", ErrForbidden, err)
	}
}

// TestTriggerBuild_409_ビルド中 はビルド中の場合に ErrBuildConflict を返すことを確認する
func TestTriggerBuild_409_ビルド中(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	deploymentData := &models.Deployment{
		ID:        "deployment-1",  // デプロイメント ID を設定する
		ProjectID: "project-1",    // プロジェクト ID を設定する
		Type:      models.DeploymentTypeRailpack,
	}

	projectData := &models.Project{
		ID:     "project-1", // プロジェクト ID を設定する
		UserID: "user-1",    // ユーザー ID を設定する
	}

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return deploymentData, nil // テスト用デプロイメントを返す
		},
	}

	buildRepo := &mockDeploymentBuildRepository{
		findAllByDeploymentIDFunc: func(ctx context.Context, deploymentID string) ([]models.DeploymentBuild, error) {
			return []models.DeploymentBuild{ // ビルド中のレコードを返す
				{ID: "existing-build", Status: models.BuildStatusBuilding},
			}, nil
		},
	}

	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return projectData, nil // テスト用プロジェクトを返す
		},
	}

	harborCredRepo := &mockHarborCredentialRepository{}     // harbor credential リポジトリのモックを生成する
	k8sClient := fake.NewSimpleClientset()                  // フェイク k8s クライアントを生成する

	buildSvc := NewBuildService(deploymentRepo, buildRepo, projectRepo, harborCredRepo, &mockBuildLogChunkRepository{}, k8sClient, nil, "") // サービスを生成する

	_, err := buildSvc.TriggerBuild(ctx, "user-1", "deployment-1") // ビルドをトリガーする
	if err == nil {                                                  // エラーが返ることを確認する
		t.Fatal("TriggerBuild() はエラーを返すべきです")
	}
	if err != ErrBuildConflict { // ErrBuildConflict が返ることを確認する
		t.Errorf("期待するエラー %v、実際のエラー %v", ErrBuildConflict, err)
	}
}

// TestCancelBuild_正常系 はビルドが正常にキャンセルされることを確認する
func TestCancelBuild_正常系(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	buildData := &models.DeploymentBuild{
		ID:           "build-1",      // ビルド ID を設定する
		DeploymentID: func() *string { deploymentIDValue := "deployment-1"; return &deploymentIDValue }(), // デプロイメント ID を設定する
		Status:       models.BuildStatusBuilding, // building 状態を設定する
		K8sJobName:   "railpack-build-1",         // Job 名を設定する
		BuildType:    models.BuildTypeRailpack,   // ビルドタイプを設定する
	}

	deploymentData := &models.Deployment{
		ID:        "deployment-1", // デプロイメント ID を設定する
		ProjectID: "project-1",   // プロジェクト ID を設定する
	}

	projectData := &models.Project{
		ID:        "project-1",    // プロジェクト ID を設定する
		UserID:    "user-1",       // ユーザー ID を設定する
		Namespace: "ns-test",      // namespace を設定する
	}

	cancelledStatus := models.BuildStatusCancelled // キャンセル後に期待するステータス
	updatedStatus := models.BuildStatus("")        // UpdateStatus に渡されたステータスを記録する

	buildRepo := &mockDeploymentBuildRepository{
		findByIDFunc: func(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
			return buildData, nil // テスト用ビルドレコードを返す
		},
		updateStatusFunc: func(ctx context.Context, buildID string, status models.BuildStatus) error {
			updatedStatus = status // 更新されたステータスを記録する
			return nil             // 更新成功を返す
		},
	}

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return deploymentData, nil // テスト用デプロイメントを返す
		},
	}

	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return projectData, nil // テスト用プロジェクトを返す
		},
	}

	harborCredRepo := &mockHarborCredentialRepository{
		findByProjectIDNoTxFunc: func(ctx context.Context, projectID string) (*models.HarborCredential, error) {
			return nil, nil // テストでは使用しない
		},
	}

	existingJob := &batchv1.Job{ // テスト用 k8s Job を定義する
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildData.K8sJobName, // Job 名を設定する
			Namespace: "buildkit",           // ビルド専用 namespace を設定する（CancelBuild は buildkit namespace を使う）
		},
	}
	k8sClient := fake.NewSimpleClientset(existingJob) // フェイク k8s クライアントに Job を登録する

	buildSvc := NewBuildService(deploymentRepo, buildRepo, projectRepo, harborCredRepo, &mockBuildLogChunkRepository{}, k8sClient, nil, "") // サービスを生成する

	err := buildSvc.CancelBuild(ctx, "user-1", "build-1") // ビルドをキャンセルする
	if err != nil {                                        // エラーが返った場合はテスト失敗
		t.Fatalf("CancelBuild() が予期しないエラーを返しました: %v", err)
	}
	if updatedStatus != cancelledStatus { // ステータスが cancelled でない場合はテスト失敗
		t.Errorf("期待するステータス %s、実際のステータス %s", cancelledStatus, updatedStatus)
	}
}

// TestCancelBuild_完了済みビルドはキャンセル不可 は completed のビルドに対して ErrBuildNotCancellable を返すことを確認する
func TestCancelBuild_完了済みビルドはキャンセル不可(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	buildData := &models.DeploymentBuild{
		ID:           "build-1",      // ビルド ID を設定する
		DeploymentID: func() *string { deploymentIDValue := "deployment-1"; return &deploymentIDValue }(), // デプロイメント ID を設定する
		Status:       models.BuildStatusSucceeded, // 完了済み状態を設定する
	}

	buildRepo := &mockDeploymentBuildRepository{
		findByIDFunc: func(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
			return buildData, nil // テスト用ビルドレコードを返す
		},
	}

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: "deployment-1", ProjectID: "project-1"}, nil // テスト用デプロイメントを返す
		},
	}

	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: "project-1", UserID: "user-1", Namespace: "ns-test"}, nil // テスト用プロジェクトを返す
		},
	}

	harborCredRepo := &mockHarborCredentialRepository{
		findByProjectIDNoTxFunc: func(ctx context.Context, projectID string) (*models.HarborCredential, error) {
			return nil, nil // テストでは使用しない
		},
	}

	k8sClient := fake.NewSimpleClientset() // フェイク k8s クライアントを生成する

	buildSvc := NewBuildService(deploymentRepo, buildRepo, projectRepo, harborCredRepo, &mockBuildLogChunkRepository{}, k8sClient, nil, "") // サービスを生成する

	err := buildSvc.CancelBuild(ctx, "user-1", "build-1") // 完了済みビルドをキャンセルする
	if err != ErrBuildNotCancellable {                     // ErrBuildNotCancellable が返ることを確認する
		t.Errorf("期待するエラー %v、実際のエラー %v", ErrBuildNotCancellable, err)
	}
}

// TestCancelBuild_403_他ユーザー は他ユーザーのビルドをキャンセルすると ErrForbidden を返すことを確認する
func TestCancelBuild_403_他ユーザー(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	buildData := &models.DeploymentBuild{
		ID:           "build-1",      // ビルド ID を設定する
		DeploymentID: func() *string { deploymentIDValue := "deployment-1"; return &deploymentIDValue }(), // デプロイメント ID を設定する
		Status:       models.BuildStatusBuilding, // building 状態を設定する
	}

	buildRepo := &mockDeploymentBuildRepository{
		findByIDFunc: func(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
			return buildData, nil // テスト用ビルドレコードを返す
		},
	}

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: "deployment-1", ProjectID: "project-1"}, nil // テスト用デプロイメントを返す
		},
	}

	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: "project-1", UserID: "other-user", Namespace: "ns-test"}, nil // 別ユーザーのプロジェクトを返す
		},
	}

	harborCredRepo := &mockHarborCredentialRepository{
		findByProjectIDNoTxFunc: func(ctx context.Context, projectID string) (*models.HarborCredential, error) {
			return nil, nil // テストでは使用しない
		},
	}

	k8sClient := fake.NewSimpleClientset() // フェイク k8s クライアントを生成する

	buildSvc := NewBuildService(deploymentRepo, buildRepo, projectRepo, harborCredRepo, &mockBuildLogChunkRepository{}, k8sClient, nil, "") // サービスを生成する

	err := buildSvc.CancelBuild(ctx, "user-1", "build-1") // 他ユーザーのビルドをキャンセルする
	if err != ErrForbidden {                               // ErrForbidden が返ることを確認する
		t.Errorf("期待するエラー %v、実際のエラー %v", ErrForbidden, err)
	}
}

// TestGetBuildLogs_正常系 はビルドログが正常に取得できることを確認する
func TestGetBuildLogs_正常系(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	buildData := &models.DeploymentBuild{
		ID:           "build-1",      // ビルド ID を設定する
		DeploymentID: func() *string { deploymentIDValue := "deployment-1"; return &deploymentIDValue }(), // デプロイメント ID を設定する
	}

	buildRepo := &mockDeploymentBuildRepository{
		findByIDFunc: func(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
			return buildData, nil // テスト用ビルドレコードを返す
		},
	}

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: "deployment-1", ProjectID: "project-1"}, nil // テスト用デプロイメントを返す
		},
	}

	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: "project-1", UserID: "user-1"}, nil // テスト用プロジェクトを返す
		},
	}

	logChunkRepo := &mockBuildLogChunkRepository{
		findByBuildIDFunc: func(ctx context.Context, buildID string) ([]models.BuildLogChunk, error) {
			return []models.BuildLogChunk{ // テスト用ログチャンクを返す
				{BuildID: "build-1", Content: "line1\n"},
				{BuildID: "build-1", Content: "line2\n"},
			}, nil
		},
	}

	k8sClient := fake.NewSimpleClientset() // フェイク k8s クライアントを生成する

	buildSvc := NewBuildService(deploymentRepo, buildRepo, projectRepo, &mockHarborCredentialRepository{}, logChunkRepo, k8sClient, nil, "") // サービスを生成する

	logs, _, err := buildSvc.GetBuildLogs(ctx, "user-1", "build-1", nil) // ログを取得する
	if err != nil {                                                        // エラーが返った場合はテスト失敗
		t.Fatalf("GetBuildLogs() が予期しないエラーを返しました: %v", err)
	}
	expectedLogs := "line1\nline2\n" // 期待するログ文字列を設定する
	if logs != expectedLogs {        // ログ文字列を確認する
		t.Errorf("期待するログ %q、実際のログ %q", expectedLogs, logs)
	}
}

// TestGetBuildLogs_since指定 は since パラメータでログがフィルタされることを確認する
func TestGetBuildLogs_since指定(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	buildData := &models.DeploymentBuild{
		ID:           "build-1",      // ビルド ID を設定する
		DeploymentID: func() *string { deploymentIDValue := "deployment-1"; return &deploymentIDValue }(), // デプロイメント ID を設定する
	}

	sinceTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) // テスト用 since 時刻を設定する
	capturedSince := time.Time{}                               // FindByBuildIDSince に渡された since を記録する

	buildRepo := &mockDeploymentBuildRepository{
		findByIDFunc: func(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
			return buildData, nil // テスト用ビルドレコードを返す
		},
	}

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: "deployment-1", ProjectID: "project-1"}, nil // テスト用デプロイメントを返す
		},
	}

	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: "project-1", UserID: "user-1"}, nil // テスト用プロジェクトを返す
		},
	}

	logChunkRepo := &mockBuildLogChunkRepository{
		findByBuildIDSinceFunc: func(ctx context.Context, buildID string, since time.Time) ([]models.BuildLogChunk, error) {
			capturedSince = since // since を記録する
			return []models.BuildLogChunk{
				{BuildID: "build-1", Content: "line3\n"}, // since より後のチャンクのみ返す
			}, nil
		},
	}

	k8sClient := fake.NewSimpleClientset() // フェイク k8s クライアントを生成する

	buildSvc := NewBuildService(deploymentRepo, buildRepo, projectRepo, &mockHarborCredentialRepository{}, logChunkRepo, k8sClient, nil, "") // サービスを生成する

	logs, _, err := buildSvc.GetBuildLogs(ctx, "user-1", "build-1", &sinceTime) // since を指定してログを取得する
	if err != nil {                                                              // エラーが返った場合はテスト失敗
		t.Fatalf("GetBuildLogs() が予期しないエラーを返しました: %v", err)
	}
	if logs != "line3\n" { // since 以降のログのみ返ることを確認する
		t.Errorf("期待するログ %q、実際のログ %q", "line3\n", logs)
	}
	if !capturedSince.Equal(sinceTime) { // since が正しく渡されたことを確認する
		t.Errorf("期待する since %v、実際の since %v", sinceTime, capturedSince)
	}
}

// TestGetBuildLogs_チャンクなし はログチャンクが存在しない場合に空文字列が返ることを確認する
func TestGetBuildLogs_チャンクなし(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	buildData := &models.DeploymentBuild{
		ID:           "build-1",      // ビルド ID を設定する
		DeploymentID: func() *string { deploymentIDValue := "deployment-1"; return &deploymentIDValue }(), // デプロイメント ID を設定する
	}

	buildRepo := &mockDeploymentBuildRepository{
		findByIDFunc: func(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
			return buildData, nil // テスト用ビルドレコードを返す
		},
	}

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: "deployment-1", ProjectID: "project-1"}, nil // テスト用デプロイメントを返す
		},
	}

	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: "project-1", UserID: "user-1"}, nil // テスト用プロジェクトを返す
		},
	}

	logChunkRepo := &mockBuildLogChunkRepository{
		findByBuildIDFunc: func(ctx context.Context, buildID string) ([]models.BuildLogChunk, error) {
			return []models.BuildLogChunk{}, nil // チャンクなしを返す
		},
	}

	k8sClient := fake.NewSimpleClientset() // フェイク k8s クライアントを生成する

	buildSvc := NewBuildService(deploymentRepo, buildRepo, projectRepo, &mockHarborCredentialRepository{}, logChunkRepo, k8sClient, nil, "") // サービスを生成する

	logs, _, err := buildSvc.GetBuildLogs(ctx, "user-1", "build-1", nil) // ログを取得する
	if err != nil {                                                       // エラーが返った場合はテスト失敗
		t.Fatalf("GetBuildLogs() が予期しないエラーを返しました: %v", err)
	}
	if logs != "" { // 空文字列が返ることを確認する
		t.Errorf("期待するログ %q、実際のログ %q", "", logs)
	}
}

// TestGetBuildLogs_403_他ユーザー は他ユーザーのビルドログ取得で ErrForbidden を返すことを確認する
func TestGetBuildLogs_403_他ユーザー(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	buildData := &models.DeploymentBuild{
		ID:           "build-1",      // ビルド ID を設定する
		DeploymentID: func() *string { deploymentIDValue := "deployment-1"; return &deploymentIDValue }(), // デプロイメント ID を設定する
	}

	buildRepo := &mockDeploymentBuildRepository{
		findByIDFunc: func(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
			return buildData, nil // テスト用ビルドレコードを返す
		},
	}

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: "deployment-1", ProjectID: "project-1"}, nil // テスト用デプロイメントを返す
		},
	}

	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: "project-1", UserID: "other-user"}, nil // 別ユーザーのプロジェクトを返す
		},
	}

	k8sClient := fake.NewSimpleClientset() // フェイク k8s クライアントを生成する

	buildSvc := NewBuildService(deploymentRepo, buildRepo, projectRepo, &mockHarborCredentialRepository{}, &mockBuildLogChunkRepository{}, k8sClient, nil, "") // サービスを生成する

	_, _, err := buildSvc.GetBuildLogs(ctx, "user-1", "build-1", nil) // 他ユーザーのログを取得しようとする
	if err != ErrForbidden {                                          // ErrForbidden が返ることを確認する
		t.Errorf("期待するエラー %v、実際のエラー %v", ErrForbidden, err)
	}
}

// TestListBuildsByProject_正常系 はプロジェクト単位のビルド一覧が正常に取得できることを確認する
func TestListBuildsByProject_正常系(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	projectData := &models.Project{
		ID:     "project-1", // プロジェクト ID を設定する
		UserID: "user-1",    // ユーザー ID を設定する
	}

	expectedBuilds := []models.DeploymentBuild{
		{ID: "build-1", ProjectID: "project-1", Status: models.BuildStatusSucceeded}, // 成功済みビルドを設定する
		{ID: "build-2", ProjectID: "project-1", Status: models.BuildStatusFailed},    // 失敗ビルドを設定する
	}

	buildRepo := &mockDeploymentBuildRepository{
		findAllByProjectIDFunc: func(ctx context.Context, projectID string) ([]models.DeploymentBuild, error) {
			return expectedBuilds, nil // テスト用ビルド一覧を返す
		},
	}

	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return projectData, nil // テスト用プロジェクトを返す
		},
	}

	buildSvc := NewBuildService(&mockDeploymentRepository{}, buildRepo, projectRepo, &mockHarborCredentialRepository{findByProjectIDNoTxFunc: func(ctx context.Context, projectID string) (*models.HarborCredential, error) { return nil, nil }}, &mockBuildLogChunkRepository{}, fake.NewSimpleClientset(), nil, "") // サービスを生成する

	builds, err := buildSvc.ListBuildsByProject(ctx, "user-1", "project-1") // ビルド一覧を取得する
	if err != nil {                                                           // エラーが返った場合はテスト失敗
		t.Fatalf("ListBuildsByProject() が予期しないエラーを返しました: %v", err)
	}
	if len(builds) != 2 { // ビルド件数を確認する
		t.Errorf("期待するビルド件数 2、実際の件数 %d", len(builds))
	}
	if builds[0].ID != "build-1" { // 最初のビルド ID を確認する
		t.Errorf("期待するビルド ID %s、実際の ID %s", "build-1", builds[0].ID)
	}
}

// TestListBuildsByProject_403_他ユーザー は他ユーザーのプロジェクトのビルド一覧取得で ErrForbidden を返すことを確認する
func TestListBuildsByProject_403_他ユーザー(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	projectData := &models.Project{
		ID:     "project-1", // プロジェクト ID を設定する
		UserID: "other-user", // 別ユーザーのプロジェクトを設定する
	}

	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return projectData, nil // テスト用プロジェクトを返す
		},
	}

	buildSvc := NewBuildService(&mockDeploymentRepository{}, &mockDeploymentBuildRepository{}, projectRepo, &mockHarborCredentialRepository{findByProjectIDNoTxFunc: func(ctx context.Context, projectID string) (*models.HarborCredential, error) { return nil, nil }}, &mockBuildLogChunkRepository{}, fake.NewSimpleClientset(), nil, "") // サービスを生成する

	_, err := buildSvc.ListBuildsByProject(ctx, "user-1", "project-1") // 他ユーザーのプロジェクトのビルドを取得しようとする
	if err != ErrForbidden {                                            // ErrForbidden が返ることを確認する
		t.Errorf("期待するエラー %v、実際のエラー %v", ErrForbidden, err)
	}
}
