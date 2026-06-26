package repository

import (
	"app/models"
	"context"

	"gorm.io/gorm"
)

// EnvVarRepository は env_vars テーブルへのアクセスを定義するインターフェース
type EnvVarRepository interface {
	Create(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error                                                       // env_var を作成する
	FindByIDForUpdate(ctx context.Context, tx *gorm.DB, envVarID string) (*models.EnvVar, error)                                // env_var を ID で取得する（FOR UPDATE ロック）
	FindByID(ctx context.Context, envVarID string) (*models.EnvVar, error)                                                      // env_var を ID で取得する（トランザクション外用）
	FindAllByProjectID(ctx context.Context, projectID string) ([]*models.EnvVar, error)                                         // projectID に紐づく env_var 一覧を取得する
	ExistsByProjectIDAndKey(ctx context.Context, projectID string, key string, excludeID string) (bool, error)                  // 同一プロジェクト内でキーが重複しているか確認する（excludeID は自己除外に使用）
	Update(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error                                                       // env_var を更新する
	Delete(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error                                                       // env_var を削除する
}

// envVarRepositoryImpl は EnvVarRepository の GORM 実装
type envVarRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewEnvVarRepository は EnvVarRepository の実装を返す
func NewEnvVarRepository(db *gorm.DB) EnvVarRepository {
	return &envVarRepositoryImpl{db: db} // 実装を生成して返す
}

// Create は env_var レコードを作成する
func (repo *envVarRepositoryImpl) Create(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
	return tx.WithContext(ctx).Create(envVar).Error // tx を使って作成する
}

// FindByID は envVarID に対応する env_var を返す
func (repo *envVarRepositoryImpl) FindByID(ctx context.Context, envVarID string) (*models.EnvVar, error) {
	var envVarData models.EnvVar                                                              // env_var を格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&envVarData, "id = ?", envVarID).Error; err != nil { // db から env_var を取得する
		return nil, err // 取得エラーを返す
	}
	return &envVarData, nil // env_var を返す
}

// FindAllByProjectID は projectID に紐づく env_var 一覧を返す
func (repo *envVarRepositoryImpl) FindAllByProjectID(ctx context.Context, projectID string) ([]*models.EnvVar, error) {
	var envVarList []*models.EnvVar                                                                          // env_var 一覧を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&envVarList).Error; err != nil { // db から一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return envVarList, nil // env_var 一覧を返す
}

// FindByIDForUpdate は envVarID に対応する env_var を FOR UPDATE ロックで返す（トランザクション内用）
func (repo *envVarRepositoryImpl) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, envVarID string) (*models.EnvVar, error) {
	var envVarData models.EnvVar                                                                                            // env_var を格納する変数を定義する
	if err := tx.WithContext(ctx).Set("gorm:query_option", "FOR UPDATE").First(&envVarData, "id = ?", envVarID).Error; err != nil { // tx で FOR UPDATE ロックして取得する
		return nil, err // 取得エラーを返す
	}
	return &envVarData, nil // env_var を返す
}

// ExistsByProjectIDAndKey は同一プロジェクト内で key が重複しているか確認する（excludeID に自分自身の ID を渡すと自己除外できる）
func (repo *envVarRepositoryImpl) ExistsByProjectIDAndKey(ctx context.Context, projectID string, key string, excludeID string) (bool, error) {
	var count int64                                                                                                                                          // 件数格納用変数を定義する
	query := repo.db.WithContext(ctx).Model(&models.EnvVar{}).Where("project_id = ? AND key = ?", projectID, key) // プロジェクトとキーで絞り込む
	if excludeID != "" {                                                                                                                                     // excludeID が指定されている場合は自己除外する
		query = query.Where("id != ?", excludeID) // 自分自身を除外する
	}
	if err := query.Count(&count).Error; err != nil { // 件数を取得する
		return false, err // 取得エラーを返す
	}
	return count > 0, nil // 1件以上あれば重複と判定する
}

// Update は env_var レコードを更新する
func (repo *envVarRepositoryImpl) Update(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
	return tx.WithContext(ctx).Save(envVar).Error // tx を使って保存する
}

// Delete は env_var レコードを削除する
func (repo *envVarRepositoryImpl) Delete(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
	return tx.WithContext(ctx).Delete(envVar).Error // tx を使って削除する
}
