package repository

import (
	"app/shared/models"
	"context"

	"gorm.io/gorm"
)

// HarborCredentialRepository は harbor_credentials テーブルへのアクセスを定義するインターフェース
type HarborCredentialRepository interface {
	Create(ctx context.Context, tx *gorm.DB, credential *models.HarborCredential) error                 // credential を作成する（トランザクション内用）
	CreateNoTx(ctx context.Context, credential *models.HarborCredential) error                          // credential を作成する（トランザクション外用）
	FindByProjectID(ctx context.Context, tx *gorm.DB, projectID string) (*models.HarborCredential, error) // projectID で credential を取得する（トランザクション内用）
	FindByProjectIDNoTx(ctx context.Context, projectID string) (*models.HarborCredential, error)        // projectID で credential を取得する（トランザクション外用）
	DeleteByProjectID(ctx context.Context, tx *gorm.DB, projectID string) error                        // projectID に紐づく credential を削除する
}

// harborCredentialRepositoryImpl は HarborCredentialRepository の GORM 実装
type harborCredentialRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewHarborCredentialRepository は HarborCredentialRepository の実装を返す
func NewHarborCredentialRepository(db *gorm.DB) HarborCredentialRepository {
	return &harborCredentialRepositoryImpl{db: db} // 実装を生成して返す
}

// Create は harbor_credential レコードを作成する（トランザクション内用）
func (repo *harborCredentialRepositoryImpl) Create(ctx context.Context, tx *gorm.DB, credential *models.HarborCredential) error {
	return tx.WithContext(ctx).Create(credential).Error // tx を使って作成する
}

// CreateNoTx は harbor_credential レコードを作成する（トランザクション外用）
func (repo *harborCredentialRepositoryImpl) CreateNoTx(ctx context.Context, credential *models.HarborCredential) error {
	return repo.db.WithContext(ctx).Create(credential).Error // db を使って作成する
}

// FindByProjectID は projectID に対応する credential を返す
func (repo *harborCredentialRepositoryImpl) FindByProjectID(ctx context.Context, tx *gorm.DB, projectID string) (*models.HarborCredential, error) {
	var credentialData models.HarborCredential
	err := tx.WithContext(ctx).Where("project_id = ?", projectID).First(&credentialData).Error // tx を使って取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return &credentialData, nil
}

// FindByProjectIDNoTx は projectID に対応する credential を返す（トランザクション外用）
func (repo *harborCredentialRepositoryImpl) FindByProjectIDNoTx(ctx context.Context, projectID string) (*models.HarborCredential, error) {
	var credentialData models.HarborCredential
	err := repo.db.WithContext(ctx).Where("project_id = ?", projectID).First(&credentialData).Error // db を使って取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return &credentialData, nil
}

// DeleteByProjectID は projectID に紐づく credential レコードを削除する
func (repo *harborCredentialRepositoryImpl) DeleteByProjectID(ctx context.Context, tx *gorm.DB, projectID string) error {
	return tx.WithContext(ctx).Where("project_id = ?", projectID).Delete(&models.HarborCredential{}).Error // tx を使って削除する
}
