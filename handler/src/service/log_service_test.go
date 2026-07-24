package service

import (
	"handler/models"
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes/fake"
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

func (mock *mockPodLogChunkRepository) DeleteByDeploymentIDAndPodNameNotIn(ctx context.Context, deploymentID string, activePodNames []string) error {
	return nil // テストでは使用しない
}

func (mock *mockPodLogChunkRepository) DeleteByPodName(ctx context.Context, deploymentID string, podName string) error {
	return nil // テストでは使用しない
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

	logSvc := NewLogService(deploymentRepo, projectRepo, podLogChunkRepo, fake.NewSimpleClientset()) // サービスを生成する
	result, err := logSvc.GetPodLogs(context.Background(), "test-user-id", "deployment-1", nil)
	if err != nil { // エラーが発生した場合は失敗する
		t.Fatalf("GetPodLogs() がエラーを返しました: %v", err)
	}
	if len(result.Pods) != 0 { // Pods が空でない場合は失敗する
		t.Errorf("空 Pods を期待しますが、実際は %d 件です", len(result.Pods))
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
				{ID: "chunk-1", DeploymentID: deploymentID, PodName: "pod-aaa", Content: "line1\n", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}, // 1件目のチャンクを返す
				{ID: "chunk-2", DeploymentID: deploymentID, PodName: "pod-aaa", Content: "line2\n", CreatedAt: expectedTime},                               // 2件目のチャンクを返す
			}, nil
		},
	}

	logSvc := NewLogService(deploymentRepo, projectRepo, podLogChunkRepo, fake.NewSimpleClientset()) // サービスを生成する
	result, err := logSvc.GetPodLogs(context.Background(), "test-user-id", "deployment-1", nil)
	if err != nil { // エラーが発生した場合は失敗する
		t.Fatalf("GetPodLogs() がエラーを返しました: %v", err)
	}
	if len(result.Pods) != 1 { // Pod 数を確認する
		t.Fatalf("期待する Pod 数 1、実際の Pod 数 %d", len(result.Pods))
	}
	podEntry := result.Pods[0]                        // 1件目の Pod エントリを取得する
	if podEntry.PodName != "pod-aaa" {                // Pod 名を確認する
		t.Errorf("期待する PodName %q、実際の PodName %q", "pod-aaa", podEntry.PodName)
	}
	if podEntry.Logs != "line1\nline2\n" { // ログが結合されているか確認する
		t.Errorf("期待するログ %q、実際のログ %q", "line1\nline2\n", podEntry.Logs)
	}
	if podEntry.LastTimestamp == nil { // LastTimestamp が nil の場合は失敗する
		t.Fatal("LastTimestamp が nil です")
	}
	if !podEntry.LastTimestamp.Equal(expectedTime) { // LastTimestamp が一致しない場合は失敗する
		t.Errorf("期待する LastTimestamp %v、実際の LastTimestamp %v", expectedTime, *podEntry.LastTimestamp)
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

	logSvc := NewLogService(deploymentRepo, projectRepo, podLogChunkRepo, fake.NewSimpleClientset())                               // サービスを生成する
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

	logSvc := NewLogService(deploymentRepo, projectRepo, podLogChunkRepo, fake.NewSimpleClientset())                               // サービスを生成する
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
			sinceUsed = true                    // 呼ばれたことを記録する
			return []models.PodLogChunk{}, nil  // 空チャンクを返す
		},
	}

	logSvc := NewLogService(deploymentRepo, projectRepo, podLogChunkRepo, fake.NewSimpleClientset()) // サービスを生成する
	_, err := logSvc.GetPodLogs(context.Background(), "test-user-id", "deployment-1", &sinceTime)
	if err != nil { // エラーが発生した場合は失敗する
		t.Fatalf("GetPodLogs() がエラーを返しました: %v", err)
	}
	if !sinceUsed { // FindByDeploymentIDSince が呼ばれなかった場合は失敗する
		t.Error("since 指定時に FindByDeploymentIDSince が呼ばれませんでした")
	}
}

// TestGetPodLogs_複数Pod は複数 Pod のチャンクが Pod ごとにグループ化されることを確認する
func TestGetPodLogs_複数Pod(t *testing.T) {
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
				{ID: "c1", DeploymentID: deploymentID, PodName: "pod-aaa", Content: "a1\n", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}, // Pod-aaa の1件目
				{ID: "c2", DeploymentID: deploymentID, PodName: "pod-bbb", Content: "b1\n", CreatedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)}, // Pod-bbb の1件目
				{ID: "c3", DeploymentID: deploymentID, PodName: "pod-aaa", Content: "a2\n", CreatedAt: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)}, // Pod-aaa の2件目
			}, nil
		},
	}

	logSvc := NewLogService(deploymentRepo, projectRepo, podLogChunkRepo, fake.NewSimpleClientset()) // サービスを生成する
	result, err := logSvc.GetPodLogs(context.Background(), "test-user-id", "deployment-1", nil)
	if err != nil { // エラーが発生した場合は失敗する
		t.Fatalf("GetPodLogs() がエラーを返しました: %v", err)
	}
	if len(result.Pods) != 2 { // Pod 数を確認する
		t.Fatalf("期待する Pod 数 2、実際の Pod 数 %d", len(result.Pods))
	}
	if result.Pods[0].PodName != "pod-aaa" { // 登場順に pod-aaa が先に来ることを確認する
		t.Errorf("期待する PodName %q、実際の PodName %q", "pod-aaa", result.Pods[0].PodName)
	}
	if result.Pods[0].Logs != "a1\na2\n" { // pod-aaa のログが結合されているか確認する
		t.Errorf("期待するログ %q、実際のログ %q", "a1\na2\n", result.Pods[0].Logs)
	}
	if result.Pods[1].PodName != "pod-bbb" { // pod-bbb が2番目に来ることを確認する
		t.Errorf("期待する PodName %q、実際の PodName %q", "pod-bbb", result.Pods[1].PodName)
	}
	if result.Pods[1].Logs != "b1\n" { // pod-bbb のログを確認する
		t.Errorf("期待するログ %q、実際のログ %q", "b1\n", result.Pods[1].Logs)
	}
}

// isErrForbidden は err が ErrForbidden かどうかを確認するヘルパー関数
func isErrForbidden(err error) bool {
	return err == ErrForbidden // ErrForbidden と比較する
}
