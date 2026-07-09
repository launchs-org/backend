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
// 直近のポーリングに存在しない（スケールダウン等で消えた）Pod のレコードは除外する
func (repo *deploymentMetricsRepositoryImpl) FindByDeploymentID(ctx context.Context, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) {
	var latestRecordedAt time.Time // 直近のポーリング日時を格納する変数を定義する
	latestErr := repo.db.WithContext(ctx).
		Model(&models.DeploymentMetrics{}).
		Where("deployment_id = ?", deploymentID).
		Select("MAX(recorded_at)").
		Scan(&latestRecordedAt).Error // 直近のポーリング日時を取得する
	if latestErr != nil {
		return nil, latestErr // 取得エラーを返す
	}

	var activePodNameList []string // 直近のポーリングで生存していた Pod 名一覧を格納する変数を定義する
	if !latestRecordedAt.IsZero() { // レコードが1件も存在しない場合はスキップする
		if activeErr := repo.db.WithContext(ctx).
			Model(&models.DeploymentMetrics{}).
			Where("deployment_id = ? AND recorded_at = ?", deploymentID, latestRecordedAt).
			Pluck("pod_name", &activePodNameList).Error; activeErr != nil { // 直近のポーリングに存在する Pod 名を取得する
			return nil, activeErr // 取得エラーを返す
		}
	}

	var metricsList []*models.DeploymentMetrics // 結果を格納するスライスを定義する
	query := repo.db.WithContext(ctx).
		Where("deployment_id = ?", deploymentID) // deploymentID で絞り込む
	if len(activePodNameList) > 0 {              // 生存 Pod が存在する場合のみ絞り込む（全除外を防ぐ）
		query = query.Where("pod_name IN ?", activePodNameList) // 消えた Pod のレコードを除外する
	}
	if err := query.
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
