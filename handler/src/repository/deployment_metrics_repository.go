package repository

import (
	"handler/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// DeploymentMetricsRepository は deployment_metrics テーブルへのアクセスを定義するインターフェース
type DeploymentMetricsRepository interface {
	CreateBatch(ctx context.Context, metricsList []*models.DeploymentMetrics) error                          // メトリクスを一括保存する
	FindByDeploymentID(ctx context.Context, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) // deploymentID に紐づくメトリクスを取得する
	DeleteOlderThan(ctx context.Context, before time.Time) error                                             // 指定日時より古いメトリクスを削除する
}

// deploymentMetricsRepositoryImpl は DeploymentMetricsRepository の GORM 実装
type deploymentMetricsRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewDeploymentMetricsRepository は DeploymentMetricsRepository の実装を返す
func NewDeploymentMetricsRepository(db *gorm.DB) DeploymentMetricsRepository {
	return &deploymentMetricsRepositoryImpl{db: db} // 実装を生成して返す
}

// CreateBatch はメトリクスレコードを一括挿入する
func (repo *deploymentMetricsRepositoryImpl) CreateBatch(ctx context.Context, metricsList []*models.DeploymentMetrics) error {
	if len(metricsList) == 0 { // メトリクスが空の場合はスキップする
		return nil
	}
	return repo.db.WithContext(ctx).Create(&metricsList).Error // 一括挿入する
}

// FindByDeploymentID は deploymentID に紐づくメトリクスを RecordedAt 降順で取得する
func (repo *deploymentMetricsRepositoryImpl) FindByDeploymentID(ctx context.Context, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) {
	var metricsList []*models.DeploymentMetrics                                                             // 結果を格納するスライスを定義する
	if err := repo.db.WithContext(ctx).
		Where("deployment_id = ?", deploymentID).
		Order("recorded_at DESC").
		Limit(limit).
		Find(&metricsList).Error; err != nil { // deploymentID に紐づくメトリクスを取得する
		return nil, err // 取得エラーを返す
	}
	return metricsList, nil // メトリクス一覧を返す
}

// DeleteOlderThan は before より古いメトリクスレコードを削除する
func (repo *deploymentMetricsRepositoryImpl) DeleteOlderThan(ctx context.Context, before time.Time) error {
	return repo.db.WithContext(ctx).
		Where("recorded_at < ?", before).
		Delete(&models.DeploymentMetrics{}).Error // 古いメトリクスを削除する
}
