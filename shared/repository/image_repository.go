package repository

import (
	"app/shared/models"
	"context"

	"gorm.io/gorm"
)

// ImageRepository は images テーブルへのアクセスを定義するインターフェース
type ImageRepository interface {
	Create(ctx context.Context, image *models.Image) error                            // イメージレコードを作成する
	CreateWithTx(ctx context.Context, tx *gorm.DB, image *models.Image) error         // トランザクション内でイメージレコードを作成する
	FindByID(ctx context.Context, imageID string) (*models.Image, error)              // ID でイメージレコードを取得する
	FindByBuildID(ctx context.Context, buildID string) (*models.Image, error)         // buildID に紐づくイメージレコードを取得する（1ビルド1イメージの想定）
	FindAllByProjectID(ctx context.Context, projectID string) ([]models.Image, error) // projectID に紐づくイメージ一覧を取得する（Build を Preload して返す）
	UpdateSizeBytes(ctx context.Context, imageID string, sizeBytes int64) error       // Harbor から取得したイメージサイズを更新する
	Delete(ctx context.Context, image *models.Image) error                            // イメージレコードを1件削除する
}

// imageRepositoryImpl は ImageRepository の GORM 実装
type imageRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewImageRepository は ImageRepository の実装を返す
func NewImageRepository(db *gorm.DB) ImageRepository {
	return &imageRepositoryImpl{db: db} // 実装を生成して返す
}

// Create はイメージレコードを作成する
func (repo *imageRepositoryImpl) Create(ctx context.Context, image *models.Image) error {
	return repo.db.WithContext(ctx).Create(image).Error // db を使って作成する
}

// CreateWithTx はトランザクション内でイメージレコードを作成する
func (repo *imageRepositoryImpl) CreateWithTx(ctx context.Context, tx *gorm.DB, image *models.Image) error {
	return tx.WithContext(ctx).Create(image).Error // tx を使って作成する
}

// FindByID は imageID に対応するイメージレコードを返す
func (repo *imageRepositoryImpl) FindByID(ctx context.Context, imageID string) (*models.Image, error) {
	var imageData models.Image                                                                       // イメージレコードを格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&imageData, "id = ?", imageID).Error; err != nil { // db からイメージレコードを取得する
		return nil, err // 取得エラーを返す
	}
	return &imageData, nil // イメージレコードを返す
}

// FindByBuildID は buildID に紐づくイメージレコードを返す
func (repo *imageRepositoryImpl) FindByBuildID(ctx context.Context, buildID string) (*models.Image, error) {
	var imageData models.Image                                                                              // イメージレコードを格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&imageData, "build_id = ?", buildID).Error; err != nil { // db からイメージレコードを取得する
		return nil, err // 取得エラーを返す
	}
	return &imageData, nil // イメージレコードを返す
}

// FindAllByProjectID は projectID に紐づくイメージ一覧を返す（Build を Preload する）
func (repo *imageRepositoryImpl) FindAllByProjectID(ctx context.Context, projectID string) ([]models.Image, error) {
	var imageList []models.Image                                                                                                                     // イメージ一覧を格納するスライスを定義する
	if err := repo.db.WithContext(ctx).Preload("Build").Where("project_id = ?", projectID).Order("created_at DESC").Find(&imageList).Error; err != nil { // db からイメージ一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return imageList, nil // イメージ一覧を返す
}

// UpdateSizeBytes は imageID に対応するイメージの size_bytes を更新する
func (repo *imageRepositoryImpl) UpdateSizeBytes(ctx context.Context, imageID string, sizeBytes int64) error {
	result := repo.db.WithContext(ctx).Model(&models.Image{}).Where("id = ?", imageID).Update("size_bytes", sizeBytes) // size_bytes を更新する
	if result.Error != nil {                                                                                            // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// Delete はイメージレコードを1件削除する
func (repo *imageRepositoryImpl) Delete(ctx context.Context, image *models.Image) error {
	return repo.db.WithContext(ctx).Delete(image).Error // レコードを削除する
}
