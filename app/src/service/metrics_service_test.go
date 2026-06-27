package service

import (
	"app/models"
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// mockDeploymentMetricsRepository は DeploymentMetricsRepository のテスト用モック実装
type mockDeploymentMetricsRepository struct {
	createBatchFunc         func(ctx context.Context, metricsList []*models.DeploymentMetrics) error
	findByDeploymentIDFunc  func(ctx context.Context, deploymentID string, limit int) ([]*models.DeploymentMetrics, error)
	deleteOlderThanFunc     func(ctx context.Context, before time.Time) error
}

func (mock *mockDeploymentMetricsRepository) CreateBatch(ctx context.Context, metricsList []*models.DeploymentMetrics) error {
	if mock.createBatchFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.createBatchFunc(ctx, metricsList)
	}
	return nil // デフォルトは nil を返す
}

func (mock *mockDeploymentMetricsRepository) FindByDeploymentID(ctx context.Context, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) {
	if mock.findByDeploymentIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByDeploymentIDFunc(ctx, deploymentID, limit)
	}
	return []*models.DeploymentMetrics{}, nil // デフォルトは空一覧を返す
}

func (mock *mockDeploymentMetricsRepository) DeleteOlderThan(ctx context.Context, before time.Time) error {
	if mock.deleteOlderThanFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.deleteOlderThanFunc(ctx, before)
	}
	return nil // デフォルトは nil を返す
}

// mockDeploymentRepoForMetrics は MetricsService テスト用の DeploymentRepository モック
type mockDeploymentRepoForMetrics struct {
	findByIDFunc func(ctx context.Context, deploymentID string) (*models.Deployment, error)
}

func (mock *mockDeploymentRepoForMetrics) Create(ctx context.Context, deployment *models.Deployment) error {
	return nil
}
func (mock *mockDeploymentRepoForMetrics) CreateWithTx(ctx context.Context, tx *gorm.DB, deployment *models.Deployment) error {
	return nil
}
func (mock *mockDeploymentRepoForMetrics) FindByID(ctx context.Context, deploymentID string) (*models.Deployment, error) {
	if mock.findByIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDFunc(ctx, deploymentID)
	}
	return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // デフォルトは deployment を返す
}
func (mock *mockDeploymentRepoForMetrics) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, deploymentID string) (*models.Deployment, error) {
	return nil, nil
}
func (mock *mockDeploymentRepoForMetrics) FindAllByProjectID(ctx context.Context, projectID string) ([]models.Deployment, error) {
	return nil, nil
}
func (mock *mockDeploymentRepoForMetrics) FindAllRunning(ctx context.Context) ([]models.Deployment, error) {
	return nil, nil
}
func (mock *mockDeploymentRepoForMetrics) Save(ctx context.Context, deployment *models.Deployment) error {
	return nil
}
func (mock *mockDeploymentRepoForMetrics) Updates(ctx context.Context, tx *gorm.DB, deployment *models.Deployment, values map[string]interface{}) error {
	return nil
}
func (mock *mockDeploymentRepoForMetrics) UpdateAppStatus(ctx context.Context, deploymentID string, appStatus models.AppStatus) error {
	return nil
}
func (mock *mockDeploymentRepoForMetrics) UpdateK8sStatus(ctx context.Context, deploymentID string, k8sStatus datatypes.JSON) error {
	return nil
}
func (mock *mockDeploymentRepoForMetrics) UpdatePendingImageURL(ctx context.Context, deploymentID string, imageURL string) error {
	return nil
}
func (mock *mockDeploymentRepoForMetrics) UpdatePendingGithubCommitSHA(ctx context.Context, deploymentID string, commitSHA string) error {
	return nil
}
func (mock *mockDeploymentRepoForMetrics) UpdatePendingGithubBuildFields(ctx context.Context, deploymentID string, repoURL string, branch string, commitSHA string, directory string) error {
	return nil
}
func (mock *mockDeploymentRepoForMetrics) UpdateDeleteProgress(ctx context.Context, deploymentID string, progress string) error {
	return nil
}
func (mock *mockDeploymentRepoForMetrics) UpdateDeploymentStatus(ctx context.Context, deploymentID string, status models.DeploymentStatus) error {
	return nil
}
func (mock *mockDeploymentRepoForMetrics) UpdateCurrentBuildID(ctx context.Context, deploymentID string, buildID string) error {
	return nil
}
func (mock *mockDeploymentRepoForMetrics) ClearCurrentBuildID(ctx context.Context, deploymentID string) error {
	return nil
}
func (mock *mockDeploymentRepoForMetrics) Delete(ctx context.Context, deploymentID string) error {
	return nil
}

// mockProjectRepoForMetrics は MetricsService テスト用の ProjectRepository モック
type mockProjectRepoForMetrics struct {
	findByIDNoTxFunc func(ctx context.Context, projectID string) (*models.Project, error)
}

func (mock *mockProjectRepoForMetrics) Create(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil
}
func (mock *mockProjectRepoForMetrics) FindByID(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error) {
	return nil, nil
}
func (mock *mockProjectRepoForMetrics) FindByIDNoTx(ctx context.Context, projectID string) (*models.Project, error) {
	if mock.findByIDNoTxFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDNoTxFunc(ctx, projectID)
	}
	return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // デフォルトは所有者として返す
}
func (mock *mockProjectRepoForMetrics) FindAllByUserID(ctx context.Context, userID string) ([]*models.Project, error) {
	return nil, nil
}
func (mock *mockProjectRepoForMetrics) UpdateStatus(ctx context.Context, tx *gorm.DB, project *models.Project, status models.ProjectStatus) error {
	return nil
}
func (mock *mockProjectRepoForMetrics) Save(ctx context.Context, project *models.Project) error {
	return nil
}
func (mock *mockProjectRepoForMetrics) FindByNamespace(ctx context.Context, namespace string) (*models.Project, error) {
	return nil, nil
}
func (mock *mockProjectRepoForMetrics) Delete(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil
}
func (mock *mockProjectRepoForMetrics) DeleteNoTx(ctx context.Context, project *models.Project) error {
	return nil
}

// TestGetDeploymentMetrics_正常にメトリクスが取得される は MetricsService が正常にメトリクスを返すことを確認する
func TestGetDeploymentMetrics_正常にメトリクスが取得される(t *testing.T) {
	recordedAt := time.Now()                                       // 記録日時を設定する
	expectedMetrics := []*models.DeploymentMetrics{               // 期待するメトリクスリストを設定する
		{
			ID:            "metrics-id-1",
			DeploymentID:  "deployment-id-1",
			PodName:       "pod-a",
			CPUMillicores: 150,
			MemoryBytes:   134217728,
			ReadyReplicas: 1,
			TotalReplicas: 1,
			RecordedAt:    recordedAt,
		},
	}

	metricsRepo := &mockDeploymentMetricsRepository{
		findByDeploymentIDFunc: func(ctx context.Context, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) {
			return expectedMetrics, nil // 期待するメトリクスを返す
		},
	}
	deploymentRepo := &mockDeploymentRepoForMetrics{} // デフォルト実装を使用する
	projectRepo := &mockProjectRepoForMetrics{}       // デフォルト実装を使用する

	svc := NewMetricsService(metricsRepo, deploymentRepo, projectRepo) // サービスを生成する

	result, err := svc.GetDeploymentMetrics(context.Background(), "test-user-id", "deployment-id-1", 120) // メトリクスを取得する
	if err != nil {
		t.Fatalf("GetDeploymentMetrics がエラーを返しました: %v", err)
	}
	if len(result) != 1 { // 1 件返ることを確認する
		t.Errorf("期待する件数: 1, 実際の件数: %d", len(result))
	}
	if result[0].CPUMillicores != 150 { // CPU 使用量を確認する
		t.Errorf("期待する CPU: 150, 実際の CPU: %d", result[0].CPUMillicores)
	}
}

// TestGetDeploymentMetrics_Deployment存在しない は Deployment が存在しない場合にエラーを返すことを確認する
func TestGetDeploymentMetrics_Deployment存在しない(t *testing.T) {
	metricsRepo := &mockDeploymentMetricsRepository{}
	deploymentRepo := &mockDeploymentRepoForMetrics{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return nil, gorm.ErrRecordNotFound // 存在しないエラーを返す
		},
	}
	projectRepo := &mockProjectRepoForMetrics{}

	svc := NewMetricsService(metricsRepo, deploymentRepo, projectRepo) // サービスを生成する

	_, err := svc.GetDeploymentMetrics(context.Background(), "test-user-id", "nonexistent-id", 120) // メトリクスを取得する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("存在しない Deployment に対してエラーが返りませんでした")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) { // ErrRecordNotFound が返ることを確認する
		t.Errorf("期待するエラー: gorm.ErrRecordNotFound, 実際のエラー: %v", err)
	}
}

// TestGetDeploymentMetrics_他ユーザーのDeployment は他ユーザーの Deployment にアクセスした場合に ErrForbidden が返ることを確認する
func TestGetDeploymentMetrics_他ユーザーのDeployment(t *testing.T) {
	metricsRepo := &mockDeploymentMetricsRepository{}
	deploymentRepo := &mockDeploymentRepoForMetrics{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "other-project-id"}, nil // 他プロジェクトの deployment を返す
		},
	}
	projectRepo := &mockProjectRepoForMetrics{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "other-user-id"}, nil // 他ユーザーのプロジェクトを返す
		},
	}

	svc := NewMetricsService(metricsRepo, deploymentRepo, projectRepo) // サービスを生成する

	_, err := svc.GetDeploymentMetrics(context.Background(), "test-user-id", "deployment-id-1", 120) // メトリクスを取得する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("他ユーザーの Deployment にアクセスしてもエラーが返りませんでした")
	}
	if !errors.Is(err, ErrForbidden) { // ErrForbidden が返ることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}
