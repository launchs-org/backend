package repository

import (
	"app/shared/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// BuildLogChunkRepository は build_log_chunks テーブルへのアクセスを定義するインターフェース
type BuildLogChunkRepository interface {
	Create(ctx context.Context, chunk *models.BuildLogChunk) error                                                       // ログチャンクを作成する
	FindByBuildID(ctx context.Context, buildID string) ([]models.BuildLogChunk, error)                                   // buildID に紐づくログチャンク一覧を取得する
	FindByBuildIDSince(ctx context.Context, buildID string, since time.Time) ([]models.BuildLogChunk, error)             // since より後のログチャンクを取得する（差分ポーリング用）
}

// buildLogChunkRepositoryImpl は BuildLogChunkRepository の GORM 実装
type buildLogChunkRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewBuildLogChunkRepository は BuildLogChunkRepository の実装を返す
func NewBuildLogChunkRepository(db *gorm.DB) BuildLogChunkRepository {
	return &buildLogChunkRepositoryImpl{db: db} // 実装を生成して返す
}

// Create はログチャンクレコードを作成する
func (repo *buildLogChunkRepositoryImpl) Create(ctx context.Context, chunk *models.BuildLogChunk) error {
	return repo.db.WithContext(ctx).Create(chunk).Error // db を使って作成する
}

// FindByBuildID は buildID に紐づくログチャンク一覧を作成順で返す
func (repo *buildLogChunkRepositoryImpl) FindByBuildID(ctx context.Context, buildID string) ([]models.BuildLogChunk, error) {
	var chunkList []models.BuildLogChunk                                                                                           // ログチャンク一覧を格納するスライスを定義する
	if err := repo.db.WithContext(ctx).Where("build_id = ?", buildID).Order("created_at ASC").Find(&chunkList).Error; err != nil { // db からログチャンク一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return chunkList, nil // ログチャンク一覧を返す
}

// FindByBuildIDSince は buildID に紐づく since より後のログチャンクを作成順で返す
func (repo *buildLogChunkRepositoryImpl) FindByBuildIDSince(ctx context.Context, buildID string, since time.Time) ([]models.BuildLogChunk, error) {
	var chunkList []models.BuildLogChunk                                                                                                                              // ログチャンク一覧を格納するスライスを定義する
	if err := repo.db.WithContext(ctx).Where("build_id = ? AND created_at > ?", buildID, since).Order("created_at ASC").Find(&chunkList).Error; err != nil { // since より後のチャンクを取得する
		return nil, err // 取得エラーを返す
	}
	return chunkList, nil // ログチャンク一覧を返す
}
