package repository

import (
	"handler/models"
	"context"
	"testing"
)

// TestVolumeRepository_Create_正常に作成される は volume レコードが作成されることを確認する
func TestVolumeRepository_Create_正常に作成される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	repo := NewVolumeRepository(db) // リポジトリを生成する
	volumeData := &models.Volume{
		ProjectID: projectData.ID,          // project_id を設定する
		Name:      "test-volume",           // ボリューム名を設定する
		SizeMB:    1024,                    // サイズを設定する
		Status:    models.VolumeStatusPending, // ステータスを設定する
	}

	err := repo.Create(context.Background(), db, volumeData) // リポジトリを実行する
	if err != nil {
		t.Fatalf("Create がエラーを返しました: %v", err)
	}
	if volumeData.ID == "" { // ID が付与されていることを確認する
		t.Error("作成後に ID が設定されていません")
	}
	t.Cleanup(func() { db.Unscoped().Delete(volumeData) }) // テスト終了後にレコードを削除する

	// DB から取得して値を確認する
	var fetchedVolume models.Volume
	db.First(&fetchedVolume, "id = ?", volumeData.ID) // 作成したレコードを取得する
	if fetchedVolume.Name != "test-volume" {           // name が一致することを確認する
		t.Errorf("期待する name: test-volume, 実際の name: %s", fetchedVolume.Name)
	}
	if fetchedVolume.SizeMB != 1024 { // size_mb が一致することを確認する
		t.Errorf("期待する size_mb: 1024, 実際の size_mb: %d", fetchedVolume.SizeMB)
	}
	if fetchedVolume.Status != models.VolumeStatusPending { // status が pending であることを確認する
		t.Errorf("期待する status: pending, 実際の status: %s", fetchedVolume.Status)
	}
}

// TestVolumeRepository_FindAllByProjectID_正常に一覧が取得される は FindAllByProjectID でプロジェクトの volume 一覧が取得されることを確認する
func TestVolumeRepository_FindAllByProjectID_正常に一覧が取得される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Volume を複数作成する
	volume1 := &models.Volume{ProjectID: projectData.ID, Name: "volume-a", SizeMB: 512, Status: models.VolumeStatusPending}  // 1件目
	volume2 := &models.Volume{ProjectID: projectData.ID, Name: "volume-b", SizeMB: 1024, Status: models.VolumeStatusPending} // 2件目
	if err := db.Create(volume1).Error; err != nil {
		t.Fatalf("テスト用 Volume 1 の作成に失敗しました: %v", err)
	}
	if err := db.Create(volume2).Error; err != nil {
		t.Fatalf("テスト用 Volume 2 の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { // テスト終了後にレコードを削除する
		db.Unscoped().Delete(volume1)
		db.Unscoped().Delete(volume2)
	})

	repo := NewVolumeRepository(db) // リポジトリを生成する

	result, err := repo.FindAllByProjectID(context.Background(), projectData.ID) // リポジトリを実行する
	if err != nil {
		t.Fatalf("FindAllByProjectID がエラーを返しました: %v", err)
	}
	if len(result) != 2 { // 2 件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(result))
	}

	// 名前が含まれることを確認する
	nameSet := make(map[string]bool)
	for _, volume := range result {
		nameSet[volume.Name] = true // 名前をセットに追加する
	}
	if !nameSet["volume-a"] { // volume-a が含まれることを確認する
		t.Error("volume-a が結果に含まれていません")
	}
	if !nameSet["volume-b"] { // volume-b が含まれることを確認する
		t.Error("volume-b が結果に含まれていません")
	}
}
