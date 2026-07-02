package repository

import (
	"handler/models"
	"context"
	"testing"
)

// TestPathRuleRepository_Create_正常に作成される は PathRule レコードが作成されることを確認する
func TestPathRuleRepository_Create_正常に作成される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	ingressRouteData := &models.IngressRoute{
		ProjectID: projectData.ID,                   // project_id を設定する
		Host:      "path-rule-test.launchs.org",     // ホスト名を設定する
		Status:    models.IngressRouteStatusPending, // ステータスを設定する
	}
	if err := db.Create(ingressRouteData).Error; err != nil { // テスト用 IngressRoute を作成する
		t.Fatalf("テスト用 IngressRoute の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(ingressRouteData) }) // テスト終了後にレコードを削除する

	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,                 // project_id を設定する
		Name:      "test-app-path-rule",           // デプロイメント名を設定する
		Type:      models.DeploymentTypeImageURL,  // タイプを設定する
		Status:    models.DeploymentStatusPending, // ステータスを設定する
		AppStatus: models.AppStatusPending,        // アプリステータスを設定する
	}
	if err := db.Create(deploymentData).Error; err != nil { // テスト用 Deployment を作成する
		t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	serviceData := &models.Service{
		DeploymentID: deploymentData.ID,           // deployment_id を設定する
		Status:       models.ServiceStatusPending, // ステータスを設定する
	}
	if err := db.Create(serviceData).Error; err != nil { // テスト用 Service を作成する
		t.Fatalf("テスト用 Service の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(serviceData) }) // テスト終了後にレコードを削除する

	repo := NewPathRuleRepository(db) // リポジトリを生成する
	pathRuleData := &models.PathRule{
		IngressRouteID: ingressRouteData.ID,          // ingress_route_id を設定する
		PathPrefix:     "/api",                       // パスプレフィックスを設定する
		ServiceID:      serviceData.ID,               // service_id を設定する
		Status:         models.PathRuleStatusPending, // ステータスを設定する
	}

	err := repo.Create(context.Background(), nil, pathRuleData) // リポジトリを実行する
	if err != nil {
		t.Fatalf("Create がエラーを返しました: %v", err)
	}
	if pathRuleData.ID == "" { // ID が付与されていることを確認する
		t.Error("作成後に ID が設定されていません")
	}
	t.Cleanup(func() { db.Unscoped().Delete(pathRuleData) }) // テスト終了後にレコードを削除する

	var fetchedPathRule models.PathRule
	db.First(&fetchedPathRule, "id = ?", pathRuleData.ID) // 作成したレコードを取得する
	if fetchedPathRule.PathPrefix != "/api" {               // path_prefix が一致することを確認する
		t.Errorf("期待する path_prefix: /api, 実際の path_prefix: %s", fetchedPathRule.PathPrefix)
	}
	if fetchedPathRule.Status != models.PathRuleStatusPending { // status が pending であることを確認する
		t.Errorf("期待する status: pending, 実際の status: %s", fetchedPathRule.Status)
	}
}

// TestPathRuleRepository_FindByIngressRouteID_正常に取得される は FindByIngressRouteID で path_rule 一覧が取得されることを確認する
func TestPathRuleRepository_FindByIngressRouteID_正常に取得される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	ingressRouteData := &models.IngressRoute{
		ProjectID: projectData.ID,                   // project_id を設定する
		Host:      "find-path-rule.launchs.org",     // ホスト名を設定する
		Status:    models.IngressRouteStatusPending, // ステータスを設定する
	}
	if err := db.Create(ingressRouteData).Error; err != nil { // テスト用 IngressRoute を作成する
		t.Fatalf("テスト用 IngressRoute の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(ingressRouteData) }) // テスト終了後にレコードを削除する

	deploymentNameList := []string{"find-pathrule-dep-1", "find-pathrule-dep-2", "find-pathrule-dep-3"} // 各 Service 用の Deployment 名一覧を定義する
	serviceList := make([]*models.Service, 3)                                                          // 3つの Service を作成する
	for serviceIndex := 0; serviceIndex < 3; serviceIndex++ {
		deploymentData := &models.Deployment{
			ProjectID: projectData.ID,                              // project_id を設定する
			Name:      deploymentNameList[serviceIndex],            // デプロイメント名を設定する
			Type:      models.DeploymentTypeImageURL,               // タイプを設定する
			Status:    models.DeploymentStatusPending,              // ステータスを設定する
			AppStatus: models.AppStatusPending,                     // アプリステータスを設定する
		}
		if err := db.Create(deploymentData).Error; err != nil { // テスト用 Deployment を作成する
			t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err)
		}
		capturedDeployment := deploymentData                                // クロージャキャプチャを固定する
		t.Cleanup(func() { db.Unscoped().Delete(capturedDeployment) })     // テスト終了後に削除する

		serviceData := &models.Service{
			DeploymentID: deploymentData.ID,           // deployment_id を設定する
			Status:       models.ServiceStatusPending, // ステータスを設定する
		}
		if err := db.Create(serviceData).Error; err != nil { // Service を作成する
			t.Fatalf("テスト用 Service の作成に失敗しました: %v", err)
		}
		capturedService := serviceData                                 // クロージャキャプチャを固定する
		t.Cleanup(func() { db.Unscoped().Delete(capturedService) })   // テスト終了後に削除する
		serviceList[serviceIndex] = serviceData
	}

	pathRuleStatusList := []models.PathRuleStatus{
		models.PathRuleStatusActive,
		models.PathRuleStatusPending,
		models.PathRuleStatusDeleting,
	}
	pathPrefixList := []string{"/api", "/web", "/admin"} // パスプレフィックス一覧を定義する

	for ruleIndex := 0; ruleIndex < 3; ruleIndex++ { // PathRule を順に作成する
		pathRuleData := &models.PathRule{
			IngressRouteID: ingressRouteData.ID,             // ingress_route_id を設定する
			PathPrefix:     pathPrefixList[ruleIndex],       // パスプレフィックスを設定する
			ServiceID:      serviceList[ruleIndex].ID,       // service_id を設定する
			Status:         pathRuleStatusList[ruleIndex],   // ステータスを設定する
		}
		if err := db.Create(pathRuleData).Error; err != nil {
			t.Fatalf("テスト用 PathRule の作成に失敗しました: %v", err)
		}
		capturedPathRule := pathRuleData                               // クロージャキャプチャを固定する
		t.Cleanup(func() { db.Unscoped().Delete(capturedPathRule) })   // テスト終了後に削除する
	}

	repo := NewPathRuleRepository(db) // リポジトリを生成する

	allResult, err := repo.FindByIngressRouteID(context.Background(), ingressRouteData.ID) // 全件取得する
	if err != nil {
		t.Fatalf("FindByIngressRouteID がエラーを返しました: %v", err)
	}
	if len(allResult) != 3 { // 3件取得できることを確認する
		t.Errorf("期待する件数: 3, 実際の件数: %d", len(allResult))
	}

	activePendingResult, err := repo.FindActiveAndPendingByIngressRouteID(context.Background(), ingressRouteData.ID) // active/pending を取得する
	if err != nil {
		t.Fatalf("FindActiveAndPendingByIngressRouteID がエラーを返しました: %v", err)
	}
	if len(activePendingResult) != 2 { // 2件（active + pending）のみ取得されることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(activePendingResult))
	}
	for _, pathRule := range activePendingResult { // deleting が含まれていないことを確認する
		if pathRule.Status == models.PathRuleStatusDeleting {
			t.Errorf("FindActiveAndPendingByIngressRouteID に deleting が含まれています: %s", pathRule.PathPrefix)
		}
	}
}

// TestPathRuleRepository_UpdateStatus_正常に更新される は UpdateStatus で status が更新されることを確認する
func TestPathRuleRepository_UpdateStatus_正常に更新される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	ingressRouteData := &models.IngressRoute{
		ProjectID: projectData.ID,                   // project_id を設定する
		Host:      "status-update-test.launchs.org", // ホスト名を設定する
		Status:    models.IngressRouteStatusPending, // ステータスを設定する
	}
	if err := db.Create(ingressRouteData).Error; err != nil {
		t.Fatalf("テスト用 IngressRoute の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(ingressRouteData) })

	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,                 // project_id を設定する
		Name:      "status-update-deployment",     // デプロイメント名を設定する
		Type:      models.DeploymentTypeImageURL,  // タイプを設定する
		Status:    models.DeploymentStatusPending, // ステータスを設定する
		AppStatus: models.AppStatusPending,        // アプリステータスを設定する
	}
	if err := db.Create(deploymentData).Error; err != nil {
		t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) })

	serviceData := &models.Service{
		DeploymentID: deploymentData.ID,           // deployment_id を設定する
		Status:       models.ServiceStatusPending, // ステータスを設定する
	}
	if err := db.Create(serviceData).Error; err != nil {
		t.Fatalf("テスト用 Service の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(serviceData) })

	pathRuleData := &models.PathRule{
		IngressRouteID: ingressRouteData.ID,          // ingress_route_id を設定する
		PathPrefix:     "/test",                      // パスプレフィックスを設定する
		ServiceID:      serviceData.ID,               // service_id を設定する
		Status:         models.PathRuleStatusPending, // 初期ステータスを pending に設定する
	}
	if err := db.Create(pathRuleData).Error; err != nil {
		t.Fatalf("テスト用 PathRule の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(pathRuleData) })

	repo := NewPathRuleRepository(db) // リポジトリを生成する

	if err := repo.UpdateStatus(context.Background(), nil, pathRuleData.ID, models.PathRuleStatusActive); err != nil { // status を active に更新する
		t.Fatalf("UpdateStatus がエラーを返しました: %v", err)
	}

	var fetchedPathRule models.PathRule
	db.First(&fetchedPathRule, "id = ?", pathRuleData.ID) // 更新後のレコードを取得する
	if fetchedPathRule.Status != models.PathRuleStatusActive { // status が active になっていることを確認する
		t.Errorf("期待する status: active, 実際の status: %s", fetchedPathRule.Status)
	}
}

// TestPathRuleRepository_Delete_正常に物理削除される は Delete で PathRule が物理削除されることを確認する
func TestPathRuleRepository_Delete_正常に物理削除される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	ingressRouteData := &models.IngressRoute{
		ProjectID: projectData.ID,                   // project_id を設定する
		Host:      "delete-test.launchs.org",        // ホスト名を設定する
		Status:    models.IngressRouteStatusPending, // ステータスを設定する
	}
	if err := db.Create(ingressRouteData).Error; err != nil {
		t.Fatalf("テスト用 IngressRoute の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(ingressRouteData) })

	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,                 // project_id を設定する
		Name:      "delete-pathrule-deployment",   // デプロイメント名を設定する
		Type:      models.DeploymentTypeImageURL,  // タイプを設定する
		Status:    models.DeploymentStatusPending, // ステータスを設定する
		AppStatus: models.AppStatusPending,        // アプリステータスを設定する
	}
	if err := db.Create(deploymentData).Error; err != nil {
		t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) })

	serviceData := &models.Service{
		DeploymentID: deploymentData.ID,           // deployment_id を設定する
		Status:       models.ServiceStatusPending, // ステータスを設定する
	}
	if err := db.Create(serviceData).Error; err != nil {
		t.Fatalf("テスト用 Service の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(serviceData) })

	pathRuleData := &models.PathRule{
		IngressRouteID: ingressRouteData.ID,           // ingress_route_id を設定する
		PathPrefix:     "/delete-me",                  // パスプレフィックスを設定する
		ServiceID:      serviceData.ID,                // service_id を設定する
		Status:         models.PathRuleStatusDeleting, // ステータスを deleting に設定する
	}
	if err := db.Create(pathRuleData).Error; err != nil {
		t.Fatalf("テスト用 PathRule の作成に失敗しました: %v", err)
	}

	repo := NewPathRuleRepository(db) // リポジトリを生成する

	if err := repo.Delete(context.Background(), nil, pathRuleData.ID); err != nil { // 物理削除を実行する
		t.Fatalf("Delete がエラーを返しました: %v", err)
	}

	var fetchedPathRule models.PathRule
	result := db.First(&fetchedPathRule, "id = ?", pathRuleData.ID) // 削除後に取得を試みる
	if result.Error == nil {                                          // レコードが存在する場合はテスト失敗とする
		t.Error("Delete 後もレコードが存在しています")
	}
}
