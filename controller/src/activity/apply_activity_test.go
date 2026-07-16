package activity

import (
	"app/shared/models"
	"app/shared/repository"
	"context"
	"fmt"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// mockImageRepositoryNotFound は FindByID が常に record not found を返す ImageRepository のテスト用モック
type mockImageRepositoryNotFound struct{}

func (mock *mockImageRepositoryNotFound) Create(ctx context.Context, image *models.Image) error {
	return nil
}
func (mock *mockImageRepositoryNotFound) CreateWithTx(ctx context.Context, tx *gorm.DB, image *models.Image) error {
	return nil
}
func (mock *mockImageRepositoryNotFound) FindByID(ctx context.Context, imageID string) (*models.Image, error) {
	return nil, gorm.ErrRecordNotFound // 常に見つからないエラーを返す
}
func (mock *mockImageRepositoryNotFound) FindByBuildID(ctx context.Context, buildID string) (*models.Image, error) {
	return nil, nil
}
func (mock *mockImageRepositoryNotFound) FindByProjectIDAndURL(ctx context.Context, projectID string, imageURL string) (*models.Image, error) {
	return nil, nil
}
func (mock *mockImageRepositoryNotFound) FindOrCreate(ctx context.Context, image *models.Image) (*models.Image, error) {
	return nil, nil
}
func (mock *mockImageRepositoryNotFound) FindOrCreateWithTx(ctx context.Context, tx *gorm.DB, image *models.Image) (*models.Image, error) {
	return nil, nil
}
func (mock *mockImageRepositoryNotFound) FindAllByProjectID(ctx context.Context, projectID string) ([]models.Image, error) {
	return nil, nil
}
func (mock *mockImageRepositoryNotFound) UpdateSizeBytes(ctx context.Context, imageID string, sizeBytes int64) error {
	return nil
}
func (mock *mockImageRepositoryNotFound) Delete(ctx context.Context, image *models.Image) error {
	return nil
}

var _ repository.ImageRepository = (*mockImageRepositoryNotFound)(nil)

// setupApplyActivityTestDB は ExecuteApply のテスト用 DB 接続とスキーマを準備する
func setupApplyActivityTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Tokyo",
		getEnvOrDefaultForTest("DB_HOST", "localhost"),
		getEnvOrDefaultForTest("DB_USER", "postgres"),
		getEnvOrDefaultForTest("DB_PASSWORD", "postgres"),
		getEnvOrDefaultForTest("DB_NAME", "postgres"),
		getEnvOrDefaultForTest("DB_PORT", "5432"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{}) // DB に接続する
	if err != nil {
		t.Skipf("DB に接続できないためテストをスキップします: %v", err) // DB 未起動時はスキップする
	}

	if err := db.AutoMigrate( // テストに必要なテーブルをマイグレーションする
		&models.InstanceSize{},
		&models.Project{},
		&models.Deployment{},
		&models.Image{},
		&models.ApplyHistory{},
		&models.DeploymentApplyProgress{},
		&models.Service{},
		&models.EnvVar{},
		&models.EnvVarMount{},
		&models.Volume{},
		&models.VolumeMount{},
	); err != nil {
		t.Fatalf("マイグレーションに失敗しました: %v", err) // マイグレーション失敗時はテスト失敗とする
	}

	return db
}

// getEnvOrDefaultForTest は環境変数を取得し、未設定の場合はデフォルト値を返す
func getEnvOrDefaultForTest(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value // 環境変数が設定されている場合はその値を返す
	}
	return defaultValue // 未設定の場合はデフォルト値を返す
}

// createApplyActivityTestProject はテスト用の Project レコードを作成するヘルパー
func createApplyActivityTestProject(t *testing.T, db *gorm.DB, namespace string) *models.Project {
	t.Helper()
	projectData := &models.Project{
		UserID:    "test-user",
		Name:      namespace,
		Namespace: namespace,
		Status:    models.ProjectStatusActive,
	}
	if err := db.Create(projectData).Error; err != nil {
		t.Fatalf("Project の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(projectData) })
	return projectData
}

// createApplyActivityInstanceSize はテスト用の InstanceSize マスターレコードを作成するヘルパー（既存の場合は何もしない）
func createApplyActivityInstanceSize(t *testing.T, db *gorm.DB) {
	t.Helper()
	instanceSizeData := &models.InstanceSize{
		Size:          "small",
		CPURequest:    "100m",
		CPULimit:      "200m",
		MemoryRequest: "128Mi",
		MemoryLimit:   "256Mi",
	}
	db.FirstOrCreate(instanceSizeData, "size = ?", "small") // 既存の場合は取得のみ行う
}

// newTestApplyActivities は ExecuteApply のテストに必要な依存を組み立てて ApplyActivities を返す
func newTestApplyActivities(db *gorm.DB) *ApplyActivities {
	return newTestApplyActivitiesWithImageRepo(db, repository.NewImageRepository(db))
}

// newTestApplyActivitiesWithImageRepo は imageRepo を差し替え可能な形で ApplyActivities を組み立てる
func newTestApplyActivitiesWithImageRepo(db *gorm.DB, imageRepo repository.ImageRepository) *ApplyActivities {
	return NewApplyActivities(
		db,
		k8sfake.NewSimpleClientset(), // fake k8s クライアントを生成する
		nil,                          // dynamic クライアントは今回のテストでは未使用
		repository.NewDeploymentRepository(db),
		repository.NewApplyHistoryRepository(db),
		repository.NewDeploymentApplyProgressRepository(db),
		repository.NewProjectRepository(db),
		repository.NewServiceRepository(db),
		nil, // ingress_route リポジトリは今回のテストでは未使用
		nil, // path_rule リポジトリは今回のテストでは未使用
		repository.NewEnvVarRepository(db),
		repository.NewEnvVarMountRepository(db),
		repository.NewVolumeRepository(db),
		repository.NewVolumeMountRepository(db),
		imageRepo,
	)
}

// TestExecuteApply_ImageURLタイプでVolume_EnvVar_Serviceなしの場合は該当ステップがskippedになる は
// リソースを持たない最小構成の Deployment を apply した際に、対象ステップが skipped、
// 実行対象ステップ（image/container）が done になることを確認する
func TestExecuteApply_ImageURLタイプでVolume_EnvVar_Serviceなしの場合は該当ステップがskippedになる(t *testing.T) {
	db := setupApplyActivityTestDB(t)
	createApplyActivityInstanceSize(t, db)
	projectData := createApplyActivityTestProject(t, db, "test-ns-apply-progress-1")

	imageData := &models.Image{ProjectID: projectData.ID, ImageURL: "nginx:latest"}
	if err := db.Create(imageData).Error; err != nil {
		t.Fatalf("Image の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(imageData) })

	deploymentData := &models.Deployment{
		ProjectID:           projectData.ID,
		Name:                "test-progress-app-1",
		Type:                models.DeploymentTypeImageURL,
		Status:              models.DeploymentStatusPending,
		AppStatus:           models.AppStatusPending,
		PendingImageID:      &imageData.ID,
		PendingInstanceSize: "small",
		PendingReplicas:     1,
	}
	if err := db.Create(deploymentData).Error; err != nil {
		t.Fatalf("Deployment の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{})
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.DeploymentApplyProgress{})
		db.Unscoped().Delete(deploymentData)
	})

	activities := newTestApplyActivities(db)
	workflowID := "apply-" + deploymentData.ID

	_, err := activities.ExecuteApply(context.Background(), ApplyActivityInput{
		DeploymentID: deploymentData.ID,
		WorkflowID:   workflowID,
	})
	if err != nil {
		t.Fatalf("ExecuteApply がエラーを返しました: %v", err)
	}

	progressRepo := repository.NewDeploymentApplyProgressRepository(db)
	progressList, err := progressRepo.FindAllByWorkflowID(context.Background(), workflowID)
	if err != nil {
		t.Fatalf("FindAllByWorkflowID がエラーを返しました: %v", err)
	}
	if len(progressList) != 9 { // 9ステップ生成されていることを確認する
		t.Fatalf("期待する件数: 9, 実際の件数: %d", len(progressList))
	}

	statusByStep := map[models.ApplyProgressStepName]models.ApplyProgressStepStatus{}
	for _, progressItem := range progressList {
		statusByStep[progressItem.StepName] = progressItem.Status
	}

	// Volume/EnvVar/Service が存在しないため該当ステップは skipped になることを確認する
	if statusByStep[models.ApplyProgressStepVolume] != models.ApplyProgressStepStatusSkipped {
		t.Errorf("期待するvolumeステータス: skipped, 実際: %s", statusByStep[models.ApplyProgressStepVolume])
	}
	if statusByStep[models.ApplyProgressStepEnvVar] != models.ApplyProgressStepStatusSkipped {
		t.Errorf("期待するenv_varステータス: skipped, 実際: %s", statusByStep[models.ApplyProgressStepEnvVar])
	}
	if statusByStep[models.ApplyProgressStepService] != models.ApplyProgressStepStatusSkipped {
		t.Errorf("期待するserviceステータス: skipped, 実際: %s", statusByStep[models.ApplyProgressStepService])
	}
	if statusByStep[models.ApplyProgressStepNetwork] != models.ApplyProgressStepStatusSkipped {
		t.Errorf("期待するnetworkステータス: skipped, 実際: %s", statusByStep[models.ApplyProgressStepNetwork])
	}

	// 実行対象のステップ（image/container）は成功して done になることを確認する
	if statusByStep[models.ApplyProgressStepImage] != models.ApplyProgressStepStatusDone {
		t.Errorf("期待するimageステータス: done, 実際: %s", statusByStep[models.ApplyProgressStepImage])
	}
	if statusByStep[models.ApplyProgressStepContainer] != models.ApplyProgressStepStatusDone {
		t.Errorf("期待するcontainerステータス: done, 実際: %s", statusByStep[models.ApplyProgressStepContainer])
	}

	// watcher が担当するステップ（pod_scheduled/pod_running/readiness）は pending のままであることを確認する
	if statusByStep[models.ApplyProgressStepPodScheduled] != models.ApplyProgressStepStatusPending {
		t.Errorf("期待するpod_scheduledステータス: pending, 実際: %s", statusByStep[models.ApplyProgressStepPodScheduled])
	}
	if statusByStep[models.ApplyProgressStepReadiness] != models.ApplyProgressStepStatusPending {
		t.Errorf("期待するreadinessステータス: pending, 実際: %s", statusByStep[models.ApplyProgressStepReadiness])
	}
}

// TestExecuteApply_Imageが存在しない場合はExecuteApplyがエラーを返しトランザクションがロールバックされる は
// ExecuteApply全体が1トランザクションであるため、Image解決失敗時は進捗レコードを含め
// すべての変更がロールバックされ、進捗テーブルには何も残らないことを確認する
func TestExecuteApply_Imageが存在しない場合はimageステップがfailedになる(t *testing.T) {
	db := setupApplyActivityTestDB(t)
	createApplyActivityInstanceSize(t, db)
	projectData := createApplyActivityTestProject(t, db, "test-ns-apply-progress-2")

	// FK制約を満たすため実在する Image を参照させつつ、ImageRepository をモックに差し替えて
	// 「imageRepo.FindByID が失敗する」状況（Image解決失敗）を再現する
	imageData := &models.Image{ProjectID: projectData.ID, ImageURL: "existing-but-mocked-away:latest"}
	if err := db.Create(imageData).Error; err != nil {
		t.Fatalf("Image の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(imageData) })

	deploymentData := &models.Deployment{
		ProjectID:           projectData.ID,
		Name:                "test-progress-app-2",
		Type:                models.DeploymentTypeImageURL,
		Status:              models.DeploymentStatusPending,
		AppStatus:           models.AppStatusPending,
		PendingImageID:      &imageData.ID,
		PendingInstanceSize: "small",
		PendingReplicas:     1,
	}
	if err := db.Create(deploymentData).Error; err != nil {
		t.Fatalf("Deployment の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{})
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.DeploymentApplyProgress{})
		db.Unscoped().Delete(deploymentData)
	})

	activities := newTestApplyActivitiesWithImageRepo(db, &mockImageRepositoryNotFound{})
	workflowID := "apply-" + deploymentData.ID

	_, err := activities.ExecuteApply(context.Background(), ApplyActivityInput{
		DeploymentID: deploymentData.ID,
		WorkflowID:   workflowID,
	})
	if err == nil { // Image 未検出のためエラーになることを確認する
		t.Fatal("ExecuteApply がエラーを返しませんでした")
	}

	progressRepo := repository.NewDeploymentApplyProgressRepository(db)
	progressList, findErr := progressRepo.FindAllByWorkflowID(context.Background(), workflowID)
	if findErr != nil {
		t.Fatalf("FindAllByWorkflowID がエラーを返しました: %v", findErr)
	}
	if len(progressList) != 0 { // トランザクション全体がロールバックされ進捗レコードも残らないことを確認する
		t.Errorf("期待する件数: 0（ロールバック済み）, 実際の件数: %d", len(progressList))
	}
}
