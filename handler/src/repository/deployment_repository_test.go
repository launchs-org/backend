package repository

import (
	"handler/models"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupTestDB はテスト用の DB 接続とスキーマを準備する
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Tokyo",
		getEnvOrDefault("DB_HOST", "localhost"),
		getEnvOrDefault("DB_USER", "postgres"),
		getEnvOrDefault("DB_PASSWORD", "postgres"),
		getEnvOrDefault("DB_NAME", "postgres"),
		getEnvOrDefault("DB_PORT", "5432"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{}) // DB に接続する
	if err != nil {
		t.Skipf("DB に接続できないためテストをスキップします: %v", err) // DB 未起動時はスキップする
	}

	// テストに必要なテーブルをマイグレーションする
	if err := db.AutoMigrate(
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
		&models.PathRule{},
		&models.EnvVar{},
		&models.EnvVarMount{},
		&models.Volume{},
		&models.VolumeMount{},
	); err != nil {
		t.Fatalf("マイグレーションに失敗しました: %v", err) // マイグレーション失敗時はテスト失敗とする
	}

	return db
}

// getEnvOrDefault は環境変数を取得し、未設定の場合はデフォルト値を返す
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value // 環境変数が設定されている場合はその値を返す
	}
	return defaultValue // 未設定の場合はデフォルト値を返す
}

// createTestProject はテスト用の Project レコードを作成するヘルパー関数
func createTestProject(t *testing.T, db *gorm.DB) *models.Project {
	t.Helper()
	projectData := &models.Project{
		UserID:    "test-user-id",                // テスト用ユーザー ID を設定する
		Name:      "test-project",                // テスト用プロジェクト名を設定する
		Namespace: "test-namespace",              // テスト用 namespace を設定する
		Status:    models.ProjectStatusActive,    // ステータスを active に設定する
	}
	if err := db.Create(projectData).Error; err != nil {
		t.Fatalf("テスト用 Project の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(projectData) }) // テスト終了後にレコードを削除する
	return projectData
}

// TestDeploymentRepository_Create_正常に作成される は Deployment レコードが作成されることを確認する
func TestDeploymentRepository_Create_正常に作成される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	repo := NewDeploymentRepository(db) // リポジトリを生成する
	deploymentData := &models.Deployment{
		ProjectID:           projectData.ID,                // プロジェクト ID を設定する
		Name:                "test-app",                    // デプロイメント名を設定する
		Type:                models.DeploymentTypeImageURL, // タイプを設定する
		Status:              models.DeploymentStatusPending, // ステータスを設定する
		AppStatus:           models.AppStatusPending,       // アプリステータスを設定する
		PendingImageURL:     "nginx:latest",                // pending image_url を設定する
		PendingInstanceSize: "small",                       // pending instance_size を設定する
		PendingReplicas:     1,                             // pending replicas を設定する
	}

	err := repo.Create(context.Background(), deploymentData) // リポジトリを実行する
	if err != nil {
		t.Fatalf("Create がエラーを返しました: %v", err)
	}
	if deploymentData.ID == "" { // ID が付与されていることを確認する
		t.Error("作成後に ID が設定されていません")
	}
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	// DB から取得して値を確認する
	var fetched models.Deployment
	db.First(&fetched, "id = ?", deploymentData.ID) // 作成したレコードを取得する
	if fetched.Status != models.DeploymentStatusPending { // status が pending であることを確認する
		t.Errorf("期待する status: pending, 実際の status: %s", fetched.Status)
	}
	if fetched.PendingImageURL != "nginx:latest" { // pending_image_url が設定されていることを確認する
		t.Errorf("期待する pending_image_url: nginx:latest, 実際の pending_image_url: %s", fetched.PendingImageURL)
	}
}

// TestDeploymentRepository_FindByID_正常に取得される は FindByID で Deployment が取得されることを確認する
func TestDeploymentRepository_FindByID_正常に取得される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Deployment を作成する
	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,
		Name:      "test-app",
		Type:      models.DeploymentTypeImageURL,
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusPending,
	}
	db.Create(deploymentData)                                                      // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	repo := NewDeploymentRepository(db)                                            // リポジトリを生成する
	result, err := repo.FindByID(context.Background(), deploymentData.ID) // リポジトリを実行する
	if err != nil {
		t.Fatalf("FindByID がエラーを返しました: %v", err)
	}
	if result.ID != deploymentData.ID { // ID が一致することを確認する
		t.Errorf("期待する ID: %s, 実際の ID: %s", deploymentData.ID, result.ID)
	}
	if result.Name != "test-app" { // name が一致することを確認する
		t.Errorf("期待する name: test-app, 実際の name: %s", result.Name)
	}
}

// TestDeploymentRepository_FindByID_存在しないIDはエラーを返す は存在しない ID でエラーが返ることを確認する
func TestDeploymentRepository_FindByID_存在しないIDはエラーを返す(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	repo := NewDeploymentRepository(db)      // リポジトリを生成する

	_, err := repo.FindByID(context.Background(), "00000000-0000-0000-0000-000000000000") // 存在しない ID で検索する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("存在しない ID でエラーが返るべきですが nil が返りました")
	}
}

// TestDeploymentRepository_FindAllByProjectID_正常に一覧が取得される は FindAllByProjectID で一覧が取得されることを確認する
func TestDeploymentRepository_FindAllByProjectID_正常に一覧が取得される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Deployment を2件作成する
	deploymentData1 := &models.Deployment{ProjectID: projectData.ID, Name: "app-1", Type: models.DeploymentTypeImageURL, Status: models.DeploymentStatusPending, AppStatus: models.AppStatusPending}
	deploymentData2 := &models.Deployment{ProjectID: projectData.ID, Name: "app-2", Type: models.DeploymentTypeImageURL, Status: models.DeploymentStatusPending, AppStatus: models.AppStatusPending}
	db.Create(deploymentData1) // 1件目を作成する
	db.Create(deploymentData2) // 2件目を作成する
	t.Cleanup(func() {
		db.Unscoped().Delete(deploymentData1) // テスト終了後にレコードを削除する
		db.Unscoped().Delete(deploymentData2) // テスト終了後にレコードを削除する
	})

	repo := NewDeploymentRepository(db)                                                           // リポジトリを生成する
	result, err := repo.FindAllByProjectID(context.Background(), projectData.ID) // リポジトリを実行する
	if err != nil {
		t.Fatalf("FindAllByProjectID がエラーを返しました: %v", err)
	}
	if len(result) != 2 { // 2件取得されることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(result))
	}
}

// TestDeploymentRepository_Save_正常に更新される は Save で Deployment が更新されることを確認する
func TestDeploymentRepository_Save_正常に更新される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Deployment を作成する
	deploymentData := &models.Deployment{
		ProjectID:       projectData.ID,
		Name:            "test-app",
		Type:            models.DeploymentTypeImageURL,
		Status:          models.DeploymentStatusPending,
		AppStatus:       models.AppStatusPending,
		PendingImageURL: "nginx:1.24", // 更新前の値を設定する
	}
	db.Create(deploymentData)                                          // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	repo := NewDeploymentRepository(db)            // リポジトリを生成する
	deploymentData.PendingImageURL = "nginx:1.25" // pending_image_url を更新する
	err := repo.Save(context.Background(), deploymentData) // リポジトリを実行する
	if err != nil {
		t.Fatalf("Save がエラーを返しました: %v", err)
	}

	// DB から取得して更新を確認する
	var fetched models.Deployment
	db.First(&fetched, "id = ?", deploymentData.ID) // 更新後のレコードを取得する
	if fetched.PendingImageURL != "nginx:1.25" {    // 更新されていることを確認する
		t.Errorf("期待する pending_image_url: nginx:1.25, 実際の pending_image_url: %s", fetched.PendingImageURL)
	}
}

// TestServiceRepository_Create_正常に作成される は Service レコードが作成されることを確認する
func TestServiceRepository_Create_正常に作成される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Deployment を作成する（Service の外部キーとして必要）
	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,
		Name:      "test-app",
		Type:      models.DeploymentTypeImageURL,
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusPending,
	}
	db.Create(deploymentData)                                          // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	repo := NewServiceRepository(db) // リポジトリを生成する
	serviceData := &models.Service{
		DeploymentID: deploymentData.ID,           // デプロイメント ID を設定する
		Status:       models.ServiceStatusPending, // ステータスを設定する
	}

	err := repo.Create(context.Background(), serviceData) // リポジトリを実行する
	if err != nil {
		t.Fatalf("Create がエラーを返しました: %v", err)
	}
	if serviceData.ID == "" { // ID が付与されていることを確認する
		t.Error("作成後に ID が設定されていません")
	}
	t.Cleanup(func() { db.Unscoped().Delete(serviceData) }) // テスト終了後にレコードを削除する

	// DB から取得して値を確認する
	var fetched models.Service
	db.First(&fetched, "id = ?", serviceData.ID) // 作成したレコードを取得する
	if fetched.DeploymentID != deploymentData.ID { // deployment_id が一致することを確認する
		t.Errorf("期待する deployment_id: %s, 実際の deployment_id: %s", deploymentData.ID, fetched.DeploymentID)
	}
	if fetched.Status != models.ServiceStatusPending { // status が pending であることを確認する
		t.Errorf("期待する status: pending, 実際の status: %s", fetched.Status)
	}
}

// TestServiceRepository_FindByDeploymentID_正常に取得される は FindByDeploymentID で Service が取得されることを確認する
func TestServiceRepository_FindByDeploymentID_正常に取得される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Deployment を作成する
	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,
		Name:      "test-app",
		Type:      models.DeploymentTypeImageURL,
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusPending,
	}
	db.Create(deploymentData)                                          // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	// テスト用 Service を作成する
	serviceData := &models.Service{
		DeploymentID: deploymentData.ID,           // デプロイメント ID を設定する
		Port:         8080,                        // ポート番号を設定する
		TargetPort:   3000,                        // ターゲットポートを設定する
		Status:       models.ServiceStatusPending, // ステータスを設定する
	}
	db.Create(serviceData)                                          // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(serviceData) }) // テスト終了後にレコードを削除する

	repo := NewServiceRepository(db)                                                                          // リポジトリを生成する
	result, err := repo.FindByDeploymentID(context.Background(), deploymentData.ID) // リポジトリを実行する
	if err != nil {
		t.Fatalf("FindByDeploymentID がエラーを返しました: %v", err)
	}
	if result.DeploymentID != deploymentData.ID { // deployment_id が一致することを確認する
		t.Errorf("期待する deployment_id: %s, 実際の deployment_id: %s", deploymentData.ID, result.DeploymentID)
	}
	if result.Port != 8080 { // ポート番号が一致することを確認する
		t.Errorf("期待する port: 8080, 実際の port: %d", result.Port)
	}
}

// TestServiceRepository_FindByDeploymentID_存在しないIDはエラーを返す は存在しない ID でエラーが返ることを確認する
func TestServiceRepository_FindByDeploymentID_存在しないIDはエラーを返す(t *testing.T) {
	db := setupTestDB(t)                // テスト用 DB を準備する
	repo := NewServiceRepository(db)   // リポジトリを生成する

	_, err := repo.FindByDeploymentID(context.Background(), "00000000-0000-0000-0000-000000000000") // 存在しない ID で検索する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("存在しない deployment_id でエラーが返るべきですが nil が返りました")
	}
}

// TestServiceRepository_Update_正常に更新される は Update で Service が更新されることを確認する
func TestServiceRepository_Update_正常に更新される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Deployment を作成する
	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,
		Name:      "test-app",
		Type:      models.DeploymentTypeImageURL,
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusPending,
	}
	db.Create(deploymentData)                                          // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	// テスト用 Service を作成する
	serviceData := &models.Service{
		DeploymentID:      deploymentData.ID,           // デプロイメント ID を設定する
		PendingPort:       8080,                        // 更新前の pending port を設定する
		PendingTargetPort: 3000,                        // 更新前の pending target port を設定する
		Status:            models.ServiceStatusPending, // ステータスを設定する
	}
	db.Create(serviceData)                                          // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(serviceData) }) // テスト終了後にレコードを削除する

	repo := NewServiceRepository(db)   // リポジトリを生成する
	serviceData.PendingPort = 9090    // pending_port を更新する
	err := repo.Update(context.Background(), serviceData) // リポジトリを実行する
	if err != nil {
		t.Fatalf("Update がエラーを返しました: %v", err)
	}

	// DB から取得して更新を確認する
	var fetched models.Service
	db.First(&fetched, "id = ?", serviceData.ID) // 更新後のレコードを取得する
	if fetched.PendingPort != 9090 {              // pending_port が更新されていることを確認する
		t.Errorf("期待する pending_port: 9090, 実際の pending_port: %d", fetched.PendingPort)
	}
	if fetched.PendingTargetPort != 3000 { // pending_target_port が変化していないことを確認する
		t.Errorf("pending_target_port は変化しないはずですが変化しています: %d", fetched.PendingTargetPort)
	}
}

// TestDeploymentRepository_UpdateAppStatus_正常に更新される は UpdateAppStatus で app_status が更新されることを確認する
func TestDeploymentRepository_UpdateAppStatus_正常に更新される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	deploymentData := &models.Deployment{ // テスト用 Deployment を作成する
		ProjectID: projectData.ID,            // プロジェクト ID を設定する
		Name:      "test-watcher-app",        // デプロイメント名を設定する
		Type:      models.DeploymentTypeImageURL, // タイプを設定する
		Status:    models.DeploymentStatusPending, // ステータスを設定する
		AppStatus: models.AppStatusPending,   // 初期 app_status を設定する
	}
	db.Create(deploymentData)                                                // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	repo := NewDeploymentRepository(db) // リポジトリを生成する

	err := repo.UpdateAppStatus(context.Background(), deploymentData.ID, models.AppStatusRunning) // app_status を running に更新する
	if err != nil {
		t.Fatalf("UpdateAppStatus がエラーを返しました: %v", err)
	}

	var fetchedDeployment models.Deployment
	db.First(&fetchedDeployment, "id = ?", deploymentData.ID) // 更新後のレコードを取得する
	if fetchedDeployment.AppStatus != models.AppStatusRunning { // app_status が running に更新されていることを確認する
		t.Errorf("期待する app_status: running, 実際の app_status: %s", fetchedDeployment.AppStatus)
	}
}

// TestDeploymentRepository_UpdateAppStatus_存在しないIDはエラーを返す は存在しない ID でエラーが返ることを確認する
func TestDeploymentRepository_UpdateAppStatus_存在しないIDはエラーを返す(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する

	repo := NewDeploymentRepository(db) // リポジトリを生成する

	err := repo.UpdateAppStatus(context.Background(), "00000000-0000-0000-0000-000000000000", models.AppStatusRunning) // 存在しない ID で更新する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("存在しない deploymentID でエラーが返るべきですが nil が返りました")
	}
}

// TestDeploymentRepository_UpdateK8sStatus_正常に更新される は UpdateK8sStatus で k8s_status が更新されることを確認する
func TestDeploymentRepository_UpdateK8sStatus_正常に更新される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	deploymentData := &models.Deployment{ // テスト用 Deployment を作成する
		ProjectID: projectData.ID,               // プロジェクト ID を設定する
		Name:      "test-k8s-status-app",        // デプロイメント名を設定する
		Type:      models.DeploymentTypeImageURL, // タイプを設定する
		Status:    models.DeploymentStatusPending, // ステータスを設定する
		AppStatus: models.AppStatusPending,       // 初期 app_status を設定する
	}
	db.Create(deploymentData)                                                // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	repo := NewDeploymentRepository(db) // リポジトリを生成する

	k8sStatusJSON := datatypes.JSON([]byte(`{"replicas":1,"readyReplicas":1,"availableReplicas":1}`)) // テスト用 k8s_status JSON を生成する

	err := repo.UpdateK8sStatus(context.Background(), deploymentData.ID, k8sStatusJSON) // k8s_status を更新する
	if err != nil {
		t.Fatalf("UpdateK8sStatus がエラーを返しました: %v", err)
	}

	var fetchedDeployment models.Deployment
	db.First(&fetchedDeployment, "id = ?", deploymentData.ID) // 更新後のレコードを取得する
	if fetchedDeployment.K8sStatus == nil {                   // k8s_status が nil でないことを確認する
		t.Fatal("k8s_status が nil です")
	}

	// PostgreSQL は JSON を整形して返すため、デシリアライズして値を比較する
	var fetchedMap map[string]interface{}
	var expectedMap map[string]interface{}
	if err := json.Unmarshal(fetchedDeployment.K8sStatus, &fetchedMap); err != nil { // 取得値をデシリアライズする
		t.Fatalf("k8s_status のデシリアライズに失敗しました: %v", err)
	}
	if err := json.Unmarshal(k8sStatusJSON, &expectedMap); err != nil { // 期待値をデシリアライズする
		t.Fatalf("期待値のデシリアライズに失敗しました: %v", err)
	}
	for expectedKey, expectedValue := range expectedMap { // 各フィールドを比較する
		if fetchedMap[expectedKey] != expectedValue {
			t.Errorf("k8s_status[%s]: 期待値 %v, 実際の値 %v", expectedKey, expectedValue, fetchedMap[expectedKey])
		}
	}
}

// TestDeploymentRepository_UpdateK8sStatus_存在しないIDはエラーを返す は存在しない ID でエラーが返ることを確認する
func TestDeploymentRepository_UpdateK8sStatus_存在しないIDはエラーを返す(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する

	repo := NewDeploymentRepository(db) // リポジトリを生成する

	k8sStatusJSON := datatypes.JSON([]byte(`{"replicas":1}`)) // テスト用 k8s_status JSON を生成する

	err := repo.UpdateK8sStatus(context.Background(), "00000000-0000-0000-0000-000000000000", k8sStatusJSON) // 存在しない ID で更新する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("存在しない deploymentID でエラーが返るべきですが nil が返りました")
	}
}
