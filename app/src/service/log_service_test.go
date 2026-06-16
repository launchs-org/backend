package service

import (
	"app/models"
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
)

// mockPodLogChunkRepository は PodLogChunkRepository のテスト用モック実装
type mockPodLogChunkRepository struct {
	findByDeploymentIDFunc      func(ctx context.Context, deploymentID string) ([]models.PodLogChunk, error)
	findByDeploymentIDSinceFunc func(ctx context.Context, deploymentID string, since time.Time) ([]models.PodLogChunk, error)
}

func (mock *mockPodLogChunkRepository) Create(ctx context.Context, chunk *models.PodLogChunk) error {
	return nil // テストでは使用しない
}

func (mock *mockPodLogChunkRepository) FindByDeploymentID(ctx context.Context, deploymentID string) ([]models.PodLogChunk, error) {
	if mock.findByDeploymentIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByDeploymentIDFunc(ctx, deploymentID)
	}
	return nil, nil // デフォルトは nil を返す
}

func (mock *mockPodLogChunkRepository) FindByDeploymentIDSince(ctx context.Context, deploymentID string, since time.Time) ([]models.PodLogChunk, error) {
	if mock.findByDeploymentIDSinceFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByDeploymentIDSinceFunc(ctx, deploymentID, since)
	}
	return nil, nil // デフォルトは nil を返す
}

// TestGetPodLogs_チャンクなし は Pod ログチャンクが0件の場合に空文字列と nil timestamp が返ることを確認する
func TestGetPodLogs_チャンクなし(t *testing.T) {
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-1"}, nil // テスト用 Deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}
	podLogChunkRepo := &mockPodLogChunkRepository{
		findByDeploymentIDFunc: func(ctx context.Context, deploymentID string) ([]models.PodLogChunk, error) {
			return []models.PodLogChunk{}, nil // 空チャンクを返す
		},
	}

	logSvc := NewLogService(deploymentRepo, projectRepo, podLogChunkRepo) // サービスを生成する
	result, err := logSvc.GetPodLogs(context.Background(), "test-user-id", "deployment-1", nil)
	if err != nil { // エラーが発生した場合は失敗する
		t.Fatalf("GetPodLogs() がエラーを返しました: %v", err)
	}
	if result.Logs != "" { // ログが空でない場合は失敗する
		t.Errorf("空ログを期待しますが、実際は %q です", result.Logs)
	}
	if result.LastTimestamp != nil { // LastTimestamp が nil でない場合は失敗する
		t.Errorf("LastTimestamp が nil ではありません: %v", result.LastTimestamp)
	}
}

// TestGetPodLogs_チャンクあり は Pod ログチャンクが結合されて返ることを確認する
func TestGetPodLogs_チャンクあり(t *testing.T) {
	expectedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) // 期待する最後の CreatedAt を設定する

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-1"}, nil // テスト用 Deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}
	podLogChunkRepo := &mockPodLogChunkRepository{
		findByDeploymentIDFunc: func(ctx context.Context, deploymentID string) ([]models.PodLogChunk, error) {
			return []models.PodLogChunk{
				{ID: "chunk-1", DeploymentID: deploymentID, Content: "line1\n", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}, // 1件目のチャンクを返す
				{ID: "chunk-2", DeploymentID: deploymentID, Content: "line2\n", CreatedAt: expectedTime},                               // 2件目のチャンクを返す
			}, nil
		},
	}

	logSvc := NewLogService(deploymentRepo, projectRepo, podLogChunkRepo) // サービスを生成する
	result, err := logSvc.GetPodLogs(context.Background(), "test-user-id", "deployment-1", nil)
	if err != nil { // エラーが発生した場合は失敗する
		t.Fatalf("GetPodLogs() がエラーを返しました: %v", err)
	}
	if result.Logs != "line1\nline2\n" { // ログが結合されていない場合は失敗する
		t.Errorf("期待するログ %q、実際のログ %q", "line1\nline2\n", result.Logs)
	}
	if result.LastTimestamp == nil { // LastTimestamp が nil の場合は失敗する
		t.Fatal("LastTimestamp が nil です")
	}
	if !result.LastTimestamp.Equal(expectedTime) { // LastTimestamp が一致しない場合は失敗する
		t.Errorf("期待する LastTimestamp %v、実際の LastTimestamp %v", expectedTime, *result.LastTimestamp)
	}
}

// TestGetPodLogs_他ユーザーのDeployment は他ユーザーの Deployment で ErrForbidden が返ることを確認する
func TestGetPodLogs_他ユーザーのDeployment(t *testing.T) {
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-1"}, nil // テスト用 Deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "other-user-id"}, nil // 他ユーザーとして返す
		},
	}
	podLogChunkRepo := &mockPodLogChunkRepository{}

	logSvc := NewLogService(deploymentRepo, projectRepo, podLogChunkRepo)                               // サービスを生成する
	_, err := logSvc.GetPodLogs(context.Background(), "test-user-id", "deployment-1", nil)
	if err == nil { // エラーが発生しない場合は失敗する
		t.Fatal("ErrForbidden を期待しますがエラーが返りませんでした")
	}
	if !isErrForbidden(err) { // ErrForbidden でない場合は失敗する
		t.Errorf("ErrForbidden を期待しますが %v が返りました", err)
	}
}

// TestGetPodLogs_存在しないDeployment は存在しない Deployment で gorm.ErrRecordNotFound が返ることを確認する
func TestGetPodLogs_存在しないDeployment(t *testing.T) {
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return nil, gorm.ErrRecordNotFound // レコードなしエラーを返す
		},
	}
	projectRepo := &mockProjectRepository{}
	podLogChunkRepo := &mockPodLogChunkRepository{}

	logSvc := NewLogService(deploymentRepo, projectRepo, podLogChunkRepo)                               // サービスを生成する
	_, err := logSvc.GetPodLogs(context.Background(), "test-user-id", "deployment-1", nil)
	if err == nil { // エラーが発生しない場合は失敗する
		t.Fatal("エラーを期待しますが nil が返りました")
	}
}

// TestGetPodLogs_since指定でSinceメソッドが使われる は since 指定時に FindByDeploymentIDSince が呼ばれることを確認する
func TestGetPodLogs_since指定でSinceメソッドが使われる(t *testing.T) {
	sinceUsed := false // FindByDeploymentIDSince が呼ばれたか記録する
	sinceTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) // テスト用 since を設定する

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-1"}, nil // テスト用 Deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}
	podLogChunkRepo := &mockPodLogChunkRepository{
		findByDeploymentIDSinceFunc: func(ctx context.Context, deploymentID string, since time.Time) ([]models.PodLogChunk, error) {
			sinceUsed = true                     // 呼ばれたことを記録する
			return []models.PodLogChunk{}, nil   // 空チャンクを返す
		},
	}

	logSvc := NewLogService(deploymentRepo, projectRepo, podLogChunkRepo) // サービスを生成する
	_, err := logSvc.GetPodLogs(context.Background(), "test-user-id", "deployment-1", &sinceTime)
	if err != nil { // エラーが発生した場合は失敗する
		t.Fatalf("GetPodLogs() がエラーを返しました: %v", err)
	}
	if !sinceUsed { // FindByDeploymentIDSince が呼ばれなかった場合は失敗する
		t.Error("since 指定時に FindByDeploymentIDSince が呼ばれませんでした")
	}
}

// isErrForbidden は err が ErrForbidden かどうかを確認するヘルパー関数
func isErrForbidden(err error) bool {
	return err == ErrForbidden // ErrForbidden と比較する
}
