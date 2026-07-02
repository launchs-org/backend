package service

import (
	"handler/models"
	"context"
	"testing"

	"gorm.io/gorm"
)

// mockWebhookRepository は WebhookRepository のテスト用モック実装
type mockWebhookRepository struct {
	createFunc             func(ctx context.Context, webhookData *models.DeploymentWebhook) error
	findByDeploymentIDFunc func(ctx context.Context, deploymentID string) (*models.DeploymentWebhook, error)
	findByIDFunc           func(ctx context.Context, webhookID string) (*models.DeploymentWebhook, error)
	deleteFunc             func(ctx context.Context, webhookID string) error
}

func (mock *mockWebhookRepository) Create(ctx context.Context, webhookData *models.DeploymentWebhook) error {
	return mock.createFunc(ctx, webhookData) // モック関数を呼び出す
}

func (mock *mockWebhookRepository) FindByDeploymentID(ctx context.Context, deploymentID string) (*models.DeploymentWebhook, error) {
	if mock.findByDeploymentIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByDeploymentIDFunc(ctx, deploymentID)
	}
	return &models.DeploymentWebhook{DeploymentID: deploymentID}, nil // デフォルトは webhook を返す
}

func (mock *mockWebhookRepository) FindByID(ctx context.Context, webhookID string) (*models.DeploymentWebhook, error) {
	if mock.findByIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDFunc(ctx, webhookID)
	}
	return &models.DeploymentWebhook{ID: webhookID, DeploymentID: "dep-id-1"}, nil // デフォルトは webhook を返す
}

func (mock *mockWebhookRepository) Delete(ctx context.Context, webhookID string) error {
	if mock.deleteFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.deleteFunc(ctx, webhookID)
	}
	return nil // デフォルトは nil を返す
}

// mockApplyService は ApplyServiceInterface のテスト用モック実装
type mockApplyService struct {
	applyFunc func(ctx context.Context, userID string, deploymentID string) (*ApplyResult, error)
}

func (mock *mockApplyService) Apply(ctx context.Context, userID string, deploymentID string) (*ApplyResult, error) {
	if mock.applyFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.applyFunc(ctx, userID, deploymentID)
	}
	return &ApplyResult{}, nil // デフォルトは正常終了を返す
}

func (mock *mockApplyService) ApplyProject(ctx context.Context, userID string, projectID string) error {
	return nil // テストでは使用しないためデフォルト実装を返す
}

func (mock *mockApplyService) ListApplyHistories(ctx context.Context, userID string, deploymentID string) ([]*models.ApplyHistory, error) {
	return nil, nil // テストでは使用しないためデフォルト nil を返す
}

// TestCreateWebhook_正常にwebhookが作成される は CreateWebhook が webhook を作成することを確認する
func TestCreateWebhook_正常にwebhookが作成される_service(t *testing.T) {
	var capturedWebhook *models.DeploymentWebhook // キャプチャした webhook を格納する変数を定義する

	webhookRepo := &mockWebhookRepository{
		createFunc: func(ctx context.Context, webhookData *models.DeploymentWebhook) error {
			capturedWebhook = webhookData   // webhook をキャプチャする
			webhookData.ID = "new-webhook-id" // ID を付与する
			return nil
		},
	}
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	svc := NewWebhookService(webhookRepo, deploymentRepo, projectRepo, nil, nil) // サービスを生成する

	result, err := svc.CreateWebhook(context.Background(), "test-user-id", "dep-id-1", CreateWebhookRequest{
		GithubRepoURL: "https://github.com/org/repo", // GitHub リポジトリ URL を設定する
	})
	if err != nil {
		t.Fatalf("CreateWebhook がエラーを返しました: %v", err)
	}
	if result.ID != "new-webhook-id" { // ID が設定されていることを確認する
		t.Errorf("期待する ID: new-webhook-id, 実際の ID: %s", result.ID)
	}
	if capturedWebhook.Secret == "" { // シークレットが自動生成されていることを確認する
		t.Error("シークレットが自動生成されていません")
	}
	if capturedWebhook.GithubRepoURL != "https://github.com/org/repo" { // GitHub リポジトリ URL が設定されていることを確認する
		t.Errorf("期待する GithubRepoURL: https://github.com/org/repo, 実際の GithubRepoURL: %s", capturedWebhook.GithubRepoURL)
	}
	if !capturedWebhook.IsActive { // IsActive が true であることを確認する
		t.Error("IsActive が true になっていません")
	}
}

// TestCreateWebhook_他ユーザーのDeploymentはErrForbiddenを返す は所有者でないユーザーが ErrForbidden を受け取ることを確認する
func TestCreateWebhook_他ユーザーのDeploymentはErrForbiddenを返す_service(t *testing.T) {
	webhookRepo := &mockWebhookRepository{}
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "owner-user-id"}, nil // 別ユーザーが所有するプロジェクトを返す
		},
	}

	svc := NewWebhookService(webhookRepo, deploymentRepo, projectRepo, nil, nil) // サービスを生成する

	_, err := svc.CreateWebhook(context.Background(), "other-user-id", "dep-id-1", CreateWebhookRequest{}) // 他ユーザーとして作成する
	if err != ErrForbidden { // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestGetWebhook_正常にwebhookが取得される は GetWebhook が webhook を返すことを確認する
func TestGetWebhook_正常にwebhookが取得される_service(t *testing.T) {
	expectedWebhook := &models.DeploymentWebhook{
		ID:            "webhook-id-1",                      // webhook を設定する
		DeploymentID:  "dep-id-1",
		Secret:        "my-secret",
		GithubRepoURL: "https://github.com/org/repo",
		IsActive:      true,
	}
	webhookRepo := &mockWebhookRepository{
		findByDeploymentIDFunc: func(ctx context.Context, deploymentID string) (*models.DeploymentWebhook, error) {
			return expectedWebhook, nil // webhook を返す
		},
	}
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	svc := NewWebhookService(webhookRepo, deploymentRepo, projectRepo, nil, nil) // サービスを生成する

	result, err := svc.GetWebhook(context.Background(), "test-user-id", "dep-id-1") // webhook を取得する
	if err != nil {
		t.Fatalf("GetWebhook がエラーを返しました: %v", err)
	}
	if result.ID != "webhook-id-1" { // ID が一致することを確認する
		t.Errorf("期待する ID: webhook-id-1, 実際の ID: %s", result.ID)
	}
}

// TestGetWebhook_他ユーザーのDeploymentはErrForbiddenを返す は所有者でないユーザーが ErrForbidden を受け取ることを確認する
func TestGetWebhook_他ユーザーのDeploymentはErrForbiddenを返す_service(t *testing.T) {
	webhookRepo := &mockWebhookRepository{}
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "owner-user-id"}, nil // 別ユーザーが所有するプロジェクトを返す
		},
	}

	svc := NewWebhookService(webhookRepo, deploymentRepo, projectRepo, nil, nil) // サービスを生成する

	_, err := svc.GetWebhook(context.Background(), "other-user-id", "dep-id-1") // 他ユーザーとして取得する
	if err != ErrForbidden { // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestDeleteWebhook_正常にwebhookが削除される は DeleteWebhook が webhook を削除することを確認する
func TestDeleteWebhook_正常にwebhookが削除される_service(t *testing.T) {
	var deletedWebhookID string // 削除された webhook ID を格納する変数を定義する

	webhookRepo := &mockWebhookRepository{
		findByIDFunc: func(ctx context.Context, webhookID string) (*models.DeploymentWebhook, error) {
			return &models.DeploymentWebhook{ID: webhookID, DeploymentID: "dep-id-1"}, nil // webhook を返す
		},
		deleteFunc: func(ctx context.Context, webhookID string) error {
			deletedWebhookID = webhookID // 削除された ID をキャプチャする
			return nil
		},
	}
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	svc := NewWebhookService(webhookRepo, deploymentRepo, projectRepo, nil, nil) // サービスを生成する

	err := svc.DeleteWebhook(context.Background(), "test-user-id", "webhook-id-1") // webhook を削除する
	if err != nil {
		t.Fatalf("DeleteWebhook がエラーを返しました: %v", err)
	}
	if deletedWebhookID != "webhook-id-1" { // 正しい ID が削除されたことを確認する
		t.Errorf("期待する削除 ID: webhook-id-1, 実際の削除 ID: %s", deletedWebhookID)
	}
}

// TestDeleteWebhook_他ユーザーのWebhookはErrForbiddenを返す は所有者でないユーザーが ErrForbidden を受け取ることを確認する
func TestDeleteWebhook_他ユーザーのWebhookはErrForbiddenを返す_service(t *testing.T) {
	webhookRepo := &mockWebhookRepository{
		findByIDFunc: func(ctx context.Context, webhookID string) (*models.DeploymentWebhook, error) {
			return &models.DeploymentWebhook{ID: webhookID, DeploymentID: "dep-id-1"}, nil // webhook を返す
		},
	}
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "owner-user-id"}, nil // 別ユーザーが所有するプロジェクトを返す
		},
	}

	svc := NewWebhookService(webhookRepo, deploymentRepo, projectRepo, nil, nil) // サービスを生成する

	err := svc.DeleteWebhook(context.Background(), "other-user-id", "webhook-id-1") // 他ユーザーとして削除する
	if err != ErrForbidden { // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestCreateWebhook_シークレットが毎回異なる は生成されるシークレットが毎回異なることを確認する
func TestCreateWebhook_シークレットが毎回異なる(t *testing.T) {
	secrets := make([]string, 0, 3) // 生成されたシークレットを格納するスライスを定義する

	webhookRepo := &mockWebhookRepository{
		createFunc: func(ctx context.Context, webhookData *models.DeploymentWebhook) error {
			secrets = append(secrets, webhookData.Secret) // シークレットをキャプチャする
			return nil
		},
	}
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{}

	svc := NewWebhookService(webhookRepo, deploymentRepo, projectRepo, nil, nil) // サービスを生成する

	for callIndex := 0; callIndex < 3; callIndex++ { // 3 回作成を実行する
		_, err := svc.CreateWebhook(context.Background(), "test-user-id", "dep-id-1", CreateWebhookRequest{})
		if err != nil {
			t.Fatalf("CreateWebhook がエラーを返しました: %v", err)
		}
	}

	if secrets[0] == secrets[1] || secrets[1] == secrets[2] { // シークレットが毎回異なることを確認する
		t.Error("シークレットが毎回同じ値になっています")
	}
}

// TestCreateWebhook_存在しないDeploymentは404が返る は存在しない deployment に作成すると ErrRecordNotFound が返ることを確認する
func TestCreateWebhook_存在しないDeploymentは404が返る(t *testing.T) {
	webhookRepo := &mockWebhookRepository{}
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return nil, gorm.ErrRecordNotFound // 未発見エラーを返す
		},
	}
	projectRepo := &mockProjectRepository{}

	svc := NewWebhookService(webhookRepo, deploymentRepo, projectRepo, nil, nil) // サービスを生成する

	_, err := svc.CreateWebhook(context.Background(), "test-user-id", "nonexistent-dep", CreateWebhookRequest{}) // 存在しない deployment を指定する
	if err == nil {                                                                                                // エラーが返ることを確認する
		t.Fatal("ErrRecordNotFound が返ることを期待しましたが、エラーが返りませんでした")
	}
}

