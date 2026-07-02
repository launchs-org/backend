package repository

import (
	"handler/models"
	"context"
	"testing"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// TestApplyHistoryRepository_Create_正常に作成される は apply_history レコードが作成されることを確認する
func TestApplyHistoryRepository_Create_正常に作成される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Deployment を作成する
	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,                 // プロジェクト ID を設定する
		Name:      "test-app-hist",                // デプロイメント名を設定する
		Type:      models.DeploymentTypeImageURL,  // タイプを設定する
		Status:    models.DeploymentStatusPending, // ステータスを設定する
		AppStatus: models.AppStatusPending,        // アプリステータスを設定する
	}
	db.Create(deploymentData)                                          // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	repo := NewApplyHistoryRepository(db) // リポジトリを生成する
	historyData := &models.ApplyHistory{  // apply_history レコードを生成する
		DeploymentID: deploymentData.ID,          // deployment_id を設定する
		Manifests:    datatypes.JSON(`{}`),       // マニフェスト JSON を設定する
		Status:       models.ApplyStatusApplied,  // ステータスを applied に設定する
		AppliedAt:    time.Now(),                  // apply 時刻を設定する
	}

	err := repo.Create(context.Background(), db, historyData) // リポジトリを実行する
	if err != nil {
		t.Fatalf("Create がエラーを返しました: %v", err)
	}
	if historyData.ID == "" { // ID が付与されていることを確認する
		t.Error("作成後に ID が設定されていません")
	}
	t.Cleanup(func() { db.Unscoped().Delete(historyData) }) // テスト終了後にレコードを削除する

	// DB から取得して値を確認する
	var fetchedHistory models.ApplyHistory
	db.First(&fetchedHistory, "id = ?", historyData.ID) // 作成したレコードを取得する
	if fetchedHistory.Status != models.ApplyStatusApplied { // status が applied であることを確認する
		t.Errorf("期待する status: applied, 実際の status: %s", fetchedHistory.Status)
	}
	if fetchedHistory.DeploymentID != deploymentData.ID { // deployment_id が一致することを確認する
		t.Errorf("期待する deployment_id: %s, 実際の deployment_id: %s", deploymentData.ID, fetchedHistory.DeploymentID)
	}
}

// TestApplyHistoryRepository_UpdateStatus_failedに更新できる は UpdateStatus で status が failed に変更されることを確認する
func TestApplyHistoryRepository_UpdateStatus_failedに更新できる(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Deployment を作成する
	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,
		Name:      "test-app-hist2",
		Type:      models.DeploymentTypeImageURL,
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusPending,
	}
	db.Create(deploymentData)                                          // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	// apply_history を applied で作成する
	historyData := &models.ApplyHistory{
		DeploymentID: deploymentData.ID,
		Manifests:    datatypes.JSON(`{}`),
		Status:       models.ApplyStatusApplied, // 初期ステータスは applied とする
		AppliedAt:    time.Now(),
	}
	db.Create(historyData)                                          // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(historyData) }) // テスト終了後にレコードを削除する

	repo := NewApplyHistoryRepository(db)              // リポジトリを生成する
	historyData.ErrorMessage = "k8s apply failed"     // エラーメッセージを設定する

	err := repo.UpdateStatus(context.Background(), db, historyData, models.ApplyStatusFailed) // status を failed に更新する
	if err != nil {
		t.Fatalf("UpdateStatus がエラーを返しました: %v", err)
	}

	// DB から取得して更新を確認する
	var fetchedHistory models.ApplyHistory
	db.First(&fetchedHistory, "id = ?", historyData.ID) // 更新後のレコードを取得する
	if fetchedHistory.Status != models.ApplyStatusFailed { // status が failed に更新されていることを確認する
		t.Errorf("期待する status: failed, 実際の status: %s", fetchedHistory.Status)
	}
	if fetchedHistory.ErrorMessage != "k8s apply failed" { // error_message が設定されていることを確認する
		t.Errorf("期待する error_message: k8s apply failed, 実際の error_message: %s", fetchedHistory.ErrorMessage)
	}
}

// TestApplyHistoryRepository_FindAllByDeploymentID_新しい順で取得できる は履歴が applied_at 降順で返ることを確認する
func TestApplyHistoryRepository_FindAllByDeploymentID_新しい順で取得できる(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Deployment を作成する
	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,
		Name:      "test-app-hist-list",
		Type:      models.DeploymentTypeImageURL,
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusPending,
	}
	db.Create(deploymentData)                                      // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	// 古い履歴を先に作成する
	oldHistory := &models.ApplyHistory{
		DeploymentID: deploymentData.ID,
		Manifests:    datatypes.JSON(`{}`),
		Status:       models.ApplyStatusApplied,
		AppliedAt:    time.Now().Add(-1 * time.Hour), // 1時間前を設定する
	}
	db.Create(oldHistory)                                      // 古い履歴を作成する
	t.Cleanup(func() { db.Unscoped().Delete(oldHistory) }) // テスト終了後にレコードを削除する

	// 新しい履歴を後から作成する
	newHistory := &models.ApplyHistory{
		DeploymentID: deploymentData.ID,
		Manifests:    datatypes.JSON(`{}`),
		Status:       models.ApplyStatusFailed,
		AppliedAt:    time.Now(), // 現在時刻を設定する
	}
	db.Create(newHistory)                                      // 新しい履歴を作成する
	t.Cleanup(func() { db.Unscoped().Delete(newHistory) }) // テスト終了後にレコードを削除する

	repo := NewApplyHistoryRepository(db) // リポジトリを生成する

	historyList, err := repo.FindAllByDeploymentID(context.Background(), deploymentData.ID) // 履歴一覧を取得する
	if err != nil {
		t.Fatalf("FindAllByDeploymentID がエラーを返しました: %v", err)
	}
	if len(historyList) != 2 { // 2件返ることを確認する
		t.Fatalf("期待する件数: 2, 実際の件数: %d", len(historyList))
	}
	if historyList[0].ID != newHistory.ID { // 最初の要素が新しい履歴であることを確認する
		t.Errorf("最初の要素が新しい履歴でありません: %s", historyList[0].ID)
	}
	if historyList[1].ID != oldHistory.ID { // 2番目の要素が古い履歴であることを確認する
		t.Errorf("2番目の要素が古い履歴でありません: %s", historyList[1].ID)
	}
}

// TestApplyHistoryRepository_FindAllByDeploymentID_履歴が存在しない場合は空スライスが返る は履歴が0件のとき空スライスが返ることを確認する
func TestApplyHistoryRepository_FindAllByDeploymentID_履歴が存在しない場合は空スライスが返る(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,
		Name:      "test-app-hist-empty",
		Type:      models.DeploymentTypeImageURL,
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusPending,
	}
	db.Create(deploymentData)                                      // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	repo := NewApplyHistoryRepository(db) // リポジトリを生成する

	historyList, err := repo.FindAllByDeploymentID(context.Background(), deploymentData.ID) // 履歴一覧を取得する
	if err != nil {
		t.Fatalf("FindAllByDeploymentID がエラーを返しました: %v", err)
	}
	if len(historyList) != 0 { // 空スライスが返ることを確認する
		t.Errorf("期待する件数: 0, 実際の件数: %d", len(historyList))
	}
}

// TestDeploymentRepository_FindByIDForUpdate_正常にロック付きで取得される は FindByIDForUpdate で Deployment が取得されることを確認する
func TestDeploymentRepository_FindByIDForUpdate_正常にロック付きで取得される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Deployment を作成する
	pendingImageID := "image-id-lock" // pending_image_id に設定する ID を定義する
	deploymentData := &models.Deployment{
		ProjectID:      projectData.ID,
		Name:           "test-app-lock",
		Type:           models.DeploymentTypeImageURL,
		Status:         models.DeploymentStatusPending,
		AppStatus:      models.AppStatusPending,
		PendingImageID: &pendingImageID,
	}
	db.Create(deploymentData)                                          // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	repo := NewDeploymentRepository(db) // リポジトリを生成する

	// トランザクション内で FOR UPDATE ロック付き取得を実行する
	var fetchedDeployment *models.Deployment
	txErr := db.Transaction(func(tx *gorm.DB) error { // トランザクションを開始する
		result, err := repo.FindByIDForUpdate(context.Background(), tx, deploymentData.ID) // FOR UPDATE で取得する
		if err != nil {
			return err // 取得エラーを返す
		}
		fetchedDeployment = result // 結果を格納する
		return nil                 // コミットする
	})
	if txErr != nil {
		t.Fatalf("FindByIDForUpdate がエラーを返しました: %v", txErr)
	}

	if fetchedDeployment.ID != deploymentData.ID { // ID が一致することを確認する
		t.Errorf("期待する ID: %s, 実際の ID: %s", deploymentData.ID, fetchedDeployment.ID)
	}
	if fetchedDeployment.PendingImageID == nil || *fetchedDeployment.PendingImageID != "image-id-lock" { // pending_image_id が正しいことを確認する
		t.Errorf("期待する pending_image_id: image-id-lock, 実際の pending_image_id: %v", fetchedDeployment.PendingImageID)
	}
}

// TestDeploymentRepository_Updates_apply後のcurrentフィールドが更新される は Updates で apply 後の current フィールドが更新されることを確認する
func TestDeploymentRepository_Updates_apply後のcurrentフィールドが更新される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Image レコードを作成する（pending_image_id が参照する image_url を保持する）
	imageData := &models.Image{
		ProjectID: projectData.ID, // プロジェクト ID を設定する
		ImageURL:  "nginx:latest", // イメージ URL を設定する
	}
	db.Create(imageData)                                          // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(imageData) }) // テスト終了後にレコードを削除する

	// テスト用 Deployment を作成する（pending 状態）
	deploymentData := &models.Deployment{
		ProjectID:           projectData.ID,
		Name:                "test-app-updates",
		Type:                models.DeploymentTypeImageURL,
		Status:              models.DeploymentStatusPending,
		AppStatus:           models.AppStatusPending,
		PendingImageID:      &imageData.ID, // pending image_id を設定する
		PendingInstanceSize: "small",       // pending instance_size を設定する
		PendingReplicas:     2,             // pending replicas を設定する
	}
	db.Create(deploymentData)                                          // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	repo := NewDeploymentRepository(db) // リポジトリを生成する

	// apply 後の昇格を模倣する更新を実行する
	updates := map[string]interface{}{
		"image_id":              imageData.ID,                      // image_id を昇格する
		"pending_image_id":      nil,                               // pending_image_id をクリアする
		"instance_size":         "small",                      // instance_size を昇格する
		"pending_instance_size": "",                           // pending_instance_size をクリアする
		"replicas":              int32(2),                     // replicas を昇格する
		"pending_replicas":      int32(0),                     // pending_replicas をクリアする
		"status":                models.DeploymentStatusRunning,   // status を running に更新する
		"app_status":            models.AppStatusDeploying,         // app_status を deploying に更新する
	}

	err := repo.Updates(context.Background(), db, deploymentData, updates) // Updates を実行する
	if err != nil {
		t.Fatalf("Updates がエラーを返しました: %v", err)
	}

	// DB から取得して更新を確認する
	var fetchedDeployment models.Deployment
	db.First(&fetchedDeployment, "id = ?", deploymentData.ID) // 更新後のレコードを取得する
	if fetchedDeployment.ImageID == nil || *fetchedDeployment.ImageID != imageData.ID { // image_id が昇格されていることを確認する
		t.Errorf("期待する image_id: %s, 実際の image_id: %v", imageData.ID, fetchedDeployment.ImageID)
	}
	if fetchedDeployment.PendingImageID != nil { // pending_image_id がクリアされていることを確認する
		t.Errorf("期待する pending_image_id: nil, 実際の pending_image_id: %v", fetchedDeployment.PendingImageID)
	}
	if fetchedDeployment.Status != models.DeploymentStatusRunning { // status が running であることを確認する
		t.Errorf("期待する status: running, 実際の status: %s", fetchedDeployment.Status)
	}
	if fetchedDeployment.AppStatus != models.AppStatusDeploying { // app_status が deploying であることを確認する
		t.Errorf("期待する app_status: deploying, 実際の app_status: %s", fetchedDeployment.AppStatus)
	}
}
