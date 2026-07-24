package repository

import (
	"handler/models"
	"context"
	"testing"
)

// TestVolumeMountRepository_FindAllByDeploymentID_正常に一覧が取得される は FindAllByDeploymentID でマウント設定一覧が取得されることを確認する
func TestVolumeMountRepository_FindAllByDeploymentID_正常に一覧が取得される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	deploymentData := &models.Deployment{ // テスト用 Deployment を作成する
		ProjectID: projectData.ID,                 // プロジェクト ID を設定する
		Name:      "test-deployment-for-vmount",   // デプロイメント名を設定する
		Type:      models.DeploymentTypeImageURL,  // タイプを設定する
		Status:    models.DeploymentStatusPending, // ステータスを設定する
		AppStatus: models.AppStatusPending,        // アプリステータスを設定する
	}
	if err := db.Create(deploymentData).Error; err != nil {
		t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	// テスト用 Volume を作成する
	volumeData := &models.Volume{
		ProjectID: projectData.ID,             // project_id を設定する
		Name:      "test-volume-for-mount",    // ボリューム名を設定する
		SizeMB:    512,                        // サイズを設定する
		Status:    models.VolumeStatusPending, // ステータスを設定する
	}
	if err := db.Create(volumeData).Error; err != nil {
		t.Fatalf("テスト用 Volume の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(volumeData) }) // テスト終了後にレコードを削除する

	// テスト用 VolumeMount を複数作成する
	mount1 := &models.VolumeMount{
		DeploymentID: deploymentData.ID,               // deployment_id を設定する
		VolumeID:     volumeData.ID,                   // volume_id を設定する
		MountPath:    "/data",                         // マウントパスを設定する
		Status:       models.VolumeMountStatusPending, // ステータスを設定する
	}
	mount2 := &models.VolumeMount{
		DeploymentID: deploymentData.ID,               // deployment_id を設定する
		VolumeID:     volumeData.ID,                   // volume_id を設定する
		MountPath:    "/logs",                         // マウントパスを設定する
		Status:       models.VolumeMountStatusPending, // ステータスを設定する
	}
	if err := db.Create(mount1).Error; err != nil {
		t.Fatalf("テスト用 VolumeMount 1 の作成に失敗しました: %v", err)
	}
	if err := db.Create(mount2).Error; err != nil {
		t.Fatalf("テスト用 VolumeMount 2 の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { // テスト終了後にレコードを削除する
		db.Unscoped().Delete(mount1)
		db.Unscoped().Delete(mount2)
	})

	repo := NewVolumeMountRepository(db) // リポジトリを生成する

	result, err := repo.FindAllByDeploymentID(context.Background(), deploymentData.ID) // リポジトリを実行する
	if err != nil {
		t.Fatalf("FindAllByDeploymentID がエラーを返しました: %v", err)
	}
	if len(result) != 2 { // 2 件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(result))
	}

	// マウントパスが含まれることを確認する
	mountPathSet := make(map[string]bool)
	for _, mountItem := range result {
		mountPathSet[mountItem.MountPath] = true // マウントパスをセットに追加する
	}
	if !mountPathSet["/data"] { // /data が含まれることを確認する
		t.Error("/data が結果に含まれていません")
	}
	if !mountPathSet["/logs"] { // /logs が含まれることを確認する
		t.Error("/logs が結果に含まれていません")
	}
}
