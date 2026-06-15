package repository

import (
	"app/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// PodLogChunkRepository は pod_log_chunks テーブルへのアクセスを定義するインターフェース
type PodLogChunkRepository interface {
	Create(ctx context.Context, chunk *models.PodLogChunk) error                                                             // ログチャンクを作成する
	FindByDeploymentID(ctx context.Context, deploymentID string) ([]models.PodLogChunk, error)                               // deploymentID に紐づくログチャンク一覧を取得する
	FindByDeploymentIDSince(ctx context.Context, deploymentID string, since time.Time) ([]models.PodLogChunk, error)         // since より後のログチャンクを取得する（差分ポーリング用）
}

// podLogChunkRepositoryImpl は PodLogChunkRepository の GORM 実装
type podLogChunkRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewPodLogChunkRepository は PodLogChunkRepository の実装を返す
func NewPodLogChunkRepository(db *gorm.DB) PodLogChunkRepository {
	return &podLogChunkRepositoryImpl{db: db} // 実装を生成して返す
}

// Create はログチャンクレコードを作成する
func (repo *podLogChunkRepositoryImpl) Create(ctx context.Context, chunk *models.PodLogChunk) error {
	return repo.db.WithContext(ctx).Create(chunk).Error // db を使って作成する
}

// FindByDeploymentID は deploymentID に紐づくログチャンク一覧を作成順で返す
func (repo *podLogChunkRepositoryImpl) FindByDeploymentID(ctx context.Context, deploymentID string) ([]models.PodLogChunk, error) {
	var chunkList []models.PodLogChunk                                                                                                          // ログチャンク一覧を格納するスライスを定義する
	if err := repo.db.WithContext(ctx).Where("deployment_id = ?", deploymentID).Order("created_at ASC").Find(&chunkList).Error; err != nil { // db からログチャンク一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return chunkList, nil // ログチャンク一覧を返す
}

// FindByDeploymentIDSince は deploymentID に紐づく since より後のログチャンクを作成順で返す
func (repo *podLogChunkRepositoryImpl) FindByDeploymentIDSince(ctx context.Context, deploymentID string, since time.Time) ([]models.PodLogChunk, error) {
	var chunkList []models.PodLogChunk                                                                                                                                           // ログチャンク一覧を格納するスライスを定義する
	if err := repo.db.WithContext(ctx).Where("deployment_id = ? AND created_at > ?", deploymentID, since).Order("created_at ASC").Find(&chunkList).Error; err != nil { // since より後のチャンクを取得する
		return nil, err // 取得エラーを返す
	}
	return chunkList, nil // ログチャンク一覧を返す
}
