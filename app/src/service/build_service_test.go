package service

import (
	"app/models"
	"context"
	"testing"

	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes/fake"
)

// mockDeploymentBuildRepository は DeploymentBuildRepository のテスト用モック実装
type mockDeploymentBuildRepository struct {
	createFunc                func(ctx context.Context, build *models.DeploymentBuild) error
	findByIDFunc              func(ctx context.Context, buildID string) (*models.DeploymentBuild, error)
	findAllByDeploymentIDFunc func(ctx context.Context, deploymentID string) ([]models.DeploymentBuild, error)
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

	buildSvc := NewBuildService(deploymentRepo, buildRepo, projectRepo, harborCredRepo, k8sClient) // サービスを生成する

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
		Type:      models.DeploymentTypeDockerfile,
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

	buildSvc := NewBuildService(deploymentRepo, buildRepo, projectRepo, harborCredRepo, k8sClient) // サービスを生成する

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
		Type:      models.DeploymentTypeDockerfile,
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

	buildSvc := NewBuildService(deploymentRepo, buildRepo, projectRepo, harborCredRepo, k8sClient) // サービスを生成する

	_, err := buildSvc.TriggerBuild(ctx, "user-1", "deployment-1") // ビルドをトリガーする
	if err == nil {                                                  // エラーが返ることを確認する
		t.Fatal("TriggerBuild() はエラーを返すべきです")
	}
	if err != ErrBuildConflict { // ErrBuildConflict が返ることを確認する
		t.Errorf("期待するエラー %v、実際のエラー %v", ErrBuildConflict, err)
	}
}
