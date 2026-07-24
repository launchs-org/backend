package repository

import (
	"handler/models"
	"context"
	"testing"
)

// TestProjectRepository_DeleteNoTx_正常に削除される は DeleteNoTx で Project レコードが削除されることを確認する
func TestProjectRepository_DeleteNoTx_正常に削除される(t *testing.T) {
	db := setupTestDB(t)         // テスト用 DB を準備する
	ctx := context.Background() // テスト用コンテキストを生成する

	projectData := &models.Project{ // テスト用 Project レコードを生成する
		UserID:    "test-user-delete",         // テスト用ユーザー ID を設定する
		Name:      "test-project-delete",      // テスト用プロジェクト名を設定する
		Namespace: "test-namespace-delete",    // テスト用 namespace を設定する
		Status:    models.ProjectStatusActive, // ステータスを active に設定する
	}
	if err := db.Create(projectData).Error; err != nil { // テスト用 Project を作成する
		t.Fatalf("テスト用 Project の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(projectData) }) // テスト終了後に未削除の場合のクリーンアップ

	repo := NewProjectRepository(db) // repository を生成する

	if err := repo.DeleteNoTx(ctx, projectData); err != nil { // Project をトランザクション外で削除する
		t.Fatalf("DeleteNoTx() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	var deletedProject models.Project
	result := db.First(&deletedProject, "id = ?", projectData.ID) // 削除後に取得を試みる
	if result.Error == nil {                                        // エラーなし（レコードが残っている）の場合はテスト失敗
		t.Fatal("削除後も Project レコードが残っています")
	}
}

// TestProjectRepository_FindByNamespace_正常に取得される は FindByNamespace で Project が取得されることを確認する
func TestProjectRepository_FindByNamespace_正常に取得される(t *testing.T) {
	db := setupTestDB(t)        // テスト用 DB を準備する
	ctx := context.Background() // テスト用コンテキストを生成する

	projectData := &models.Project{ // テスト用 Project レコードを生成する
		UserID:    "test-user-ns",         // テスト用ユーザー ID を設定する
		Name:      "test-project-ns",      // テスト用プロジェクト名を設定する
		Namespace: "test-find-namespace",  // テスト用 namespace を設定する
		Status:    models.ProjectStatusActive, // ステータスを active に設定する
	}
	if err := db.Create(projectData).Error; err != nil { // テスト用 Project を作成する
		t.Fatalf("テスト用 Project の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(projectData) }) // テスト終了後にレコードを削除する

	repo := NewProjectRepository(db) // repository を生成する

	foundProject, err := repo.FindByNamespace(ctx, "test-find-namespace") // namespace で Project を取得する
	if err != nil {
		t.Fatalf("FindByNamespace() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}
	if foundProject.ID != projectData.ID { // 取得した Project の ID を確認する
		t.Errorf("期待する ID: %s, 実際の ID: %s", projectData.ID, foundProject.ID)
	}
}

// TestProjectRepository_FindByNamespace_存在しないnamespaceはエラーを返す は存在しない namespace で取得するとエラーになることを確認する
func TestProjectRepository_FindByNamespace_存在しないnamespaceはエラーを返す(t *testing.T) {
	db := setupTestDB(t)        // テスト用 DB を準備する
	ctx := context.Background() // テスト用コンテキストを生成する

	repo := NewProjectRepository(db) // repository を生成する

	_, err := repo.FindByNamespace(ctx, "non-existent-namespace") // 存在しない namespace で取得する
	if err == nil {                                                // エラーなしの場合はテスト失敗とする
		t.Fatal("存在しない namespace で FindByNamespace() はエラーを返すべきです")
	}
}
