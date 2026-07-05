package repository

import (
	"context"
	"handler/models"
	"time"

	"gorm.io/gorm"
)

// CliTokenRepository は cli_tokens テーブルへのアクセスを定義するインターフェース
type CliTokenRepository interface {
	Create(ctx context.Context, cliTokenData *models.CliToken) error                // CLIトークンを作成する
	FindByID(ctx context.Context, cliTokenID string) (*models.CliToken, error)      // jtiでCLIトークンを取得する
	FindAllByUserID(ctx context.Context, userID string) ([]*models.CliToken, error) // ユーザーIDに紐づくCLIトークン一覧を取得する
	Revoke(ctx context.Context, cliTokenID string) error                            // CLIトークンを失効させる
}

// cliTokenRepositoryImpl は CliTokenRepository の GORM 実装
type cliTokenRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewCliTokenRepository は CliTokenRepository の実装を返す
func NewCliTokenRepository(db *gorm.DB) CliTokenRepository {
	return &cliTokenRepositoryImpl{db: db} // 実装を生成して返す
}

// Create は cli_tokens レコードを作成する
func (repo *cliTokenRepositoryImpl) Create(ctx context.Context, cliTokenData *models.CliToken) error {
	return repo.db.WithContext(ctx).Create(cliTokenData).Error // db を使って作成する
}

// FindByID は id（jti）に対応する CLIトークンを返す
func (repo *cliTokenRepositoryImpl) FindByID(ctx context.Context, cliTokenID string) (*models.CliToken, error) {
	var cliTokenData models.CliToken                                                                  // CLIトークンを格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&cliTokenData, "id = ?", cliTokenID).Error; err != nil { // db から取得する
		return nil, err // 取得エラーを返す
	}
	return &cliTokenData, nil // CLIトークンを返す
}

// FindAllByUserID は userID に紐づく CLIトークン一覧を返す
func (repo *cliTokenRepositoryImpl) FindAllByUserID(ctx context.Context, userID string) ([]*models.CliToken, error) {
	var cliTokenList []*models.CliToken                                                                                              // CLIトークン一覧を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc").Find(&cliTokenList).Error; err != nil { // db から一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return cliTokenList, nil // CLIトークン一覧を返す
}

// Revoke は id（jti）に対応する CLIトークンを失効させる
func (repo *cliTokenRepositoryImpl) Revoke(ctx context.Context, cliTokenID string) error {
	revokedAt := time.Now() // 失効日時を現在時刻にする
	return repo.db.WithContext(ctx).
		Model(&models.CliToken{}).
		Where("id = ?", cliTokenID).
		Update("revoked_at", revokedAt).Error // revoked_at を更新する
}
