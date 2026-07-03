package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"handler/models"
	"handler/repository"

	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes/fake"

	temporalclient "go.temporal.io/sdk/client"
)

var (
	applyTestDB     *gorm.DB  // テスト用 DB 接続（パッケージ内で共有する）
	applyTestDBOnce sync.Once // 初期化を一度だけ実行するための Once
)

// setupApplyTestDB はテスト用の DB 接続とスキーマを準備する（パッケージ内で一度だけ初期化する）
func setupApplyTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	applyTestDBOnce.Do(func() { // 一度だけ実行する
		dsn := fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Tokyo",
			getApplyEnvOrDefault("DB_HOST", "localhost"),
			getApplyEnvOrDefault("DB_USER", "postgres"),
			getApplyEnvOrDefault("DB_PASSWORD", "postgres"),
			getApplyEnvOrDefault("DB_NAME", "postgres"),
			getApplyEnvOrDefault("DB_PORT", "5432"),
		)

		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{}) // DB に接続する
		if err != nil {
			return // 接続失敗時は nil のまま（テスト関数でスキップする）
		}

		// テストに必要なテーブルをマイグレーションする（一度だけ実行する）
		if migrateErr := db.AutoMigrate(
			&models.InstanceSize{},
			&models.Plan{},
			&models.PlanInstanceLimit{},
			&models.UserQuota{},
			&models.Project{},
			&models.HarborCredential{},
			&models.Deployment{},
			&models.DeploymentBuild{},
			&models.Image{},
			&models.ApplyHistory{},
			&models.DeploymentWebhook{},
			&models.Service{},
			&models.IngressRoute{},
			&models.EnvVar{},
			&models.EnvVarMount{},
			&models.Volume{},
			&models.VolumeMount{},
		); migrateErr != nil {
			return // マイグレーション失敗時は nil のまま
		}

		applyTestDB = db // 成功時のみセットする
	})

	if applyTestDB == nil { // DB が取得できない場合はスキップする
		t.Skip("DB に接続できないためテストをスキップします")
	}
	return applyTestDB
}

// getApplyEnvOrDefault は環境変数を取得し、未設定の場合はデフォルト値を返す
func getApplyEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value // 環境変数が設定されている場合はその値を返す
	}
	return defaultValue // 未設定の場合はデフォルト値を返す
}

// createApplyTestProject はテスト用の Project レコードを作成するヘルパー関数
func createApplyTestProject(t *testing.T, db *gorm.DB, namespace string) *models.Project {
	t.Helper()
	projectData := &models.Project{
		UserID:    "test-user-id",             // テスト用ユーザー ID を設定する
		Name:      "test-project-" + namespace, // テスト用プロジェクト名を設定する
		Namespace: namespace,                  // テスト用 namespace を設定する
		Status:    models.ProjectStatusActive, // ステータスを active に設定する
	}
	if err := db.Create(projectData).Error; err != nil {
		t.Fatalf("テスト用 Project の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(projectData) }) // テスト終了後にレコードを削除する
	return projectData
}

// createApplyTestDeployment はテスト用の Deployment レコードを作成するヘルパー関数
func createApplyTestDeployment(t *testing.T, db *gorm.DB, projectID string, name string) *models.Deployment {
	t.Helper()

	// テスト用 Image レコードを作成する（pending_image_id が参照する image_url を保持する。同一 Project 内でユニーク制約に抵触しないよう name を含める）
	imageData := &models.Image{
		ProjectID: projectID,                  // プロジェクト ID を設定する
		ImageURL:  "nginx:latest-" + name,      // イメージ URL を設定する
	}
	if err := db.Create(imageData).Error; err != nil {
		t.Fatalf("テスト用 Image の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(imageData) }) // テスト終了後にレコードを削除する

	deploymentData := &models.Deployment{
		ProjectID:           projectID,                     // プロジェクト ID を設定する
		Name:                name,                          // デプロイメント名を設定する
		Type:                models.DeploymentTypeImageURL, // タイプを設定する
		Status:              models.DeploymentStatusPending, // ステータスを pending に設定する
		AppStatus:           models.AppStatusPending,       // アプリステータスを pending に設定する
		PendingImageID:      &imageData.ID,                 // pending image_id を設定する
		PendingInstanceSize: "small",                       // pending instance_size を設定する
		PendingReplicas:     1,                             // pending replicas を設定する
	}
	if err := db.Create(deploymentData).Error; err != nil {
		t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する
	return deploymentData
}

// mockWorkflowStarter は WorkflowStarter のテスト用モック
type mockWorkflowStarter struct {
	executeWorkflowFunc func(ctx context.Context, options temporalclient.StartWorkflowOptions, workflow interface{}, args ...interface{}) (temporalclient.WorkflowRun, error)
	cancelWorkflowFunc  func(ctx context.Context, workflowID string, runID string) error
}

func (mock *mockWorkflowStarter) ExecuteWorkflow(ctx context.Context, options temporalclient.StartWorkflowOptions, workflow interface{}, args ...interface{}) (temporalclient.WorkflowRun, error) {
	if mock.executeWorkflowFunc != nil {
		return mock.executeWorkflowFunc(ctx, options, workflow, args...) // モック関数を呼び出す
	}
	return nil, nil // デフォルトは成功を返す（WorkflowRun は使用しない）
}

func (mock *mockWorkflowStarter) CancelWorkflow(ctx context.Context, workflowID string, runID string) error {
	if mock.cancelWorkflowFunc != nil {
		return mock.cancelWorkflowFunc(ctx, workflowID, runID) // モック関数を呼び出す
	}
	return nil // デフォルトは成功を返す
}

// newApplyServiceForTest は Temporal モックを使ったテスト用 ApplyService を生成する
func newApplyServiceForTest(db *gorm.DB, temporalMock WorkflowStarter) *ApplyService {
	fakeK8sClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	return NewApplyService(
		db,
		fakeK8sClient,
		nil,
		repository.NewDeploymentRepository(db),
		repository.NewApplyHistoryRepository(db),
		repository.NewProjectRepository(db),
		repository.NewServiceRepository(db),
		repository.NewIngressRouteRepository(db),
		repository.NewPathRuleRepository(db),
		&noopUserQuotaRepository{},
		temporalMock,
		"",
	)
}

// TestApplyService_Apply_正常にWorkflowIDが返る は apply 成功時に WorkflowID が返ることを確認する
func TestApplyService_Apply_正常にWorkflowIDが返る(t *testing.T) {
	db := setupApplyTestDB(t)                                     // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-wf") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-app-wf") // テスト用 Deployment を作成する

	temporalMock := &mockWorkflowStarter{} // Temporal モックを生成する

	applyService := newApplyServiceForTest(db, temporalMock) // テスト用サービスを生成する

	result, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // Apply を実行する
	if err != nil {
		t.Fatalf("Apply がエラーを返しました: %v", err)
	}
	expectedWorkflowID := "apply-" + deploymentData.ID // 期待する WorkflowID を生成する
	if result.WorkflowID != expectedWorkflowID {        // WorkflowID を確認する
		t.Errorf("期待する WorkflowID: %s, 実際: %s", expectedWorkflowID, result.WorkflowID)
	}
}

// TestApplyService_Apply_他ユーザーのdeploymentはErrForbiddenになる は他ユーザーの deployment に apply すると ErrForbidden になることを確認する
func TestApplyService_Apply_他ユーザーのdeploymentはErrForbiddenになる(t *testing.T) {
	db := setupApplyTestDB(t)                                           // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-forbid") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-app-forbid") // テスト用 Deployment を作成する

	applyService := newApplyServiceForTest(db, &mockWorkflowStarter{}) // テスト用サービスを生成する

	_, err := applyService.Apply(context.Background(), "other-user-id", deploymentData.ID) // 別ユーザーで Apply を実行する
	if !errors.Is(err, ErrForbidden) {                                                      // ErrForbidden が返ることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際: %v", err)
	}
}

// TestApplyService_Apply_not_init状態はErrNotInitializedになる は not_init 状態の deployment に apply すると ErrNotInitialized になることを確認する
func TestApplyService_Apply_not_init状態はErrNotInitializedになる(t *testing.T) {
	db := setupApplyTestDB(t)                                             // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-notinit") // テスト用 Project を作成する

	// not_init 状態の Deployment を作成する
	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,                  // プロジェクト ID を設定する
		Name:      "test-app-notinit",              // デプロイメント名を設定する
		Type:      models.DeploymentTypeImageURL,   // タイプを設定する
		Status:    models.DeploymentStatusNotInit,  // ステータスを not_init に設定する（初回ビルド未完了）
		AppStatus: models.AppStatusPending,         // アプリステータスを pending に設定する
	}
	if err := db.Create(deploymentData).Error; err != nil {
		t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	applyService := newApplyServiceForTest(db, &mockWorkflowStarter{}) // テスト用サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // Apply を実行する
	if !errors.Is(err, ErrNotInitialized) {                                                // ErrNotInitialized が返ることを確認する
		t.Errorf("期待するエラー: ErrNotInitialized, 実際: %v", err)
	}
}

// TestApplyService_Apply_deploying状態はErrAlreadyApplyingになる は deploying 状態の deployment に apply すると ErrAlreadyApplying になることを確認する
func TestApplyService_Apply_deploying状態はErrAlreadyApplyingになる(t *testing.T) {
	db := setupApplyTestDB(t)                                              // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-deploying") // テスト用 Project を作成する

	// deploying 状態の Deployment を作成する
	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,                  // プロジェクト ID を設定する
		Name:      "test-app-deploying",            // デプロイメント名を設定する
		Type:      models.DeploymentTypeImageURL,   // タイプを設定する
		Status:    models.DeploymentStatusPending,  // ステータスを pending に設定する
		AppStatus: models.AppStatusDeploying,       // アプリステータスを deploying に設定する（apply 中）
		PendingReplicas: 1,                         // pending replicas を設定する
	}
	if err := db.Create(deploymentData).Error; err != nil {
		t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	applyService := newApplyServiceForTest(db, &mockWorkflowStarter{}) // テスト用サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // Apply を実行する
	if !errors.Is(err, ErrAlreadyApplying) {                                               // ErrAlreadyApplying が返ることを確認する
		t.Errorf("期待するエラー: ErrAlreadyApplying, 実際: %v", err)
	}
}

// TestApplyService_Apply_Temporal起動失敗時にエラーを返す は Temporal Workflow 起動失敗時にエラーを返すことを確認する
func TestApplyService_Apply_Temporal起動失敗時にエラーを返す(t *testing.T) {
	db := setupApplyTestDB(t)                                              // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-tempfail") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-app-tempfail") // テスト用 Deployment を作成する

	temporalMock := &mockWorkflowStarter{ // Temporal モックを生成する
		executeWorkflowFunc: func(ctx context.Context, options temporalclient.StartWorkflowOptions, workflow interface{}, args ...interface{}) (temporalclient.WorkflowRun, error) {
			return nil, errors.New("temporal connection refused") // 接続エラーを返す
		},
	}

	applyService := newApplyServiceForTest(db, temporalMock) // テスト用サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // Apply を実行する
	if err == nil {                                                                         // エラーが返ることを確認する
		t.Error("エラーが返されるべきですが、nil が返りました")
	}
}

// TestApplyService_ListApplyHistories_正常に履歴一覧が取得できる は apply 履歴一覧が取得できることを確認する
func TestApplyService_ListApplyHistories_正常に履歴一覧が取得できる(t *testing.T) {
	db := setupApplyTestDB(t)                                            // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-history-list") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-app-hist") // テスト用 Deployment を作成する

	// ApplyHistory レコードを作成する
	historyData := &models.ApplyHistory{
		DeploymentID: deploymentData.ID,                      // deployment ID を設定する
		Status:       models.ApplyStatusApplied,              // ステータスを applied に設定する
		Manifests:    datatypes.JSON([]byte(`{}`)),            // not null 制約を満たすためダミー JSON を設定する
	}
	if err := db.Create(historyData).Error; err != nil {
		t.Fatalf("テスト用 ApplyHistory の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(historyData) }) // テスト終了後にレコードを削除する

	applyService := newApplyServiceForTest(db, &mockWorkflowStarter{}) // テスト用サービスを生成する

	historyList, err := applyService.ListApplyHistories(context.Background(), "test-user-id", deploymentData.ID) // 履歴一覧を取得する
	if err != nil {
		t.Fatalf("ListApplyHistories がエラーを返しました: %v", err)
	}
	if len(historyList) == 0 { // 1件以上取得できることを確認する
		t.Error("ApplyHistory が1件以上返されるべきですが、0件でした")
	}
}

// TestApplyService_ListApplyHistories_他ユーザーのdeploymentはErrForbiddenになる は他ユーザーの deployment の履歴取得で ErrForbidden になることを確認する
func TestApplyService_ListApplyHistories_他ユーザーのdeploymentはErrForbiddenになる(t *testing.T) {
	db := setupApplyTestDB(t)                                               // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-history-forbid")  // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-app-histforbid") // テスト用 Deployment を作成する

	applyService := newApplyServiceForTest(db, &mockWorkflowStarter{}) // テスト用サービスを生成する

	_, err := applyService.ListApplyHistories(context.Background(), "other-user-id", deploymentData.ID) // 別ユーザーで実行する
	if !errors.Is(err, ErrForbidden) {                                                                   // ErrForbidden が返ることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際: %v", err)
	}
}

// TestApplyService_ApplyProject_pendingのあるDeploymentのみapplyされる は pending がある Deployment のみ apply され、成功件数が返ることを確認する
func TestApplyService_ApplyProject_pendingのあるDeploymentのみapplyされる(t *testing.T) {
	db := setupApplyTestDB(t)                                                 // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-project-apply-ok") // テスト用 Project を作成する
	pendingDeploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-app-pending") // pending ありの Deployment を作成する

	// pending なしの Deployment を作成する（apply 対象から除外されることを確認する）
	noPendingDeploymentData := &models.Deployment{
		ProjectID: projectData.ID,                    // プロジェクト ID を設定する
		Name:      "test-app-nopending",              // デプロイメント名を設定する
		Type:      models.DeploymentTypeImageURL,     // タイプを設定する
		Status:    models.DeploymentStatusRunning,    // ステータスを running に設定する
		AppStatus: models.AppStatusRunning,           // アプリステータスを running に設定する
		Replicas:  1,                                 // current replicas を設定する
	}
	if err := db.Create(noPendingDeploymentData).Error; err != nil {
		t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(noPendingDeploymentData) }) // テスト終了後にレコードを削除する

	var appliedDeploymentIDList []string // apply された deployment ID を記録するスライスを定義する
	temporalMock := &mockWorkflowStarter{
		executeWorkflowFunc: func(ctx context.Context, options temporalclient.StartWorkflowOptions, workflow interface{}, args ...interface{}) (temporalclient.WorkflowRun, error) {
			appliedDeploymentIDList = append(appliedDeploymentIDList, options.ID) // 起動された WorkflowID を記録する
			return nil, nil                                                       // 成功を返す
		},
	}

	applyService := newApplyServiceForTest(db, temporalMock) // テスト用サービスを生成する

	result, err := applyService.ApplyProject(context.Background(), "test-user-id", projectData.ID) // ApplyProject を実行する
	if err != nil {
		t.Fatalf("ApplyProject がエラーを返しました: %v", err)
	}
	if result.AppliedDeploymentCount != 1 { // pending ありの Deployment 1件のみ apply されることを確認する
		t.Errorf("期待する AppliedDeploymentCount: 1, 実際: %d", result.AppliedDeploymentCount)
	}
	if len(result.FailedDeploymentList) != 0 { // 失敗がないことを確認する
		t.Errorf("FailedDeploymentList は空であるべきですが、%d件でした", len(result.FailedDeploymentList))
	}
	if !result.IngressRouteApplied { // IngressRoute の apply が実行されたことを確認する
		t.Error("IngressRouteApplied が true であるべきですが、false でした")
	}
	expectedWorkflowID := "apply-" + pendingDeploymentData.ID // pending ありの Deployment のみ apply されることを確認する
	if len(appliedDeploymentIDList) != 1 || appliedDeploymentIDList[0] != expectedWorkflowID {
		t.Errorf("期待する apply 対象 WorkflowID: [%s], 実際: %v", expectedWorkflowID, appliedDeploymentIDList)
	}
}

// TestApplyService_ApplyProject_一部Deploymentが失敗しても他は継続する は一部の Deployment の apply が失敗しても他の Deployment・IngressRoute の apply が継続することを確認する
func TestApplyService_ApplyProject_一部Deploymentが失敗しても他は継続する(t *testing.T) {
	db := setupApplyTestDB(t)                                                    // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-project-apply-partial") // テスト用 Project を作成する
	okDeploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-app-ok") // 成功する Deployment を作成する
	failDeploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-app-fail") // 失敗する Deployment を作成する

	temporalMock := &mockWorkflowStarter{
		executeWorkflowFunc: func(ctx context.Context, options temporalclient.StartWorkflowOptions, workflow interface{}, args ...interface{}) (temporalclient.WorkflowRun, error) {
			if options.ID == "apply-"+failDeploymentData.ID { // 失敗させたい Deployment の場合はエラーを返す
				return nil, errors.New("temporal connection refused")
			}
			return nil, nil // それ以外は成功を返す
		},
	}

	applyService := newApplyServiceForTest(db, temporalMock) // テスト用サービスを生成する

	result, err := applyService.ApplyProject(context.Background(), "test-user-id", projectData.ID) // ApplyProject を実行する
	if err != nil {
		t.Fatalf("ApplyProject がエラーを返しました: %v", err)
	}
	if result.AppliedDeploymentCount != 1 { // 成功した Deployment が1件であることを確認する
		t.Errorf("期待する AppliedDeploymentCount: 1, 実際: %d", result.AppliedDeploymentCount)
	}
	if len(result.FailedDeploymentList) != 1 { // 失敗した Deployment が1件記録されることを確認する
		t.Fatalf("期待する FailedDeploymentList件数: 1, 実際: %d", len(result.FailedDeploymentList))
	}
	if result.FailedDeploymentList[0].DeploymentID != failDeploymentData.ID { // 失敗した Deployment ID が正しいことを確認する
		t.Errorf("期待する失敗 DeploymentID: %s, 実際: %s", failDeploymentData.ID, result.FailedDeploymentList[0].DeploymentID)
	}
	if !result.IngressRouteApplied { // 一部失敗しても IngressRoute の apply は実行されることを確認する
		t.Error("IngressRouteApplied が true であるべきですが、false でした")
	}
	_ = okDeploymentData // 変数を使用したことにする（テストの意図を明示するため）
}

// TestApplyService_ApplyProject_他ユーザーのprojectはErrForbiddenになる は他ユーザーの project に対する一括 apply が ErrForbidden になることを確認する
func TestApplyService_ApplyProject_他ユーザーのprojectはErrForbiddenになる(t *testing.T) {
	db := setupApplyTestDB(t)                                                    // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-project-apply-forbid") // テスト用 Project を作成する

	applyService := newApplyServiceForTest(db, &mockWorkflowStarter{}) // テスト用サービスを生成する

	_, err := applyService.ApplyProject(context.Background(), "other-user-id", projectData.ID) // 別ユーザーで実行する
	if !errors.Is(err, ErrForbidden) {                                                          // ErrForbidden が返ることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際: %v", err)
	}
}

// TestApplyService_GetProjectPendingSummary_pendingを正しく集計する は pending 集計が Deployment の pending 有無を正しく反映することを確認する
func TestApplyService_GetProjectPendingSummary_pendingを正しく集計する(t *testing.T) {
	db := setupApplyTestDB(t)                                                     // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-project-summary")      // テスト用 Project を作成する
	createApplyTestDeployment(t, db, projectData.ID, "test-app-summary-pending") // pending ありの Deployment を作成する

	applyService := newApplyServiceForTest(db, &mockWorkflowStarter{}) // テスト用サービスを生成する

	summary, err := applyService.GetProjectPendingSummary(context.Background(), "test-user-id", projectData.ID) // pending 集計を実行する
	if err != nil {
		t.Fatalf("GetProjectPendingSummary がエラーを返しました: %v", err)
	}
	if !summary.HasPending { // pending ありと判定されることを確認する
		t.Error("HasPending が true であるべきですが、false でした")
	}
	if summary.PendingDeploymentCount != 1 { // pending がある Deployment が1件であることを確認する
		t.Errorf("期待する PendingDeploymentCount: 1, 実際: %d", summary.PendingDeploymentCount)
	}
	if summary.PendingIngressRouteCount != 0 { // IngressRoute は作成していないため0件であることを確認する
		t.Errorf("期待する PendingIngressRouteCount: 0, 実際: %d", summary.PendingIngressRouteCount)
	}
}

// TestApplyService_GetProjectPendingSummary_pendingがない場合falseになる は pending が一切ない場合に HasPending が false になることを確認する
func TestApplyService_GetProjectPendingSummary_pendingがない場合falseになる(t *testing.T) {
	db := setupApplyTestDB(t)                                                          // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-project-summary-nopending") // テスト用 Project を作成する

	// pending なしの Deployment を作成する
	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,                 // プロジェクト ID を設定する
		Name:      "test-app-summary-nopending",   // デプロイメント名を設定する
		Type:      models.DeploymentTypeImageURL,  // タイプを設定する
		Status:    models.DeploymentStatusRunning, // ステータスを running に設定する
		AppStatus: models.AppStatusRunning,        // アプリステータスを running に設定する
		Replicas:  1,                              // current replicas を設定する
	}
	if err := db.Create(deploymentData).Error; err != nil {
		t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	applyService := newApplyServiceForTest(db, &mockWorkflowStarter{}) // テスト用サービスを生成する

	summary, err := applyService.GetProjectPendingSummary(context.Background(), "test-user-id", projectData.ID) // pending 集計を実行する
	if err != nil {
		t.Fatalf("GetProjectPendingSummary がエラーを返しました: %v", err)
	}
	if summary.HasPending { // pending なしと判定されることを確認する
		t.Error("HasPending が false であるべきですが、true でした")
	}
}

// TestApplyService_GetProjectPendingSummary_他ユーザーのprojectはErrForbiddenになる は他ユーザーの project の pending 集計が ErrForbidden になることを確認する
func TestApplyService_GetProjectPendingSummary_他ユーザーのprojectはErrForbiddenになる(t *testing.T) {
	db := setupApplyTestDB(t)                                                     // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-project-summary-forbid") // テスト用 Project を作成する

	applyService := newApplyServiceForTest(db, &mockWorkflowStarter{}) // テスト用サービスを生成する

	_, err := applyService.GetProjectPendingSummary(context.Background(), "other-user-id", projectData.ID) // 別ユーザーで実行する
	if !errors.Is(err, ErrForbidden) {                                                                      // ErrForbidden が返ることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際: %v", err)
	}
}
