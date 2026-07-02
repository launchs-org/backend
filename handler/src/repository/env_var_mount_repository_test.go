package repository

import (
	"handler/models"
	"context"
	"testing"
)

// TestEnvVarMountRepository_FindAllByDeploymentID_正常に一覧が取得される は FindAllByDeploymentID でマウント設定一覧が取得されることを確認する
func TestEnvVarMountRepository_FindAllByDeploymentID_正常に一覧が取得される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Deployment を作成する
	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,                 // プロジェクト ID を設定する
		Name:      "test-app-mount",               // デプロイメント名を設定する
		Type:      models.DeploymentTypeImageURL,  // タイプを設定する
		Status:    models.DeploymentStatusPending, // ステータスを設定する
		AppStatus: models.AppStatusPending,        // アプリステータスを設定する
	}
	if err := db.Create(deploymentData).Error; err != nil {
		t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	// テスト用 EnvVar を作成する
	envVarData := &models.EnvVar{
		ProjectID: projectData.ID, // プロジェクト ID を設定する
		Key:       "MOUNT_TEST",   // キーを設定する
		Value:     "mount-value",  // 値を設定する
		IsSecret:  false,          // シークレットフラグを設定する
	}
	if err := db.Create(envVarData).Error; err != nil {
		t.Fatalf("テスト用 EnvVar の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(envVarData) }) // テスト終了後にレコードを削除する

	// テスト用 EnvVarMount を作成する
	mountData := &models.EnvVarMount{
		DeploymentID: deploymentData.ID,              // deployment ID を設定する
		EnvVarID:     envVarData.ID,                  // env_var ID を設定する
		Status:       models.EnvVarMountStatusPending, // ステータスを設定する
	}
	if err := db.Create(mountData).Error; err != nil {
		t.Fatalf("テスト用 EnvVarMount の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(mountData) }) // テスト終了後にレコードを削除する

	repo := NewEnvVarMountRepository(db) // リポジトリを生成する

	result, err := repo.FindAllByDeploymentID(context.Background(), deploymentData.ID) // 一覧を取得する
	if err != nil {
		t.Fatalf("FindAllByDeploymentID がエラーを返しました: %v", err)
	}
	if len(result) != 1 { // 1 件返ることを確認する
		t.Errorf("期待する件数: 1, 実際の件数: %d", len(result))
	}
	if result[0].EnvVarID != envVarData.ID { // env_var ID が一致することを確認する
		t.Errorf("期待する env_var_id: %s, 実際の env_var_id: %s", envVarData.ID, result[0].EnvVarID)
	}
}

// TestEnvVarMountRepository_FindAllByDeploymentID_他のDeploymentのマウントは取得されない は別 deployment のマウント設定が取得されないことを確認する
func TestEnvVarMountRepository_FindAllByDeploymentID_他のDeploymentのマウントは取得されない(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// 2 つの Deployment を作成する
	deployment1 := &models.Deployment{
		ProjectID: projectData.ID, Name: "dep-1", Type: models.DeploymentTypeImageURL,
		Status: models.DeploymentStatusPending, AppStatus: models.AppStatusPending,
	}
	deployment2 := &models.Deployment{
		ProjectID: projectData.ID, Name: "dep-2", Type: models.DeploymentTypeImageURL,
		Status: models.DeploymentStatusPending, AppStatus: models.AppStatusPending,
	}
	if err := db.Create(deployment1).Error; err != nil {
		t.Fatalf("テスト用 Deployment1 の作成に失敗しました: %v", err)
	}
	if err := db.Create(deployment2).Error; err != nil {
		t.Fatalf("テスト用 Deployment2 の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(deployment1); db.Unscoped().Delete(deployment2) })

	// テスト用 EnvVar を作成する
	envVarData := &models.EnvVar{
		ProjectID: projectData.ID, Key: "KEY_SEPARATE", Value: "val", IsSecret: false,
	}
	if err := db.Create(envVarData).Error; err != nil {
		t.Fatalf("テスト用 EnvVar の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(envVarData) })

	// deployment2 にのみマウント設定を追加する
	mount2 := &models.EnvVarMount{
		DeploymentID: deployment2.ID, EnvVarID: envVarData.ID, Status: models.EnvVarMountStatusPending,
	}
	if err := db.Create(mount2).Error; err != nil {
		t.Fatalf("テスト用 EnvVarMount の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(mount2) })

	repo := NewEnvVarMountRepository(db) // リポジトリを生成する

	result, err := repo.FindAllByDeploymentID(context.Background(), deployment1.ID) // deployment1 の一覧を取得する
	if err != nil {
		t.Fatalf("FindAllByDeploymentID がエラーを返しました: %v", err)
	}
	if len(result) != 0 { // 0 件であることを確認する
		t.Errorf("他の deployment のマウント設定が含まれています: 件数 %d", len(result))
	}
}
