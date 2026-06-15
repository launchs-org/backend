package repository

import (
	"app/models"
	"context"
	"testing"
)

// TestWebhookRepository_FindByDeploymentID_正常に取得される は FindByDeploymentID で webhook が取得されることを確認する
func TestWebhookRepository_FindByDeploymentID_正常に取得される(t *testing.T) {
	db := setupTestDB(t)                     // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	// テスト用 Deployment を作成する
	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,                 // project_id を設定する
		Name:      "test-deployment-for-webhook",  // deployment 名を設定する
		Status:    models.DeploymentStatusPending, // ステータスを設定する
		AppStatus: models.AppStatusPending,        // app_status を設定する
		Type:      "image_url",                    // type を設定する
	}
	if err := db.Create(deploymentData).Error; err != nil {
		t.Fatalf("テスト用 Deployment の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	// テスト用 Webhook を作成する
	webhookData := &models.DeploymentWebhook{
		DeploymentID:  deploymentData.ID,             // deployment_id を設定する
		Secret:        "test-secret-value",           // シークレットを設定する
		GithubRepoURL: "https://github.com/org/repo", // GitHub リポジトリ URL を設定する
		IsActive:      true,                          // 有効に設定する
	}
	if err := db.Create(webhookData).Error; err != nil {
		t.Fatalf("テスト用 Webhook の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(webhookData) }) // テスト終了後にレコードを削除する

	repo := NewWebhookRepository(db) // リポジトリを生成する

	fetchedWebhook, err := repo.FindByDeploymentID(context.Background(), deploymentData.ID) // FindByDeploymentID を実行する
	if err != nil {
		t.Fatalf("FindByDeploymentID がエラーを返しました: %v", err)
	}
	if fetchedWebhook.ID != webhookData.ID { // ID が一致することを確認する
		t.Errorf("期待する ID: %s, 実際の ID: %s", webhookData.ID, fetchedWebhook.ID)
	}
	if fetchedWebhook.Secret != "test-secret-value" { // シークレットが一致することを確認する
		t.Errorf("期待する Secret: test-secret-value, 実際の Secret: %s", fetchedWebhook.Secret)
	}
	if fetchedWebhook.GithubRepoURL != "https://github.com/org/repo" { // GitHub リポジトリ URL が一致することを確認する
		t.Errorf("期待する GithubRepoURL: https://github.com/org/repo, 実際の GithubRepoURL: %s", fetchedWebhook.GithubRepoURL)
	}
}
