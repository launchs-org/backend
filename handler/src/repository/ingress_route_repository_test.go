package repository

import (
	"handler/models"
	"context"
	"testing"
)

// TestIngressRouteRepository_Create_正常に作成される は IngressRoute レコードが作成されることを確認する
func TestIngressRouteRepository_Create_正常に作成される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	repo := NewIngressRouteRepository(db)    // リポジトリを生成する
	ingressRouteData := &models.IngressRoute{
		ProjectID: projectData.ID,                    // project_id を設定する
		Host:      "example.launchs.org",             // ホスト名を設定する
		Status:    models.IngressRouteStatusPending,  // ステータスを設定する
	}

	err := repo.Create(context.Background(), nil, ingressRouteData) // リポジトリを実行する
	if err != nil {
		t.Fatalf("Create がエラーを返しました: %v", err)
	}
	if ingressRouteData.ID == "" { // ID が付与されていることを確認する
		t.Error("作成後に ID が設定されていません")
	}
	t.Cleanup(func() { db.Unscoped().Delete(ingressRouteData) }) // テスト終了後にレコードを削除する

	// DB から取得して値を確認する
	var fetchedIngressRoute models.IngressRoute
	db.First(&fetchedIngressRoute, "id = ?", ingressRouteData.ID) // 作成したレコードを取得する
	if fetchedIngressRoute.Host != "example.launchs.org" {         // host が一致することを確認する
		t.Errorf("期待する host: example.launchs.org, 実際の host: %s", fetchedIngressRoute.Host)
	}
	if fetchedIngressRoute.ProjectID != projectData.ID { // project_id が一致することを確認する
		t.Errorf("期待する project_id: %s, 実際の project_id: %s", projectData.ID, fetchedIngressRoute.ProjectID)
	}
	if fetchedIngressRoute.Status != models.IngressRouteStatusPending { // status が pending であることを確認する
		t.Errorf("期待する status: pending, 実際の status: %s", fetchedIngressRoute.Status)
	}
}

// TestIngressRouteRepository_FindByProjectID_正常に取得される は FindByProjectID で ingress_route が取得されることを確認する
func TestIngressRouteRepository_FindByProjectID_正常に取得される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 IngressRoute を直接 DB に作成する
	ingressRouteData := &models.IngressRoute{
		ProjectID: projectData.ID,                    // project_id を設定する
		Host:      "find-test.launchs.org",           // ホスト名を設定する
		Status:    models.IngressRouteStatusActive,   // ステータスを設定する
	}
	if err := db.Create(ingressRouteData).Error; err != nil { // テスト用レコードを作成する
		t.Fatalf("テスト用 IngressRoute の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(ingressRouteData) }) // テスト終了後にレコードを削除する

	repo := NewIngressRouteRepository(db) // リポジトリを生成する

	result, err := repo.FindByProjectID(context.Background(), projectData.ID) // リポジトリを実行する
	if err != nil {
		t.Fatalf("FindByProjectID がエラーを返しました: %v", err)
	}
	if result.Host != "find-test.launchs.org" { // host が一致することを確認する
		t.Errorf("期待する host: find-test.launchs.org, 実際の host: %s", result.Host)
	}
	if result.Status != models.IngressRouteStatusActive { // status が一致することを確認する
		t.Errorf("期待する status: active, 実際の status: %s", result.Status)
	}
}

// TestIngressRouteRepository_FindAllByProjectID_複数件取得される は FindAllByProjectID で複数の ingress_route が取得されることを確認する
func TestIngressRouteRepository_FindAllByProjectID_複数件取得される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	repo := NewIngressRouteRepository(db) // リポジトリを生成する

	firstIngressRoute := &models.IngressRoute{
		ProjectID: projectData.ID,                   // project_id を設定する
		Host:      "first.launchs.org",              // ホスト名を設定する
		Status:    models.IngressRouteStatusPending, // ステータスを設定する
	}
	if err := repo.Create(context.Background(), nil, firstIngressRoute); err != nil { // 1つ目を作成する
		t.Fatalf("1つ目の Create がエラーを返しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(firstIngressRoute) }) // テスト終了後にレコードを削除する

	secondIngressRoute := &models.IngressRoute{
		ProjectID: projectData.ID,                   // 同じ project_id に2つ目を作成する
		Host:      "second.launchs.org",             // 別のホスト名を設定する
		Status:    models.IngressRouteStatusPending, // ステータスを設定する
	}
	if err := repo.Create(context.Background(), nil, secondIngressRoute); err != nil { // 2つ目を作成する（エラーにならないはず）
		t.Fatalf("2つ目の Create がエラーを返しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(secondIngressRoute) }) // テスト終了後にレコードを削除する

	ingressRouteList, err := repo.FindAllByProjectID(context.Background(), projectData.ID) // 全件取得する
	if err != nil {
		t.Fatalf("FindAllByProjectID がエラーを返しました: %v", err)
	}
	if len(ingressRouteList) != 2 { // 2件取得されることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(ingressRouteList))
	}
}
