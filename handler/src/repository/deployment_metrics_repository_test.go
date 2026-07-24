package repository

import (
	"handler/models"
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
)

// createTestDeploymentForMetrics はテスト用の Deployment レコードを作成するヘルパー関数
func createTestDeploymentForMetrics(t *testing.T, db *gorm.DB, projectID string) *models.Deployment {
	t.Helper()
	deploymentData := &models.Deployment{
		ProjectID: projectID,                        // プロジェクト ID を設定する
		Name:      "test-deployment",                // デプロイメント名を設定する
		Type:      models.DeploymentTypeImageURL,    // タイプを設定する
		Status:    models.DeploymentStatusRunning,   // ステータスを設定する
		AppStatus: models.AppStatusRunning,          // アプリステータスを設定する
	}
	if err := db.Create(deploymentData).Error; err != nil {
		t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err) // 作成失敗時はテスト失敗とする
	}
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する
	return deploymentData
}

// TestDeploymentMetricsRepository_CreateBatch_正常に保存される は CreateBatch で複数レコードが保存されることを確認する
func TestDeploymentMetricsRepository_CreateBatch_正常に保存される(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する

	// DeploymentMetrics テーブルをマイグレーションする
	if err := db.AutoMigrate(&models.DeploymentMetrics{}); err != nil {
		t.Fatalf("DeploymentMetrics マイグレーションに失敗しました: %v", err)
	}

	projectData := createTestProject(t, db)  // テスト用 Project を作成する
	deploymentData := createTestDeploymentForMetrics(t, db, projectData.ID) // テスト用 Deployment を作成する

	repo := NewDeploymentMetricsRepository(db) // リポジトリを生成する
	recordedAt := time.Now().Truncate(time.Second)  // 記録日時を設定する

	metricsList := []*models.DeploymentMetrics{
		{
			DeploymentID:  deploymentData.ID,        // Deployment ID を設定する
			PodName:       "test-pod-1",             // Pod 名を設定する
			CPUMillicores: 100,                      // CPU 使用量を設定する
			MemoryBytes:   104857600,                // メモリ使用量を設定する（100MB）
			ReadyReplicas: 1,                        // Ready レプリカ数を設定する
			TotalReplicas: 1,                        // 合計レプリカ数を設定する
			RecordedAt:    recordedAt,               // 記録日時を設定する
		},
		{
			DeploymentID:  deploymentData.ID,        // Deployment ID を設定する
			PodName:       "test-pod-2",             // Pod 名を設定する
			CPUMillicores: 200,                      // CPU 使用量を設定する
			MemoryBytes:   209715200,                // メモリ使用量を設定する（200MB）
			ReadyReplicas: 2,                        // Ready レプリカ数を設定する
			TotalReplicas: 2,                        // 合計レプリカ数を設定する
			RecordedAt:    recordedAt,               // 記録日時を設定する
		},
	}

	err := repo.CreateBatch(context.Background(), metricsList) // 一括保存を実行する
	if err != nil {
		t.Fatalf("CreateBatch がエラーを返しました: %v", err)
	}
	t.Cleanup(func() { // テスト終了後にレコードを削除する
		for _, metricsRecord := range metricsList {
			db.Unscoped().Delete(metricsRecord)
		}
	})

	// DB から取得して件数を確認する
	var count int64
	db.Model(&models.DeploymentMetrics{}).Where("deployment_id = ?", deploymentData.ID).Count(&count) // 保存された件数を確認する
	if count != 2 { // 2 件保存されていることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", count)
	}
}

// TestDeploymentMetricsRepository_FindByDeploymentID_RecordedAt降順で取得される は RecordedAt DESC 順・件数制限が効くことを確認する
func TestDeploymentMetricsRepository_FindByDeploymentID_RecordedAt降順で取得される(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する

	// DeploymentMetrics テーブルをマイグレーションする
	if err := db.AutoMigrate(&models.DeploymentMetrics{}); err != nil {
		t.Fatalf("DeploymentMetrics マイグレーションに失敗しました: %v", err)
	}

	projectData := createTestProject(t, db)  // テスト用 Project を作成する
	deploymentData := createTestDeploymentForMetrics(t, db, projectData.ID) // テスト用 Deployment を作成する

	now := time.Now().Truncate(time.Second)  // 現在時刻を取得する

	// 同一 Pod（pod-a）について異なる RecordedAt で 3 件のメトリクスを保存する
	metricsList := []*models.DeploymentMetrics{
		{DeploymentID: deploymentData.ID, PodName: "pod-a", CPUMillicores: 100, MemoryBytes: 100, ReadyReplicas: 1, TotalReplicas: 1, RecordedAt: now.Add(-60 * time.Second)}, // 最古のレコード
		{DeploymentID: deploymentData.ID, PodName: "pod-a", CPUMillicores: 200, MemoryBytes: 200, ReadyReplicas: 1, TotalReplicas: 1, RecordedAt: now.Add(-30 * time.Second)}, // 中間のレコード
		{DeploymentID: deploymentData.ID, PodName: "pod-a", CPUMillicores: 300, MemoryBytes: 300, ReadyReplicas: 1, TotalReplicas: 1, RecordedAt: now},                        // 最新のレコード
	}
	db.Create(&metricsList) // テスト用レコードを作成する
	t.Cleanup(func() { // テスト終了後にレコードを削除する
		for _, metricsRecord := range metricsList {
			db.Unscoped().Delete(metricsRecord)
		}
	})

	repo := NewDeploymentMetricsRepository(db) // リポジトリを生成する

	// limit=2 で取得して件数と順序を確認する
	result, err := repo.FindByDeploymentID(context.Background(), deploymentData.ID, 2) // 上位 2 件を取得する
	if err != nil {
		t.Fatalf("FindByDeploymentID がエラーを返しました: %v", err)
	}
	if len(result) != 2 { // 2 件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(result))
	}
	if result[0].CPUMillicores != 300 { // 最新レコードが先頭であることを確認する
		t.Errorf("1 件目は最新レコード（CPU=300）のはずですが、実際は CPU=%d", result[0].CPUMillicores)
	}
	if result[1].CPUMillicores != 200 { // 2 番目のレコードを確認する
		t.Errorf("2 件目は中間レコード（CPU=200）のはずですが、実際は CPU=%d", result[1].CPUMillicores)
	}
}

// TestDeploymentMetricsRepository_FindByDeploymentID_消えたPodのレコードは除外される は直近のポーリングに存在しない Pod のレコードが結果から除外されることを確認する
func TestDeploymentMetricsRepository_FindByDeploymentID_消えたPodのレコードは除外される(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する

	// DeploymentMetrics テーブルをマイグレーションする
	if err := db.AutoMigrate(&models.DeploymentMetrics{}); err != nil {
		t.Fatalf("DeploymentMetrics マイグレーションに失敗しました: %v", err)
	}

	projectData := createTestProject(t, db)                                // テスト用 Project を作成する
	deploymentData := createTestDeploymentForMetrics(t, db, projectData.ID) // テスト用 Deployment を作成する

	now := time.Now().Truncate(time.Second) // 現在時刻を取得する

	// pod-old は過去にのみ存在し、直近のポーリング（now）には含まれない Pod とする
	metricsList := []*models.DeploymentMetrics{
		{DeploymentID: deploymentData.ID, PodName: "pod-old", CPUMillicores: 100, MemoryBytes: 100, ReadyReplicas: 2, TotalReplicas: 2, RecordedAt: now.Add(-60 * time.Second)}, // スケールダウンで消えた Pod の過去レコード
		{DeploymentID: deploymentData.ID, PodName: "pod-new", CPUMillicores: 200, MemoryBytes: 200, ReadyReplicas: 1, TotalReplicas: 1, RecordedAt: now.Add(-60 * time.Second)}, // 直近まで生存している Pod の過去レコード
		{DeploymentID: deploymentData.ID, PodName: "pod-new", CPUMillicores: 300, MemoryBytes: 300, ReadyReplicas: 1, TotalReplicas: 1, RecordedAt: now},                        // 直近のポーリングで取得された最新レコード
	}
	db.Create(&metricsList) // テスト用レコードを作成する
	t.Cleanup(func() {      // テスト終了後にレコードを削除する
		for _, metricsRecord := range metricsList {
			db.Unscoped().Delete(metricsRecord)
		}
	})

	repo := NewDeploymentMetricsRepository(db) // リポジトリを生成する

	result, err := repo.FindByDeploymentID(context.Background(), deploymentData.ID, 10) // 全件取得する
	if err != nil {
		t.Fatalf("FindByDeploymentID がエラーを返しました: %v", err)
	}
	if len(result) != 2 { // pod-new の 2 件のみ返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(result))
	}
	for _, metricsRecord := range result { // 全レコードが pod-new であることを確認する
		if metricsRecord.PodName != "pod-new" {
			t.Errorf("消えた Pod（pod-old）のレコードが含まれています: %+v", metricsRecord)
		}
	}
}

// TestDeploymentMetricsRepository_DeleteOlderThan_古いレコードが削除される は指定日時より古いレコードのみ削除されることを確認する
func TestDeploymentMetricsRepository_DeleteOlderThan_古いレコードが削除される(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する

	// DeploymentMetrics テーブルをマイグレーションする
	if err := db.AutoMigrate(&models.DeploymentMetrics{}); err != nil {
		t.Fatalf("DeploymentMetrics マイグレーションに失敗しました: %v", err)
	}

	projectData := createTestProject(t, db)  // テスト用 Project を作成する
	deploymentData := createTestDeploymentForMetrics(t, db, projectData.ID) // テスト用 Deployment を作成する

	now := time.Now().Truncate(time.Second) // 現在時刻を取得する
	cutoff := now.Add(-7 * 24 * time.Hour) // 7 日前の日時を設定する

	// 7 日より前のレコード（削除対象）と 7 日以内のレコード（保持対象）を作成する
	oldRecord := &models.DeploymentMetrics{
		DeploymentID: deploymentData.ID, PodName: "pod-old",
		CPUMillicores: 100, MemoryBytes: 100, ReadyReplicas: 1, TotalReplicas: 1,
		RecordedAt: cutoff.Add(-1 * time.Second), // 7 日前より 1 秒古いレコード（削除対象）
	}
	recentRecord := &models.DeploymentMetrics{
		DeploymentID: deploymentData.ID, PodName: "pod-recent",
		CPUMillicores: 200, MemoryBytes: 200, ReadyReplicas: 1, TotalReplicas: 1,
		RecordedAt: cutoff.Add(1 * time.Second), // 7 日前より 1 秒新しいレコード（保持対象）
	}
	db.Create(oldRecord)    // 削除対象レコードを作成する
	db.Create(recentRecord) // 保持対象レコードを作成する
	t.Cleanup(func() {      // テスト終了後に残存レコードを削除する
		db.Unscoped().Delete(oldRecord)
		db.Unscoped().Delete(recentRecord)
	})

	repo := NewDeploymentMetricsRepository(db)                            // リポジトリを生成する
	err := repo.DeleteOlderThan(context.Background(), cutoff)             // 7 日前より古いレコードを削除する
	if err != nil {
		t.Fatalf("DeleteOlderThan がエラーを返しました: %v", err)
	}

	// 削除対象レコードが存在しないことを確認する
	var oldCount int64
	db.Model(&models.DeploymentMetrics{}).Where("id = ?", oldRecord.ID).Count(&oldCount) // 削除対象レコードを確認する
	if oldCount != 0 { // 0 件であることを確認する
		t.Errorf("古いレコードが削除されていません（id=%s）", oldRecord.ID)
	}

	// 保持対象レコードが残っていることを確認する
	var recentCount int64
	db.Model(&models.DeploymentMetrics{}).Where("id = ?", recentRecord.ID).Count(&recentCount) // 保持対象レコードを確認する
	if recentCount != 1 { // 1 件残っていることを確認する
		t.Errorf("新しいレコードが誤って削除されました（id=%s）", recentRecord.ID)
	}
}

// TestDeploymentMetricsRepository_DeleteOlderThan_3日前の基準で削除される は3日前の日時を基準として古いレコードのみ削除されることを確認する
func TestDeploymentMetricsRepository_DeleteOlderThan_3日前の基準で削除される(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する

	// DeploymentMetrics テーブルをマイグレーションする
	if err := db.AutoMigrate(&models.DeploymentMetrics{}); err != nil {
		t.Fatalf("DeploymentMetrics マイグレーションに失敗しました: %v", err)
	}

	projectData := createTestProject(t, db)                               // テスト用 Project を作成する
	deploymentData := createTestDeploymentForMetrics(t, db, projectData.ID) // テスト用 Deployment を作成する

	now := time.Now().Truncate(time.Second)      // 現在時刻を取得する
	cutoff := now.Add(-3 * 24 * time.Hour)       // 3 日前の日時を設定する（ISSUE-065 の保持期間）

	// 3 日より前のレコード（削除対象）と 3 日以内のレコード（保持対象）を作成する
	oldRecord := &models.DeploymentMetrics{
		DeploymentID: deploymentData.ID, PodName: "pod-old-3days",
		CPUMillicores: 100, MemoryBytes: 100, ReadyReplicas: 1, TotalReplicas: 1,
		RecordedAt: cutoff.Add(-1 * time.Second), // 3 日前より 1 秒古いレコード（削除対象）
	}
	recentRecord := &models.DeploymentMetrics{
		DeploymentID: deploymentData.ID, PodName: "pod-recent-3days",
		CPUMillicores: 200, MemoryBytes: 200, ReadyReplicas: 1, TotalReplicas: 1,
		RecordedAt: cutoff.Add(1 * time.Second), // 3 日前より 1 秒新しいレコード（保持対象）
	}
	db.Create(oldRecord)    // 削除対象レコードを作成する
	db.Create(recentRecord) // 保持対象レコードを作成する
	t.Cleanup(func() {      // テスト終了後に残存レコードを削除する
		db.Unscoped().Delete(oldRecord)
		db.Unscoped().Delete(recentRecord)
	})

	repo := NewDeploymentMetricsRepository(db)                  // リポジトリを生成する
	err := repo.DeleteOlderThan(context.Background(), cutoff)   // 3 日前より古いレコードを削除する
	if err != nil {
		t.Fatalf("DeleteOlderThan がエラーを返しました: %v", err)
	}

	// 削除対象レコード（3日より前）が存在しないことを確認する
	var oldCount int64
	db.Model(&models.DeploymentMetrics{}).Where("id = ?", oldRecord.ID).Count(&oldCount) // 削除対象レコードを確認する
	if oldCount != 0 { // 0 件であることを確認する
		t.Errorf("3日より古いレコードが削除されていません（id=%s）", oldRecord.ID)
	}

	// 保持対象レコード（3日以内）が残っていることを確認する
	var recentCount int64
	db.Model(&models.DeploymentMetrics{}).Where("id = ?", recentRecord.ID).Count(&recentCount) // 保持対象レコードを確認する
	if recentCount != 1 { // 1 件残っていることを確認する
		t.Errorf("3日以内のレコードが誤って削除されました（id=%s）", recentRecord.ID)
	}
}
