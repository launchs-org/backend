package service

import (
	"app/models"
	"app/repository"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var (
	applyTestDB     *gorm.DB   // テスト用 DB 接続（パッケージ内で共有する）
	applyTestDBOnce sync.Once  // 初期化を一度だけ実行するための Once
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
			&models.UserQuota{},
			&models.Project{},
			&models.HarborCredential{},
			&models.Deployment{},
			&models.DeploymentBuild{},
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
		UserID:    "test-user-id",         // テスト用ユーザー ID を設定する
		Name:      "test-project-" + namespace, // テスト用プロジェクト名を設定する
		Namespace: namespace,              // テスト用 namespace を設定する
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
	deploymentData := &models.Deployment{
		ProjectID:           projectID,                          // プロジェクト ID を設定する
		Name:                name,                               // デプロイメント名を設定する
		Type:                models.DeploymentTypeImageURL,      // タイプを設定する
		Status:              models.DeploymentStatusPending,     // ステータスを pending に設定する
		AppStatus:           models.AppStatusPending,            // アプリステータスを pending に設定する
		PendingImageURL:     "nginx:latest",                     // pending image_url を設定する
		PendingInstanceSize: "small",                            // pending instance_size を設定する
		PendingReplicas:     1,                                  // pending replicas を設定する
	}
	if err := db.Create(deploymentData).Error; err != nil {
		t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する
	return deploymentData
}

// ensureInstanceSize はテスト用の InstanceSize レコードが存在することを確認するヘルパー関数
func ensureInstanceSize(t *testing.T, db *gorm.DB) {
	t.Helper()
	instanceSizeData := &models.InstanceSize{
		Size:          "small",    // サイズ名を設定する
		CPURequest:    "100m",     // CPU リクエストを設定する
		CPULimit:      "500m",     // CPU リミットを設定する
		MemoryRequest: "128Mi",    // メモリリクエストを設定する
		MemoryLimit:   "512Mi",    // メモリリミットを設定する
	}
	db.Where("size = ?", "small").FirstOrCreate(instanceSizeData) // 存在しない場合のみ作成する
}

// TestApplyService_Apply_正常にapplyされk8sDeploymentが作成される は apply 後に k8s Deployment が作成されることを確認する
func TestApplyService_Apply_正常にapplyされk8sDeploymentが作成される(t *testing.T) {
	db := setupApplyTestDB(t)                                    // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                    // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-1") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-apply-1") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
	})

	fakeK8sClient := fake.NewSimpleClientset()                   // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)     // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db) // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)           // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)           // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db) // ingress_route リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	result, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil {
		t.Fatalf("Apply がエラーを返しました: %v", err)
	}
	if result == nil { // 結果が返ることを確認する
		t.Fatal("Apply の結果が nil です")
	}
	if result.Status != "applied" { // ステータスが applied であることを確認する
		t.Errorf("期待する status: applied, 実際の status: %s", result.Status)
	}

	// k8s に Deployment が作成されていることを確認する
	k8sDeployment, err := fakeK8sClient.AppsV1().Deployments(projectData.Namespace).Get(
		context.Background(), deploymentData.Name, metav1.GetOptions{},
	)
	if err != nil { // k8s Deployment が作成されていることを確認する
		t.Fatalf("k8s Deployment が作成されていません: %v", err)
	}
	if k8sDeployment.Name != deploymentData.Name { // Deployment 名が一致することを確認する
		t.Errorf("期待する Deployment 名: %s, 実際の Deployment 名: %s", deploymentData.Name, k8sDeployment.Name)
	}
}

// TestApplyService_Apply_apply後にpendingフィールドが空になる は apply 後に pending_*** がクリアされることを確認する
func TestApplyService_Apply_apply後にpendingフィールドが空になる(t *testing.T) {
	db := setupApplyTestDB(t)                                    // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                    // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-2") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-apply-2") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
	})

	fakeK8sClient := fake.NewSimpleClientset()                   // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)     // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db) // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)           // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)           // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db) // ingress_route リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil {
		t.Fatalf("Apply がエラーを返しました: %v", err)
	}

	// DB から取得して pending フィールドが空になっていることを確認する
	fetchedDeployment, err := deploymentRepo.FindByID(context.Background(), deploymentData.ID) // apply 後のレコードを repository 経由で取得する
	if err != nil {
		t.Fatalf("Deployment の取得に失敗しました: %v", err)
	}
	if fetchedDeployment.PendingImageURL != "" { // pending_image_url がクリアされていることを確認する
		t.Errorf("pending_image_url がクリアされていません: %s", fetchedDeployment.PendingImageURL)
	}
	if fetchedDeployment.PendingInstanceSize != "" { // pending_instance_size がクリアされていることを確認する
		t.Errorf("pending_instance_size がクリアされていません: %s", fetchedDeployment.PendingInstanceSize)
	}
	if fetchedDeployment.PendingReplicas != 0 { // pending_replicas がクリアされていることを確認する
		t.Errorf("pending_replicas がクリアされていません: %d", fetchedDeployment.PendingReplicas)
	}
}

// TestApplyService_Apply_apply後にcurrent値が更新される は apply 後に current フィールドが昇格されることを確認する
func TestApplyService_Apply_apply後にcurrent値が更新される(t *testing.T) {
	db := setupApplyTestDB(t)                                    // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                    // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-3") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-apply-3") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
	})

	fakeK8sClient := fake.NewSimpleClientset()                   // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)     // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db) // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)           // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)           // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db) // ingress_route リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil {
		t.Fatalf("Apply がエラーを返しました: %v", err)
	}

	// DB から取得して current 値が昇格されていることを確認する
	fetchedDeployment, err := deploymentRepo.FindByID(context.Background(), deploymentData.ID) // apply 後のレコードを repository 経由で取得する
	if err != nil {
		t.Fatalf("Deployment の取得に失敗しました: %v", err)
	}
	if fetchedDeployment.ImageURL != "nginx:latest" { // image_url が昇格されていることを確認する
		t.Errorf("期待する image_url: nginx:latest, 実際の image_url: %s", fetchedDeployment.ImageURL)
	}
	if fetchedDeployment.InstanceSize != "small" { // instance_size が昇格されていることを確認する
		t.Errorf("期待する instance_size: small, 実際の instance_size: %s", fetchedDeployment.InstanceSize)
	}
	if fetchedDeployment.Replicas != 1 { // replicas が昇格されていることを確認する
		t.Errorf("期待する replicas: 1, 実際の replicas: %d", fetchedDeployment.Replicas)
	}
}

// TestApplyService_Apply_apply後にstatusがrunningappstatusがdeployingになる は apply 後のステータス遷移を確認する
func TestApplyService_Apply_apply後にstatusがrunningappstatusがdeployingになる(t *testing.T) {
	db := setupApplyTestDB(t)                                    // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                    // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-4") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-apply-4") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
	})

	fakeK8sClient := fake.NewSimpleClientset()                   // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)     // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db) // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)           // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)           // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db) // ingress_route リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil {
		t.Fatalf("Apply がエラーを返しました: %v", err)
	}

	// DB から取得してステータスを確認する
	fetchedDeployment, err := deploymentRepo.FindByID(context.Background(), deploymentData.ID) // apply 後のレコードを repository 経由で取得する
	if err != nil {
		t.Fatalf("Deployment の取得に失敗しました: %v", err)
	}
	if fetchedDeployment.Status != models.DeploymentStatusRunning { // status が running であることを確認する
		t.Errorf("期待する status: running, 実際の status: %s", fetchedDeployment.Status)
	}
	if fetchedDeployment.AppStatus != models.AppStatusDeploying { // app_status が deploying であることを確認する
		t.Errorf("期待する app_status: deploying, 実際の app_status: %s", fetchedDeployment.AppStatus)
	}
	if fetchedDeployment.AppliedAt == nil { // applied_at が設定されていることを確認する
		t.Error("applied_at が設定されていません")
	}
}

// TestApplyService_Apply_applyHistoryが1件作成される は apply 後に apply_history が1件作成されることを確認する
func TestApplyService_Apply_applyHistoryが1件作成される(t *testing.T) {
	db := setupApplyTestDB(t)                                    // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                    // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-5") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-apply-5") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
	})

	fakeK8sClient := fake.NewSimpleClientset()                   // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)     // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db) // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)           // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)           // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db) // ingress_route リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	result, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil {
		t.Fatalf("Apply がエラーを返しました: %v", err)
	}
	if result.ApplyHistoryID == "" { // apply_history の ID が返ることを確認する
		t.Error("ApplyHistoryID が設定されていません")
	}

	// apply_history が1件作成されていることを確認する
	historyList, historyErr := applyHistoryRepo.FindAllByDeploymentID(context.Background(), deploymentData.ID) // apply_history 一覧を repository 経由で取得する
	if historyErr != nil {
		t.Fatalf("apply_history の取得に失敗しました: %v", historyErr)
	}
	if len(historyList) != 1 { // 1件作成されていることを確認する
		t.Errorf("期待する apply_history 件数: 1, 実際の件数: %d", len(historyList))
	}

	// apply_history の status が applied であることを確認する
	if historyList[0].Status != models.ApplyStatusApplied { // status が applied であることを確認する
		t.Errorf("期待する apply_history status: applied, 実際の status: %s", historyList[0].Status)
	}
}

// mockFailingK8sClient は k8s apply を失敗させるためのモック用 fake クライアントを準備するヘルパー
// fake.NewSimpleClientset では apply 失敗をシミュレートできないため、
// k8s.Interface を満たすカスタムモックを定義する

// applyHistoryMockDeploymentRepository は Apply テスト専用の DeploymentRepository モック
type applyHistoryMockDeploymentRepository struct {
	deploymentData *models.Deployment // 返す deployment データ
	updatedValues  map[string]interface{} // Updates で渡された values を記録する
}

func (mock *applyHistoryMockDeploymentRepository) Create(ctx context.Context, deployment *models.Deployment) error {
	return nil // 使用しない
}

func (mock *applyHistoryMockDeploymentRepository) FindByID(ctx context.Context, deploymentID string) (*models.Deployment, error) {
	return mock.deploymentData, nil // deployment を返す
}

func (mock *applyHistoryMockDeploymentRepository) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, deploymentID string) (*models.Deployment, error) {
	return mock.deploymentData, nil // FOR UPDATE で deployment を返す
}

func (mock *applyHistoryMockDeploymentRepository) FindAllByProjectID(ctx context.Context, projectID string) ([]models.Deployment, error) {
	return nil, nil // 使用しない
}

func (mock *applyHistoryMockDeploymentRepository) Save(ctx context.Context, deployment *models.Deployment) error {
	return nil // 使用しない
}

func (mock *applyHistoryMockDeploymentRepository) Updates(ctx context.Context, tx *gorm.DB, deployment *models.Deployment, values map[string]interface{}) error {
	mock.updatedValues = values // 渡された values を記録する
	return nil
}

func (mock *applyHistoryMockDeploymentRepository) UpdateAppStatus(ctx context.Context, deploymentID string, appStatus models.AppStatus) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *applyHistoryMockDeploymentRepository) UpdateK8sStatus(ctx context.Context, deploymentID string, k8sStatus datatypes.JSON) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *applyHistoryMockDeploymentRepository) UpdatePendingImageURL(ctx context.Context, deploymentID string, imageURL string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *applyHistoryMockDeploymentRepository) UpdatePendingGithubCommitSHA(ctx context.Context, deploymentID string, commitSHA string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *applyHistoryMockDeploymentRepository) Delete(ctx context.Context, deploymentID string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *applyHistoryMockDeploymentRepository) FindAllRunning(ctx context.Context) ([]models.Deployment, error) {
	return nil, nil // テストでは使用しないためデフォルト nil を返す
}

// applyHistoryMockProjectRepository は Apply テスト専用の ProjectRepository モック
type applyHistoryMockProjectRepository struct {
	projectData *models.Project // 返す project データ
}

func (mock *applyHistoryMockProjectRepository) Create(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil // 使用しない
}

func (mock *applyHistoryMockProjectRepository) FindByID(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error) {
	return mock.projectData, nil // project を返す
}

func (mock *applyHistoryMockProjectRepository) FindByIDNoTx(ctx context.Context, projectID string) (*models.Project, error) {
	return mock.projectData, nil // project を返す
}

func (mock *applyHistoryMockProjectRepository) FindAllByUserID(ctx context.Context, userID string) ([]*models.Project, error) {
	return nil, nil // 使用しない
}

func (mock *applyHistoryMockProjectRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, project *models.Project, status models.ProjectStatus) error {
	return nil // 使用しない
}

func (mock *applyHistoryMockProjectRepository) Save(ctx context.Context, project *models.Project) error {
	return nil // 使用しない
}

func (mock *applyHistoryMockProjectRepository) FindByNamespace(ctx context.Context, namespace string) (*models.Project, error) {
	return nil, nil // 使用しない
}

func (mock *applyHistoryMockProjectRepository) Delete(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil // 使用しない
}

func (mock *applyHistoryMockProjectRepository) DeleteNoTx(ctx context.Context, project *models.Project) error {
	return nil // 使用しない
}

// applyHistoryMockRepository は Apply テスト専用の ApplyHistoryRepository モック
type applyHistoryMockRepository struct {
	createdHistory  *models.ApplyHistory // Create で渡された history を記録する
	updatedStatus   models.ApplyStatus   // UpdateStatus で渡された status を記録する
	updatedHistory  *models.ApplyHistory // UpdateStatus で渡された history を記録する
}

func (mock *applyHistoryMockRepository) Create(ctx context.Context, tx *gorm.DB, history *models.ApplyHistory) error {
	history.ID = "apply-history-id-1"  // テスト用 ID を付与する
	mock.createdHistory = history       // 記録する
	return nil
}

func (mock *applyHistoryMockRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, history *models.ApplyHistory, status models.ApplyStatus) error {
	mock.updatedStatus = status   // 更新されたステータスを記録する
	mock.updatedHistory = history // 更新された history を記録する
	return nil
}

func (mock *applyHistoryMockRepository) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.ApplyHistory, error) {
	return nil, nil // Apply テストでは使用しない
}

func (mock *applyHistoryMockRepository) DeleteAllByDeploymentID(ctx context.Context, deploymentID string) error {
	return nil // Apply テストでは使用しない
}

// TestApplyService_Apply_k8sapply失敗時にapplyHistorystatusがfailedになる は k8s apply 失敗時の挙動を確認する
func TestApplyService_Apply_k8sapply失敗時にapplyHistorystatusがfailedになる(t *testing.T) {
	db := setupApplyTestDB(t)                                    // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                    // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-6") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-apply-6") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
	})

	// k8s apply を失敗させるために Get/Create が失敗する fake クライアントを使う
	// fake クライアントに Create 前に既存の Deployment を仕込み、Update をエラーにする方法は難しいため
	// ここでは実際の DB + モックリポジトリを組み合わせる

	applyHistoryMock := &applyHistoryMockRepository{}             // apply_history モックを生成する
	deploymentMock := &applyHistoryMockDeploymentRepository{      // deployment モックを生成する
		deploymentData: deploymentData,
	}
	projectMock := &applyHistoryMockProjectRepository{            // project モックを生成する
		projectData: projectData,
	}

	// テスト用 k8s Deployment を fake に追加して Update を失敗させるオブジェクトを用意する
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentData.Name,
			Namespace: projectData.Namespace,
		},
	}
	fakeK8sClient := fake.NewSimpleClientset(existingDeployment) // 既存の Deployment を持つ fake クライアントを生成する

	// k8s apply が失敗する Reactor を追加する
	fakeK8sClient.Fake.PrependReactor("update", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("k8s update failed: simulated error") // エラーを返す
	})

	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentMock, applyHistoryMock, projectMock, repository.NewServiceRepository(db), repository.NewIngressRouteRepository(db), repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する（失敗が期待される）
	if err == nil { // エラーが返ることを確認する
		t.Fatal("k8s apply 失敗時にエラーが返るべきですが nil が返りました")
	}

	// apply_history の status が failed に更新されていることを確認する
	if applyHistoryMock.updatedStatus != models.ApplyStatusFailed { // status が failed であることを確認する
		t.Errorf("期待する apply_history status: failed, 実際の status: %s", applyHistoryMock.updatedStatus)
	}
}

// TestApplyService_Apply_k8sapply失敗時にpendingフィールドがそのまま残る は k8s apply 失敗時に pending フィールドが変更されないことを確認する
func TestApplyService_Apply_k8sapply失敗時にpendingフィールドがそのまま残る(t *testing.T) {
	db := setupApplyTestDB(t)                                    // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                    // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-7") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-apply-7") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
	})

	// k8s apply を失敗させる Reactor 付き fake クライアントを生成する
	fakeK8sClient := fake.NewSimpleClientset()                   // fake k8s クライアントを生成する
	fakeK8sClient.Fake.PrependReactor("create", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("k8s create failed: simulated error") // Create をエラーにする
	})

	projectMock := &applyHistoryMockProjectRepository{            // project モックを生成する
		projectData: projectData,
	}

	deploymentRepo := repository.NewDeploymentRepository(db)     // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db) // apply_history リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectMock, repository.NewServiceRepository(db), repository.NewIngressRouteRepository(db), repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する（失敗が期待される）
	if err == nil { // エラーが返ることを確認する
		t.Fatal("k8s apply 失敗時にエラーが返るべきですが nil が返りました")
	}

	// DB から取得して pending フィールドが変更されていないことを確認する
	fetchedDeployment, fetchErr := repository.NewDeploymentRepository(db).FindByID(context.Background(), deploymentData.ID) // apply 後のレコードを repository 経由で取得する
	if fetchErr != nil {
		t.Fatalf("Deployment の取得に失敗しました: %v", fetchErr)
	}
	if fetchedDeployment.PendingImageURL != "nginx:latest" { // pending_image_url がそのままであることを確認する
		t.Errorf("k8s apply 失敗時に pending_image_url が変更されています: %s", fetchedDeployment.PendingImageURL)
	}
	if fetchedDeployment.Status != models.DeploymentStatusPending { // status が変更されていないことを確認する
		t.Errorf("k8s apply 失敗時に status が変更されています: %s", fetchedDeployment.Status)
	}
}

// listApplyHistoriesMockRepository は ListApplyHistories テスト専用の ApplyHistoryRepository モック
type listApplyHistoriesMockRepository struct {
	historyList []*models.ApplyHistory // FindAllByDeploymentID で返す履歴一覧
}

func (mock *listApplyHistoriesMockRepository) Create(ctx context.Context, tx *gorm.DB, history *models.ApplyHistory) error {
	return nil // 使用しない
}

func (mock *listApplyHistoriesMockRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, history *models.ApplyHistory, status models.ApplyStatus) error {
	return nil // 使用しない
}

func (mock *listApplyHistoriesMockRepository) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.ApplyHistory, error) {
	return mock.historyList, nil // 設定した履歴一覧を返す
}

func (mock *listApplyHistoriesMockRepository) DeleteAllByDeploymentID(ctx context.Context, deploymentID string) error {
	return nil // テストでは使用しない
}

// TestApplyService_ListApplyHistories_正常に履歴一覧が取得できる は正常系で履歴一覧が返ることを確認する
func TestApplyService_ListApplyHistories_正常に履歴一覧が取得できる(t *testing.T) {
	db := setupApplyTestDB(t)                                           // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-list-hist-1") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-list-hist-1") // テスト用 Deployment を作成する

	expectedHistoryList := []*models.ApplyHistory{
		{ID: "hist-1", DeploymentID: deploymentData.ID, Status: models.ApplyStatusApplied}, // 1件目の履歴
		{ID: "hist-2", DeploymentID: deploymentData.ID, Status: models.ApplyStatusFailed},  // 2件目の履歴
	}

	deploymentRepo := repository.NewDeploymentRepository(db)          // リポジトリを生成する
	applyHistoryRepo := &listApplyHistoriesMockRepository{             // モックリポジトリを生成する
		historyList: expectedHistoryList,
	}
	projectRepo := repository.NewProjectRepository(db)                // project リポジトリを生成する
	applyService := NewApplyService(db, nil, nil, deploymentRepo, applyHistoryRepo, projectRepo, repository.NewServiceRepository(db), repository.NewIngressRouteRepository(db), repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	historyList, err := applyService.ListApplyHistories(context.Background(), "test-user-id", deploymentData.ID) // 履歴一覧を取得する
	if err != nil {
		t.Fatalf("ListApplyHistories がエラーを返しました: %v", err)
	}
	if len(historyList) != 2 { // 2件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(historyList))
	}
}

// TestApplyService_ListApplyHistories_他ユーザーのdeploymentはErrForbiddenになる は所有者不一致時に ErrForbidden が返ることを確認する
func TestApplyService_ListApplyHistories_他ユーザーのdeploymentはErrForbiddenになる(t *testing.T) {
	db := setupApplyTestDB(t)                                           // テスト用 DB を準備する
	projectData := createApplyTestProject(t, db, "test-ns-list-hist-2") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-list-hist-2") // テスト用 Deployment を作成する

	deploymentRepo := repository.NewDeploymentRepository(db)          // リポジトリを生成する
	applyHistoryRepo := &listApplyHistoriesMockRepository{}            // モックリポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)                // project リポジトリを生成する
	applyService := NewApplyService(db, nil, nil, deploymentRepo, applyHistoryRepo, projectRepo, repository.NewServiceRepository(db), repository.NewIngressRouteRepository(db), repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.ListApplyHistories(context.Background(), "other-user-id", deploymentData.ID) // 別ユーザーで取得する
	if !errors.Is(err, ErrForbidden) { // ErrForbidden が返ることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestApplyService_ListApplyHistories_存在しないdeploymentはエラーになる は deployment が存在しない場合にエラーが返ることを確認する
func TestApplyService_ListApplyHistories_存在しないdeploymentはエラーになる(t *testing.T) {
	db := setupApplyTestDB(t) // テスト用 DB を準備する

	deploymentRepo := repository.NewDeploymentRepository(db)          // リポジトリを生成する
	applyHistoryRepo := &listApplyHistoriesMockRepository{}            // モックリポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)                // project リポジトリを生成する
	applyService := NewApplyService(db, nil, nil, deploymentRepo, applyHistoryRepo, projectRepo, repository.NewServiceRepository(db), repository.NewIngressRouteRepository(db), repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.ListApplyHistories(context.Background(), "test-user-id", "non-existent-id") // 存在しない ID で取得する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("存在しない deployment ID でエラーが返るべきですが nil が返りました")
	}
}

// TestApplyService_Apply_applyでk8sServiceが作成される は apply 後に k8s Service が作成されることを確認する
func TestApplyService_Apply_applyでk8sServiceが作成される(t *testing.T) {
	db := setupApplyTestDB(t)                                       // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                       // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-svc-1") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-svc-1") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.Service{})      // service を削除する
	})

	serviceRecord := &models.Service{                               // テスト用 Service レコードを生成する
		DeploymentID:      deploymentData.ID,                       // デプロイメント ID を設定する
		PendingPort:       80,                                      // pending ポートを設定する
		PendingTargetPort: 8080,                                    // pending ターゲットポートを設定する
		Type:              models.ServiceTypeClusterIP,             // タイプを設定する
		Status:            models.ServiceStatusPending,             // ステータスを pending に設定する
	}
	if err := db.Create(serviceRecord).Error; err != nil {          // Service レコードを作成する
		t.Fatalf("テスト用 Service の作成に失敗しました: %v", err)
	}

	fakeK8sClient := fake.NewSimpleClientset()                      // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)        // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db)    // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)              // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)              // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db)    // ingress_route リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil {
		t.Fatalf("Apply がエラーを返しました: %v", err)
	}

	// k8s に Service が作成されていることを確認する（Service 名は deploymentName + "-svc"）
	expectedServiceName := deploymentData.Name + "-svc"                                     // 期待する Service 名を生成する
	k8sService, err := fakeK8sClient.CoreV1().Services(projectData.Namespace).Get(
		context.Background(), expectedServiceName, metav1.GetOptions{},
	)
	if err != nil { // k8s Service が作成されていることを確認する
		t.Fatalf("k8s Service が作成されていません: %v", err)
	}
	if k8sService.Name != expectedServiceName { // Service 名が一致することを確認する
		t.Errorf("期待する Service 名: %s, 実際の Service 名: %s", expectedServiceName, k8sService.Name)
	}
}

// TestApplyService_Apply_apply後にServiceのpendingフィールドがクリアされる は apply 後に Service の pending フィールドがクリアされることを確認する
func TestApplyService_Apply_apply後にServiceのpendingフィールドがクリアされる(t *testing.T) {
	db := setupApplyTestDB(t)                                       // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                       // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-svc-2") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-svc-2") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.Service{})      // service を削除する
	})

	serviceRecord := &models.Service{                               // テスト用 Service レコードを生成する
		DeploymentID:      deploymentData.ID,                       // デプロイメント ID を設定する
		PendingPort:       80,                                      // pending ポートを設定する
		PendingTargetPort: 8080,                                    // pending ターゲットポートを設定する
		Type:              models.ServiceTypeClusterIP,             // タイプを設定する
		Status:            models.ServiceStatusPending,             // ステータスを pending に設定する
	}
	if err := db.Create(serviceRecord).Error; err != nil {          // Service レコードを作成する
		t.Fatalf("テスト用 Service の作成に失敗しました: %v", err)
	}

	fakeK8sClient := fake.NewSimpleClientset()                      // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)        // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db)    // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)              // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)              // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db)    // ingress_route リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil {
		t.Fatalf("Apply がエラーを返しました: %v", err)
	}

	// DB から Service を再取得して pending フィールドがクリアされていることを確認する
	fetchedService, serviceErr := serviceRepo.FindByDeploymentID(context.Background(), deploymentData.ID) // apply 後のレコードを repository 経由で取得する
	if serviceErr != nil {
		t.Fatalf("Service の取得に失敗しました: %v", serviceErr)
	}
	if fetchedService.PendingPort != 0 {                             // pending_port がクリアされていることを確認する
		t.Errorf("pending_port がクリアされていません: %d", fetchedService.PendingPort)
	}
	if fetchedService.PendingTargetPort != 0 {                       // pending_target_port がクリアされていることを確認する
		t.Errorf("pending_target_port がクリアされていません: %d", fetchedService.PendingTargetPort)
	}
	if fetchedService.Status != models.ServiceStatusActive {         // status が active であることを確認する
		t.Errorf("期待する status: active, 実際の status: %s", fetchedService.Status)
	}
}

// TestApplyService_Apply_Serviceがない場合でもapplyが成功する は Service レコードなしでも apply が成功することを確認する
func TestApplyService_Apply_Serviceがない場合でもapplyが成功する(t *testing.T) {
	db := setupApplyTestDB(t)                                       // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                       // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-svc-3") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-svc-3") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
	})
	_ = projectData // namespace 確認用に参照する

	fakeK8sClient := fake.NewSimpleClientset()                      // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)        // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db)    // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)              // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)              // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db)    // ingress_route リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil {                                                  // エラーが返らないことを確認する
		t.Fatalf("Service なしの場合に Apply がエラーを返しました: %v", err)
	}
}

// TestApplyService_Apply_k8sService失敗時にapplyHistoryがfailedになる は k8s Service 作成失敗時に apply_history が failed になることを確認する
func TestApplyService_Apply_k8sService失敗時にapplyHistoryがfailedになる(t *testing.T) {
	db := setupApplyTestDB(t)                                       // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                       // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-svc-4") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-svc-4") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.Service{})      // service を削除する
	})

	serviceRecord := &models.Service{                               // テスト用 Service レコードを生成する
		DeploymentID:      deploymentData.ID,                       // デプロイメント ID を設定する
		PendingPort:       80,                                      // pending ポートを設定する
		PendingTargetPort: 8080,                                    // pending ターゲットポートを設定する
		Type:              models.ServiceTypeClusterIP,             // タイプを設定する
		Status:            models.ServiceStatusPending,             // ステータスを pending に設定する
	}
	if err := db.Create(serviceRecord).Error; err != nil {          // Service レコードを作成する
		t.Fatalf("テスト用 Service の作成に失敗しました: %v", err)
	}

	applyHistoryMock := &applyHistoryMockRepository{}               // apply_history モックを生成する
	deploymentMock := &applyHistoryMockDeploymentRepository{         // deployment モックを生成する
		deploymentData: deploymentData,
	}
	projectMock := &applyHistoryMockProjectRepository{              // project モックを生成する
		projectData: projectData,
	}

	fakeK8sClient := fake.NewSimpleClientset()                      // fake k8s クライアントを生成する
	fakeK8sClient.Fake.PrependReactor("create", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("k8s service create failed: simulated error") // k8s Service 作成をエラーにする
	})

	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentMock, applyHistoryMock, projectMock, repository.NewServiceRepository(db), repository.NewIngressRouteRepository(db), repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する（失敗が期待される）
	if err == nil {                                                  // エラーが返ることを確認する
		t.Fatal("k8s Service 作成失敗時にエラーが返るべきですが nil が返りました")
	}

	// apply_history の status が failed に更新されていることを確認する
	if applyHistoryMock.updatedStatus != models.ApplyStatusFailed { // status が failed であることを確認する
		t.Errorf("期待する apply_history status: failed, 実際の status: %s", applyHistoryMock.updatedStatus)
	}
}

// createApplyTestEnvVar はテスト用の EnvVar レコードを作成するヘルパー関数
func createApplyTestEnvVar(t *testing.T, db *gorm.DB, projectID string, key string, value string, isSecret bool) *models.EnvVar {
	t.Helper()
	envVarData := &models.EnvVar{
		ProjectID: projectID, // プロジェクト ID を設定する
		Key:       key,       // キーを設定する
		Value:     value,     // 値を設定する
		IsSecret:  isSecret,  // シークレットフラグを設定する
		Status:    "active",  // ステータスを active に設定する
	}
	if err := db.Create(envVarData).Error; err != nil {
		t.Fatalf("テスト用 EnvVar の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(envVarData) }) // テスト終了後にレコードを削除する
	return envVarData
}

// createApplyTestEnvVarMount はテスト用の EnvVarMount レコードを作成するヘルパー関数
func createApplyTestEnvVarMount(t *testing.T, db *gorm.DB, deploymentID string, envVarID string, overrideKey string) *models.EnvVarMount {
	t.Helper()
	mountData := &models.EnvVarMount{
		DeploymentID: deploymentID,                     // デプロイメント ID を設定する
		EnvVarID:     envVarID,                         // env_var ID を設定する
		OverrideKey:  overrideKey,                      // オーバーライドキーを設定する
		Status:       models.EnvVarMountStatusPending,  // ステータスを pending に設定する
	}
	if err := db.Create(mountData).Error; err != nil {
		t.Fatalf("テスト用 EnvVarMount の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(mountData) }) // テスト終了後にレコードを削除する
	return mountData
}

// TestApplyService_Apply_applyでConfigMapとSecretが作成される は apply 後に k8s ConfigMap と Secret が作成されることを確認する
func TestApplyService_Apply_applyでConfigMapとSecretが作成される(t *testing.T) {
	db := setupApplyTestDB(t)                                           // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                           // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-cm-1") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-cm-1") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.EnvVarMount{})  // env_var_mount を削除する
	})

	// 非シークレット環境変数とシークレット環境変数を作成する
	envVarPlain := createApplyTestEnvVar(t, db, projectData.ID, "APP_ENV", "production", false)    // 非シークレット env_var を作成する
	envVarSecret := createApplyTestEnvVar(t, db, projectData.ID, "DB_PASSWORD", "secret123", true) // シークレット env_var を作成する
	createApplyTestEnvVarMount(t, db, deploymentData.ID, envVarPlain.ID, "")                       // 非シークレットマウントを作成する
	createApplyTestEnvVarMount(t, db, deploymentData.ID, envVarSecret.ID, "")                      // シークレットマウントを作成する

	fakeK8sClient := fake.NewSimpleClientset()                          // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)            // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db)        // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)                  // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)                  // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db)        // ingress_route リポジトリを生成する
	envVarRepo := repository.NewEnvVarRepository(db)                    // env_var リポジトリを生成する
	envVarMountRepo := repository.NewEnvVarMountRepository(db)          // env_var_mount リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), envVarRepo, envVarMountRepo, repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil {
		t.Fatalf("Apply がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	// k8s に ConfigMap が作成されていることを確認する（命名規則: {deployName}-env）
	expectedConfigMapName := deploymentData.Name + "-env"                                         // 期待する ConfigMap 名を生成する
	configMap, cmErr := fakeK8sClient.CoreV1().ConfigMaps(projectData.Namespace).Get(
		context.Background(), expectedConfigMapName, metav1.GetOptions{},
	)
	if cmErr != nil { // ConfigMap が作成されていることを確認する
		t.Fatalf("k8s ConfigMap が作成されていません: %v", cmErr)
	}
	if configMap.Data["APP_ENV"] != "production" { // ConfigMap のデータが正しいことを確認する
		t.Errorf("期待する APP_ENV: production, 実際: %s", configMap.Data["APP_ENV"])
	}

	// k8s に Secret が作成されていることを確認する（命名規則: {deployName}-secret）
	expectedSecretName := deploymentData.Name + "-secret"                                        // 期待する Secret 名を生成する
	secretObj, secretErr := fakeK8sClient.CoreV1().Secrets(projectData.Namespace).Get(
		context.Background(), expectedSecretName, metav1.GetOptions{},
	)
	if secretErr != nil { // Secret が作成されていることを確認する
		t.Fatalf("k8s Secret が作成されていません: %v", secretErr)
	}
	if string(secretObj.Data["DB_PASSWORD"]) != "secret123" { // Secret のデータが正しいことを確認する
		t.Errorf("期待する DB_PASSWORD: secret123, 実際: %s", string(secretObj.Data["DB_PASSWORD"]))
	}
}

// TestApplyService_Apply_DeploymentのenvFromにConfigMapとSecretが設定される は apply 後に Deployment の envFrom にマウント設定が反映されることを確認する
func TestApplyService_Apply_DeploymentのenvFromにConfigMapとSecretが設定される(t *testing.T) {
	db := setupApplyTestDB(t)                                           // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                           // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-cm-2") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-cm-2") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.EnvVarMount{})  // env_var_mount を削除する
	})

	// 環境変数とマウントを作成する
	envVarData := createApplyTestEnvVar(t, db, projectData.ID, "API_KEY", "key-value", false) // env_var を作成する
	createApplyTestEnvVarMount(t, db, deploymentData.ID, envVarData.ID, "")                   // マウントを作成する

	fakeK8sClient := fake.NewSimpleClientset()                          // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)            // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db)        // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)                  // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)                  // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db)        // ingress_route リポジトリを生成する
	envVarRepo := repository.NewEnvVarRepository(db)                    // env_var リポジトリを生成する
	envVarMountRepo := repository.NewEnvVarMountRepository(db)          // env_var_mount リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), envVarRepo, envVarMountRepo, repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil {
		t.Fatalf("Apply がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	// k8s Deployment の envFrom に ConfigMap/Secret が設定されていることを確認する
	k8sDeployment, deployErr := fakeK8sClient.AppsV1().Deployments(projectData.Namespace).Get(
		context.Background(), deploymentData.Name, metav1.GetOptions{},
	)
	if deployErr != nil { // Deployment が取得できることを確認する
		t.Fatalf("k8s Deployment の取得に失敗しました: %v", deployErr)
	}

	containers := k8sDeployment.Spec.Template.Spec.Containers // コンテナ一覧を取得する
	if len(containers) == 0 {                                  // コンテナが存在することを確認する
		t.Fatal("Deployment にコンテナが存在しません")
	}
	envFromList := containers[0].EnvFrom                       // envFrom を取得する
	if len(envFromList) == 0 {                                 // envFrom が設定されていることを確認する
		t.Fatal("Deployment の envFrom が設定されていません")
	}

	// ConfigMap の envFrom が設定されていることを確認する
	foundConfigMap := false // ConfigMap が見つかったかどうかのフラグ
	foundSecret := false    // Secret が見つかったかどうかのフラグ
	for _, envFromItem := range envFromList {
		if envFromItem.ConfigMapRef != nil && envFromItem.ConfigMapRef.Name == deploymentData.Name+"-env" {
			foundConfigMap = true // ConfigMap の envFrom が設定されていることを確認する
		}
		if envFromItem.SecretRef != nil && envFromItem.SecretRef.Name == deploymentData.Name+"-secret" {
			foundSecret = true // Secret の envFrom が設定されていることを確認する
		}
	}
	if !foundConfigMap { // ConfigMap の envFrom が設定されていることを確認する
		t.Errorf("Deployment の envFrom に ConfigMap（%s-env）が設定されていません", deploymentData.Name)
	}
	if !foundSecret { // Secret の envFrom が設定されていることを確認する
		t.Errorf("Deployment の envFrom に Secret（%s-secret）が設定されていません", deploymentData.Name)
	}
}

// duplicateKeyMockEnvVarMountRepository は重複キーテスト専用の EnvVarMountRepository モック
type duplicateKeyMockEnvVarMountRepository struct {
	mountList []*models.EnvVarMount // FindAllByDeploymentID で返すマウント一覧
}

func (mock *duplicateKeyMockEnvVarMountRepository) Create(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error {
	return nil // 使用しない
}

func (mock *duplicateKeyMockEnvVarMountRepository) FindByID(ctx context.Context, mountID string) (*models.EnvVarMount, error) {
	return nil, nil // 使用しない
}

func (mock *duplicateKeyMockEnvVarMountRepository) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.EnvVarMount, error) {
	return mock.mountList, nil // 設定したマウント一覧を返す
}

func (mock *duplicateKeyMockEnvVarMountRepository) FindByDeploymentIDAndEnvVarID(ctx context.Context, deploymentID string, envVarID string) (*models.EnvVarMount, error) {
	return nil, nil // 使用しない
}

func (mock *duplicateKeyMockEnvVarMountRepository) Delete(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error {
	return nil // 使用しない
}

// duplicateKeyMockEnvVarRepository は重複キーテスト専用の EnvVarRepository モック
type duplicateKeyMockEnvVarRepository struct {
	envVarMap map[string]*models.EnvVar // ID から EnvVar を返すマップ
}

func (mock *duplicateKeyMockEnvVarRepository) Create(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
	return nil // 使用しない
}

func (mock *duplicateKeyMockEnvVarRepository) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, envVarID string) (*models.EnvVar, error) {
	return nil, nil // 使用しない
}

func (mock *duplicateKeyMockEnvVarRepository) FindByID(ctx context.Context, envVarID string) (*models.EnvVar, error) {
	envVarData, ok := mock.envVarMap[envVarID] // マップから EnvVar を取得する
	if !ok {
		return nil, errors.New("env_var not found") // 見つからない場合はエラーを返す
	}
	return envVarData, nil // EnvVar を返す
}

func (mock *duplicateKeyMockEnvVarRepository) FindAllByProjectID(ctx context.Context, projectID string) ([]*models.EnvVar, error) {
	return nil, nil // 使用しない
}

func (mock *duplicateKeyMockEnvVarRepository) Update(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
	return nil // 使用しない
}

func (mock *duplicateKeyMockEnvVarRepository) Delete(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
	return nil // 使用しない
}

// TestApplyService_Apply_重複キーが存在する場合applyがエラーになる は重複キーが存在する場合に apply がエラーになることを確認する
func TestApplyService_Apply_重複キーが存在する場合applyがエラーになる(t *testing.T) {
	db := setupApplyTestDB(t)                                           // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                           // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-dup-1") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-dup-1") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
	})

	// 同じキー名 "DUPLICATE_KEY" を持つ2つのマウントを用意する
	envVar1ID := "env-var-id-1" // 1つ目の env_var ID
	envVar2ID := "env-var-id-2" // 2つ目の env_var ID
	envVarMountMock := &duplicateKeyMockEnvVarMountRepository{ // マウントモックを生成する
		mountList: []*models.EnvVarMount{
			{EnvVarID: envVar1ID, OverrideKey: ""},              // 1つ目（キー名: DUPLICATE_KEY）
			{EnvVarID: envVar2ID, OverrideKey: "DUPLICATE_KEY"}, // 2つ目（override_key で重複させる）
		},
	}
	envVarMock := &duplicateKeyMockEnvVarRepository{ // env_var モックを生成する
		envVarMap: map[string]*models.EnvVar{
			envVar1ID: {ID: envVar1ID, Key: "DUPLICATE_KEY", Value: "value1", IsSecret: false}, // 1つ目
			envVar2ID: {ID: envVar2ID, Key: "OTHER_KEY", Value: "value2", IsSecret: false},     // 2つ目（override_key で DUPLICATE_KEY になる）
		},
	}

	applyHistoryMockRepo := &applyHistoryMockRepository{}            // apply_history モックを生成する
	deploymentMock := &applyHistoryMockDeploymentRepository{         // deployment モックを生成する
		deploymentData: deploymentData,
	}
	projectMock := &applyHistoryMockProjectRepository{               // project モックを生成する
		projectData: projectData,
	}

	fakeK8sClient := fake.NewSimpleClientset()                       // fake k8s クライアントを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentMock, applyHistoryMockRepo, projectMock, repository.NewServiceRepository(db), repository.NewIngressRouteRepository(db), repository.NewPathRuleRepository(db), envVarMock, envVarMountMock, repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する（失敗が期待される）
	if err == nil { // エラーが返ることを確認する
		t.Fatal("重複キーが存在する場合にエラーが返るべきですが nil が返りました")
	}
	if !errors.Is(err, ErrDuplicateEnvKey) { // ErrDuplicateEnvKey が返ることを確認する
		t.Errorf("期待するエラー: ErrDuplicateEnvKey, 実際のエラー: %v", err)
	}

	// apply_history の status が failed に更新されていることをモックで確認する
	if applyHistoryMockRepo.updatedStatus != models.ApplyStatusFailed { // status が failed であることを確認する
		t.Errorf("期待する apply_history status: failed, 実際の status: %s", applyHistoryMockRepo.updatedStatus)
	}
}

// TestApplyService_Apply_ConfigMapのみの場合も正常にapplyできる は ConfigMap のみの場合（Secret なし）に apply が成功することを確認する
func TestApplyService_Apply_ConfigMapのみの場合も正常にapplyできる(t *testing.T) {
	db := setupApplyTestDB(t)                                            // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                            // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-cm-3")  // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-cm-3") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.EnvVarMount{})  // env_var_mount を削除する
	})

	// 非シークレット環境変数のみ作成する（Secret なし）
	envVarData := createApplyTestEnvVar(t, db, projectData.ID, "PLAIN_KEY", "plain-value", false) // 非シークレット env_var を作成する
	createApplyTestEnvVarMount(t, db, deploymentData.ID, envVarData.ID, "")                       // マウントを作成する

	fakeK8sClient := fake.NewSimpleClientset()                           // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)             // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db)         // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)                   // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)                   // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db)         // ingress_route リポジトリを生成する
	envVarRepo := repository.NewEnvVarRepository(db)                     // env_var リポジトリを生成する
	envVarMountRepo := repository.NewEnvVarMountRepository(db)           // env_var_mount リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), envVarRepo, envVarMountRepo, repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil { // エラーが返らないことを確認する
		t.Fatalf("ConfigMap のみの場合に Apply がエラーを返しました: %v", err)
	}

	// k8s に ConfigMap が作成されていることを確認する
	expectedConfigMapName := deploymentData.Name + "-env"                                        // 期待する ConfigMap 名を生成する
	_, cmErr := fakeK8sClient.CoreV1().ConfigMaps(projectData.Namespace).Get(
		context.Background(), expectedConfigMapName, metav1.GetOptions{},
	)
	if cmErr != nil { // ConfigMap が作成されていることを確認する
		t.Fatalf("k8s ConfigMap が作成されていません: %v", cmErr)
	}

	// k8s に Secret が作成されていないことを確認する（シークレットが存在しないため）
	expectedSecretName := deploymentData.Name + "-secret"                                       // 期待する Secret 名を生成する
	_, secretErr := fakeK8sClient.CoreV1().Secrets(projectData.Namespace).Get(
		context.Background(), expectedSecretName, metav1.GetOptions{},
	)
	if secretErr == nil { // Secret が作成されていないことを確認する
		t.Error("シークレットが存在しないのに k8s Secret が作成されています")
	}
}

// TestApplyService_Apply_Secretのみの場合も正常にapplyできる は Secret のみの場合（ConfigMap なし）に apply が成功することを確認する
func TestApplyService_Apply_Secretのみの場合も正常にapplyできる(t *testing.T) {
	db := setupApplyTestDB(t)                                            // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                            // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-apply-sec-1") // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-sec-1") // テスト用 Deployment を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.EnvVarMount{})  // env_var_mount を削除する
	})

	// シークレット環境変数のみ作成する（ConfigMap なし）
	envVarData := createApplyTestEnvVar(t, db, projectData.ID, "SECRET_KEY", "secret-value", true) // シークレット env_var を作成する
	createApplyTestEnvVarMount(t, db, deploymentData.ID, envVarData.ID, "")                        // マウントを作成する

	fakeK8sClient := fake.NewSimpleClientset()                           // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)             // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db)         // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)                   // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)                   // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db)         // ingress_route リポジトリを生成する
	envVarRepo := repository.NewEnvVarRepository(db)                     // env_var リポジトリを生成する
	envVarMountRepo := repository.NewEnvVarMountRepository(db)           // env_var_mount リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), envVarRepo, envVarMountRepo, repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil { // エラーが返らないことを確認する
		t.Fatalf("Secret のみの場合に Apply がエラーを返しました: %v", err)
	}

	// k8s に Secret が作成されていることを確認する
	expectedSecretName := deploymentData.Name + "-secret"                                        // 期待する Secret 名を生成する
	_, secretErr := fakeK8sClient.CoreV1().Secrets(projectData.Namespace).Get(
		context.Background(), expectedSecretName, metav1.GetOptions{},
	)
	if secretErr != nil { // Secret が作成されていることを確認する
		t.Fatalf("k8s Secret が作成されていません: %v", secretErr)
	}

	// k8s に ConfigMap が作成されていないことを確認する（非シークレットが存在しないため）
	expectedConfigMapName := deploymentData.Name + "-env"                                      // 期待する ConfigMap 名を生成する
	_, cmErr := fakeK8sClient.CoreV1().ConfigMaps(projectData.Namespace).Get(
		context.Background(), expectedConfigMapName, metav1.GetOptions{},
	)
	if cmErr == nil { // ConfigMap が作成されていないことを確認する
		t.Error("非シークレットが存在しないのに k8s ConfigMap が作成されています")
	}
}

// createApplyTestVolume はテスト用の Volume レコードを作成するヘルパー関数
func createApplyTestVolume(t *testing.T, db *gorm.DB, projectID string, name string, sizeMB int) *models.Volume {
	t.Helper()
	volumeData := &models.Volume{
		ProjectID: projectID,              // プロジェクト ID を設定する
		Name:      name,                   // ボリューム名を設定する
		SizeMB:    sizeMB,                 // サイズを設定する
		Status:    models.VolumeStatusPending, // ステータスを pending に設定する
	}
	if err := db.Create(volumeData).Error; err != nil {
		t.Fatalf("テスト用 Volume の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(volumeData) }) // テスト終了後にレコードを削除する
	return volumeData
}

// createApplyTestVolumeMount はテスト用の VolumeMount レコードを作成するヘルパー関数
func createApplyTestVolumeMount(t *testing.T, db *gorm.DB, volumeID string, deploymentID string, mountPath string) *models.VolumeMount {
	t.Helper()
	mountData := &models.VolumeMount{
		VolumeID:     volumeID,                       // ボリューム ID を設定する
		DeploymentID: deploymentID,                   // デプロイメント ID を設定する
		MountPath:    mountPath,                      // マウントパスを設定する
		Status:       models.VolumeMountStatusPending, // ステータスを pending に設定する
	}
	if err := db.Create(mountData).Error; err != nil {
		t.Fatalf("テスト用 VolumeMount の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(mountData) }) // テスト終了後にレコードを削除する
	return mountData
}

// TestApplyService_Apply_applyでPVCが作成される は apply 後に k8s PVC が作成されることを確認する
func TestApplyService_Apply_applyでPVCが作成される(t *testing.T) {
	db := setupApplyTestDB(t)                                          // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                          // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-pvc-1")     // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-pvc-1") // テスト用 Deployment を作成する
	volumeData := createApplyTestVolume(t, db, projectData.ID, "test-volume-1", 1024)       // テスト用 Volume を作成する
	createApplyTestVolumeMount(t, db, volumeData.ID, deploymentData.ID, "/data")            // テスト用 VolumeMount を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{})  // apply_history を削除する
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.VolumeMount{})   // volume_mount を削除する
	})

	fakeK8sClient := fake.NewSimpleClientset()                         // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)           // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db)       // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)                 // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)                 // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db)       // ingress_route リポジトリを生成する
	volumeRepo := repository.NewVolumeRepository(db)                   // volume リポジトリを生成する
	volumeMountRepo := repository.NewVolumeMountRepository(db)         // volume_mount リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), volumeRepo, volumeMountRepo, &noopUserQuotaRepository{}) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil {
		t.Fatalf("Apply がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	// k8s に PVC が作成されていることを確認する（命名規則: {volumeID}-pvc）
	expectedPVCName := volumeData.ID + "-pvc"                                             // 期待する PVC 名を生成する
	_, pvcErr := fakeK8sClient.CoreV1().PersistentVolumeClaims(projectData.Namespace).Get(
		context.Background(), expectedPVCName, metav1.GetOptions{},
	)
	if pvcErr != nil { // PVC が作成されていることを確認する
		t.Fatalf("k8s PVC が作成されていません（期待する名前: %s）: %v", expectedPVCName, pvcErr)
	}
}

// TestApplyService_Apply_applyでDeploymentのvolumeMountsが設定される は apply 後に Deployment の volumeMounts にマウント設定が反映されることを確認する
func TestApplyService_Apply_applyでDeploymentのvolumeMountsが設定される(t *testing.T) {
	db := setupApplyTestDB(t)                                          // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                          // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-pvc-2")     // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-pvc-2") // テスト用 Deployment を作成する
	volumeData := createApplyTestVolume(t, db, projectData.ID, "test-volume-2", 512)        // テスト用 Volume を作成する
	createApplyTestVolumeMount(t, db, volumeData.ID, deploymentData.ID, "/mnt/data")        // テスト用 VolumeMount を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.VolumeMount{})  // volume_mount を削除する
	})

	fakeK8sClient := fake.NewSimpleClientset()                         // fake k8s クライアントを生成する
	deploymentRepo := repository.NewDeploymentRepository(db)           // リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(db)       // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(db)                 // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(db)                 // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(db)       // ingress_route リポジトリを生成する
	volumeRepo := repository.NewVolumeRepository(db)                   // volume リポジトリを生成する
	volumeMountRepo := repository.NewVolumeMountRepository(db)         // volume_mount リポジトリを生成する
	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), volumeRepo, volumeMountRepo, &noopUserQuotaRepository{}) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する
	if err != nil {
		t.Fatalf("Apply がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	// k8s Deployment の volumeMounts にマウント設定が反映されていることを確認する
	k8sDeployment, deployErr := fakeK8sClient.AppsV1().Deployments(projectData.Namespace).Get(
		context.Background(), deploymentData.Name, metav1.GetOptions{},
	)
	if deployErr != nil { // Deployment が取得できることを確認する
		t.Fatalf("k8s Deployment の取得に失敗しました: %v", deployErr)
	}

	containers := k8sDeployment.Spec.Template.Spec.Containers // コンテナ一覧を取得する
	if len(containers) == 0 {                                  // コンテナが存在することを確認する
		t.Fatal("Deployment にコンテナが存在しません")
	}

	volumeMountList := containers[0].VolumeMounts // volumeMounts を取得する
	if len(volumeMountList) == 0 {               // volumeMounts が設定されていることを確認する
		t.Fatal("Deployment の volumeMounts が設定されていません")
	}

	found := false // マウント設定が見つかったかどうかのフラグ
	for _, volumeMountItem := range volumeMountList {
		if volumeMountItem.MountPath == "/mnt/data" { // マウントパスが一致することを確認する
			found = true
		}
	}
	if !found { // マウント設定が反映されていることを確認する
		t.Errorf("Deployment の volumeMounts にマウントパス /mnt/data が設定されていません")
	}

	// Pod の volumes に PVC が設定されていることを確認する
	podVolumes := k8sDeployment.Spec.Template.Spec.Volumes // Pod の volumes を取得する
	if len(podVolumes) == 0 {                              // volumes が設定されていることを確認する
		t.Fatal("Pod の volumes が設定されていません")
	}
}

// TestApplyService_Apply_PVC作成失敗時にapplyHistoryがfailedになる は PVC 作成失敗時に apply_history が failed になることを確認する
func TestApplyService_Apply_PVC作成失敗時にapplyHistoryがfailedになる(t *testing.T) {
	db := setupApplyTestDB(t)                                          // テスト用 DB を準備する
	ensureInstanceSize(t, db)                                          // InstanceSize を準備する
	projectData := createApplyTestProject(t, db, "test-ns-pvc-3")     // テスト用 Project を作成する
	deploymentData := createApplyTestDeployment(t, db, projectData.ID, "test-deploy-pvc-3") // テスト用 Deployment を作成する
	volumeData := createApplyTestVolume(t, db, projectData.ID, "test-volume-3", 256)        // テスト用 Volume を作成する
	createApplyTestVolumeMount(t, db, volumeData.ID, deploymentData.ID, "/var/data")        // テスト用 VolumeMount を作成する
	t.Cleanup(func() {
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.ApplyHistory{}) // apply_history を削除する
		db.Unscoped().Where("deployment_id = ?", deploymentData.ID).Delete(&models.VolumeMount{})  // volume_mount を削除する
	})

	applyHistoryMockRepo := &applyHistoryMockRepository{}             // apply_history モックを生成する
	deploymentMock := &applyHistoryMockDeploymentRepository{           // deployment モックを生成する
		deploymentData: deploymentData,
	}
	projectMock := &applyHistoryMockProjectRepository{                // project モックを生成する
		projectData: projectData,
	}

	fakeK8sClient := fake.NewSimpleClientset()                         // fake k8s クライアントを生成する
	fakeK8sClient.Fake.PrependReactor("create", "persistentvolumeclaims", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("k8s pvc create failed: simulated error") // PVC 作成をエラーにする
	})

	applyService := NewApplyService(db, fakeK8sClient, nil, deploymentMock, applyHistoryMockRepo, projectMock, repository.NewServiceRepository(db), repository.NewIngressRouteRepository(db), repository.NewPathRuleRepository(db), repository.NewEnvVarRepository(db), repository.NewEnvVarMountRepository(db), repository.NewVolumeRepository(db), repository.NewVolumeMountRepository(db), repository.NewUserQuotaRepository(db)) // サービスを生成する

	_, err := applyService.Apply(context.Background(), "test-user-id", deploymentData.ID) // apply を実行する（失敗が期待される）
	if err == nil { // エラーが返ることを確認する
		t.Fatal("PVC 作成失敗時にエラーが返るべきですが nil が返りました")
	}

	// apply_history の status が failed に更新されていることを確認する
	if applyHistoryMockRepo.updatedStatus != models.ApplyStatusFailed { // status が failed であることを確認する
		t.Errorf("期待する apply_history status: failed, 実際の status: %s", applyHistoryMockRepo.updatedStatus)
	}
}

// 未使用変数のコンパイルエラーを防ぐためのダミー変数
var _ = datatypes.JSON(`{}`)
var _ = time.Now
