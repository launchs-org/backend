package repository

import (
	"app/shared/models"
	"context"

	"gorm.io/gorm"
)

// WebhookRepository は deployment_webhooks テーブルへのアクセスを定義するインターフェース
type WebhookRepository interface {
	Create(ctx context.Context, webhookData *models.DeploymentWebhook) error                      // webhook を作成する
	FindByDeploymentID(ctx context.Context, deploymentID string) (*models.DeploymentWebhook, error) // deployment_id で webhook を取得する
	FindByID(ctx context.Context, webhookID string) (*models.DeploymentWebhook, error)            // id で webhook を取得する
	Delete(ctx context.Context, webhookID string) error                                           // webhook を削除する
}

// webhookRepositoryImpl は WebhookRepository の GORM 実装
type webhookRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewWebhookRepository は WebhookRepository の実装を返す
func NewWebhookRepository(db *gorm.DB) WebhookRepository {
	return &webhookRepositoryImpl{db: db} // 実装を生成して返す
}

// Create は webhook レコードを作成する
func (repo *webhookRepositoryImpl) Create(ctx context.Context, webhookData *models.DeploymentWebhook) error {
	return repo.db.WithContext(ctx).Create(webhookData).Error // db を使って作成する
}

// FindByDeploymentID は deploymentID に紐づく webhook を返す
func (repo *webhookRepositoryImpl) FindByDeploymentID(ctx context.Context, deploymentID string) (*models.DeploymentWebhook, error) {
	var webhookData models.DeploymentWebhook                                                                                    // webhook を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("deployment_id = ?", deploymentID).First(&webhookData).Error; err != nil { // db から webhook を取得する
		return nil, err // 取得エラーを返す
	}
	return &webhookData, nil // webhook を返す
}

// FindByID は id に対応する webhook を返す
func (repo *webhookRepositoryImpl) FindByID(ctx context.Context, webhookID string) (*models.DeploymentWebhook, error) {
	var webhookData models.DeploymentWebhook                                                                   // webhook を格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&webhookData, "id = ?", webhookID).Error; err != nil { // db から webhook を取得する
		return nil, err // 取得エラーを返す
	}
	return &webhookData, nil // webhook を返す
}

// Delete は id に対応する webhook を削除する
func (repo *webhookRepositoryImpl) Delete(ctx context.Context, webhookID string) error {
	return repo.db.WithContext(ctx).Delete(&models.DeploymentWebhook{}, "id = ?", webhookID).Error // db から webhook を削除する
}
