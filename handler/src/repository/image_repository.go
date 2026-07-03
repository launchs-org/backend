package repository

import (
	"handler/models"
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// ImageRepository は images テーブルへのアクセスを定義するインターフェース
type ImageRepository interface {
	Create(ctx context.Context, image *models.Image) error                            // イメージレコードを作成する
	CreateWithTx(ctx context.Context, tx *gorm.DB, image *models.Image) error         // トランザクション内でイメージレコードを作成する
	FindByID(ctx context.Context, imageID string) (*models.Image, error)              // ID でイメージレコードを取得する
	FindByBuildID(ctx context.Context, buildID string) (*models.Image, error)         // buildID に紐づくイメージレコードを取得する（1ビルド1イメージの想定）
	FindByProjectIDAndURL(ctx context.Context, projectID string, imageURL string) (*models.Image, error) // projectID + imageURL に一致するイメージを検索する（(project_id, image_url) は複合ユニークキーのため重複作成防止に使う）
	FindOrCreate(ctx context.Context, image *models.Image) (*models.Image, error)    // (project_id, image_url) が既存であれば再利用し、なければ作成する
	FindOrCreateWithTx(ctx context.Context, tx *gorm.DB, image *models.Image) (*models.Image, error) // トランザクション内で (project_id, image_url) が既存であれば再利用し、なければ作成する
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

// FindByProjectIDAndURL は projectID + imageURL に一致するイメージレコードを返す
func (repo *imageRepositoryImpl) FindByProjectIDAndURL(ctx context.Context, projectID string, imageURL string) (*models.Image, error) {
	var imageData models.Image                                                                                                                   // イメージレコードを格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("project_id = ? AND image_url = ?", projectID, imageURL).First(&imageData).Error; err != nil { // db からイメージレコードを取得する
		return nil, err // 取得エラーを返す
	}
	return &imageData, nil // イメージレコードを返す
}

// FindOrCreate は (project_id, image_url) が既存であれば再利用し、なければ作成して返す
func (repo *imageRepositoryImpl) FindOrCreate(ctx context.Context, image *models.Image) (*models.Image, error) {
	return findOrCreateImage(ctx, repo.db, image)
}

// FindOrCreateWithTx はトランザクション内で (project_id, image_url) が既存であれば再利用し、なければ作成して返す
func (repo *imageRepositoryImpl) FindOrCreateWithTx(ctx context.Context, tx *gorm.DB, image *models.Image) (*models.Image, error) {
	return findOrCreateImage(ctx, tx, image)
}

// findOrCreateImage は (project_id, image_url) が既存であれば再利用し、なければ作成して返す共通実装
// (project_id, image_url) は複合ユニークキーのため、同時実行でユニーク制約違反が発生した場合は再取得してフォールバックする
func findOrCreateImage(ctx context.Context, db *gorm.DB, image *models.Image) (*models.Image, error) {
	var existingImage models.Image
	err := db.WithContext(ctx).Where("project_id = ? AND image_url = ?", image.ProjectID, image.ImageURL).First(&existingImage).Error // 既存のイメージを検索する
	if err == nil {                                                                                                                     // 既存レコードが見つかった場合はそれを返す
		return &existingImage, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) { // レコードなし以外のエラーはそのまま返す
		return nil, err
	}

	if createErr := db.WithContext(ctx).Create(image).Error; createErr != nil { // 見つからない場合は新規作成する
		// 同時実行によりユニーク制約違反が発生した場合は、既に他リクエストが作成したレコードを再取得する
		if isUniqueViolation(createErr) {
			var conflictedImage models.Image
			if findErr := db.WithContext(ctx).Where("project_id = ? AND image_url = ?", image.ProjectID, image.ImageURL).First(&conflictedImage).Error; findErr != nil {
				return nil, findErr // 再取得エラーを返す
			}
			return &conflictedImage, nil // 他リクエストが作成したレコードを返す
		}
		return nil, createErr // その他の作成エラーを返す
	}
	return image, nil // 新規作成したイメージを返す
}

// isUniqueViolation は PostgreSQL のユニーク制約違反エラーかどうかを判定する
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" // 23505 = unique_violation
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
