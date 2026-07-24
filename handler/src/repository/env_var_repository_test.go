package repository

import (
	"handler/models"
	"context"
	"testing"
)

// TestEnvVarRepository_Create_正常に作成される は env_var レコードが作成されることを確認する
func TestEnvVarRepository_Create_正常に作成される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	repo := NewEnvVarRepository(db) // リポジトリを生成する
	envVarData := &models.EnvVar{
		ProjectID: projectData.ID,   // project_id を設定する
		Key:       "TEST_KEY",       // キーを設定する
		Value:     "test-value",     // 値を設定する
		IsSecret:  false,            // シークレットフラグを設定する
	}

	err := repo.Create(context.Background(), db, envVarData) // リポジトリを実行する
	if err != nil {
		t.Fatalf("Create がエラーを返しました: %v", err)
	}
	if envVarData.ID == "" { // ID が付与されていることを確認する
		t.Error("作成後に ID が設定されていません")
	}
	t.Cleanup(func() { db.Unscoped().Delete(envVarData) }) // テスト終了後にレコードを削除する

	// DB から取得して値を確認する
	var fetchedEnvVar models.EnvVar
	db.First(&fetchedEnvVar, "id = ?", envVarData.ID) // 作成したレコードを取得する
	if fetchedEnvVar.Key != "TEST_KEY" {               // key が一致することを確認する
		t.Errorf("期待する key: TEST_KEY, 実際の key: %s", fetchedEnvVar.Key)
	}
	if fetchedEnvVar.Value != "test-value" { // value が一致することを確認する
		t.Errorf("期待する value: test-value, 実際の value: %s", fetchedEnvVar.Value)
	}
	if fetchedEnvVar.IsSecret { // is_secret が false であることを確認する
		t.Error("期待する is_secret: false, 実際の is_secret: true")
	}
}

// TestEnvVarRepository_FindAllByProjectID_正常に一覧が取得される は FindAllByProjectID でプロジェクトの env_var 一覧が取得されることを確認する
func TestEnvVarRepository_FindAllByProjectID_正常に一覧が取得される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 EnvVar を複数作成する
	envVar1 := &models.EnvVar{ProjectID: projectData.ID, Key: "KEY_A", Value: "val-a", IsSecret: false} // 1件目
	envVar2 := &models.EnvVar{ProjectID: projectData.ID, Key: "KEY_B", Value: "val-b", IsSecret: true}  // 2件目
	if err := db.Create(envVar1).Error; err != nil {
		t.Fatalf("テスト用 EnvVar 1 の作成に失敗しました: %v", err)
	}
	if err := db.Create(envVar2).Error; err != nil {
		t.Fatalf("テスト用 EnvVar 2 の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { // テスト終了後にレコードを削除する
		db.Unscoped().Delete(envVar1)
		db.Unscoped().Delete(envVar2)
	})

	repo := NewEnvVarRepository(db) // リポジトリを生成する

	result, err := repo.FindAllByProjectID(context.Background(), projectData.ID) // リポジトリを実行する
	if err != nil {
		t.Fatalf("FindAllByProjectID がエラーを返しました: %v", err)
	}
	if len(result) != 2 { // 2 件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(result))
	}

	// キーが含まれることを確認する
	keySet := make(map[string]bool)
	for _, envVar := range result {
		keySet[envVar.Key] = true // キーをセットに追加する
	}
	if !keySet["KEY_A"] { // KEY_A が含まれることを確認する
		t.Error("KEY_A が結果に含まれていません")
	}
	if !keySet["KEY_B"] { // KEY_B が含まれることを確認する
		t.Error("KEY_B が結果に含まれていません")
	}
}
