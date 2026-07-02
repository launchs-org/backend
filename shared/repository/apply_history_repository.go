package repository

import (
	"app/shared/models"
	"context"

	"gorm.io/gorm"
)

// ApplyHistoryRepository は apply_history テーブルへのアクセスを定義するインターフェース
type ApplyHistoryRepository interface {
	Create(ctx context.Context, tx *gorm.DB, history *models.ApplyHistory) error                                    // apply_history を作成する
	UpdateStatus(ctx context.Context, tx *gorm.DB, history *models.ApplyHistory, status models.ApplyStatus) error  // apply_history のステータスを更新する
	FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.ApplyHistory, error)                 // deploymentID に紐づく履歴一覧を取得する
	DeleteAllByDeploymentID(ctx context.Context, deploymentID string) error                                         // deploymentID に紐づく履歴を全件削除する
}

// applyHistoryRepositoryImpl は ApplyHistoryRepository の GORM 実装
type applyHistoryRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewApplyHistoryRepository は ApplyHistoryRepository の実装を返す
func NewApplyHistoryRepository(db *gorm.DB) ApplyHistoryRepository {
	return &applyHistoryRepositoryImpl{db: db} // 実装を生成して返す
}

// Create は apply_history レコードを作成する
func (repo *applyHistoryRepositoryImpl) Create(ctx context.Context, tx *gorm.DB, history *models.ApplyHistory) error {
	return tx.WithContext(ctx).Create(history).Error // tx を使って作成する
}

// UpdateStatus は apply_history のステータスとエラーメッセージを更新する
func (repo *applyHistoryRepositoryImpl) UpdateStatus(ctx context.Context, tx *gorm.DB, history *models.ApplyHistory, status models.ApplyStatus) error {
	return tx.WithContext(ctx).Model(history).Updates(map[string]interface{}{ // tx を使ってステータスを更新する
		"status":        status,               // ステータスを更新する
		"error_message": history.ErrorMessage, // エラーメッセージを更新する
	}).Error
}

// FindAllByDeploymentID は deploymentID に紐づく apply_history 一覧を applied_at 降順で取得する
func (repo *applyHistoryRepositoryImpl) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.ApplyHistory, error) {
	var historyList []*models.ApplyHistory                                                                                     // 結果を格納するスライスを定義する
	err := repo.db.WithContext(ctx).Where("deployment_id = ?", deploymentID).Order("applied_at DESC").Find(&historyList).Error // 新しい順に取得する
	return historyList, err                                                                                                    // 結果とエラーを返す
}

// DeleteAllByDeploymentID は deploymentID に紐づく apply_history を全件削除する
func (repo *applyHistoryRepositoryImpl) DeleteAllByDeploymentID(ctx context.Context, deploymentID string) error {
	return repo.db.WithContext(ctx).Where("deployment_id = ?", deploymentID).Delete(&models.ApplyHistory{}).Error // 全件削除する
}
