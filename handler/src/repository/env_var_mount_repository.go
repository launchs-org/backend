package repository

import (
	"handler/models"
	"context"

	"gorm.io/gorm"
)

// EnvVarMountRepository は env_var_mounts テーブルへのアクセスを定義するインターフェース
type EnvVarMountRepository interface {
	Create(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error                            // マウント設定を作成する
	FindByID(ctx context.Context, mountID string) (*models.EnvVarMount, error)                           // ID でマウント設定を取得する
	FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.EnvVarMount, error)       // deploymentID に紐づくマウント設定一覧を取得する
	FindByDeploymentIDAndEnvVarID(ctx context.Context, deploymentID string, envVarID string) (*models.EnvVarMount, error) // deploymentID と envVarID でマウント設定を取得する
	UpdateStatus(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount, status models.EnvVarMountStatus) error // ステータスを更新する
	Delete(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error                                        // マウント設定を削除する
	DeleteAllByDeploymentID(ctx context.Context, tx *gorm.DB, deploymentID string) error                             // deploymentID に紐づくマウント設定を全件削除する
	CountByEnvVarID(ctx context.Context, envVarID string) (int64, error)                                             // envVarID を参照しているマウント件数を数える
}

// envVarMountRepositoryImpl は EnvVarMountRepository の GORM 実装
type envVarMountRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewEnvVarMountRepository は EnvVarMountRepository の実装を返す
func NewEnvVarMountRepository(db *gorm.DB) EnvVarMountRepository {
	return &envVarMountRepositoryImpl{db: db} // 実装を生成して返す
}

// Create は env_var_mounts レコードを作成する
func (repo *envVarMountRepositoryImpl) Create(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error {
	return tx.WithContext(ctx).Create(mount).Error // tx を使って作成する
}

// FindByID は mountID に対応するマウント設定を返す
func (repo *envVarMountRepositoryImpl) FindByID(ctx context.Context, mountID string) (*models.EnvVarMount, error) {
	var mountData models.EnvVarMount                                                                          // マウント設定を格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&mountData, "id = ?", mountID).Error; err != nil {               // db からマウント設定を取得する
		return nil, err // 取得エラーを返す
	}
	return &mountData, nil // マウント設定を返す
}

// FindAllByDeploymentID は deploymentID に紐づくマウント設定一覧を返す
func (repo *envVarMountRepositoryImpl) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.EnvVarMount, error) {
	var mountList []*models.EnvVarMount                                                                                              // マウント設定一覧を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("deployment_id = ?", deploymentID).Find(&mountList).Error; err != nil {                 // db から一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return mountList, nil // マウント設定一覧を返す
}

// FindByDeploymentIDAndEnvVarID は deploymentID と envVarID に対応するマウント設定を返す（重複チェック用）
func (repo *envVarMountRepositoryImpl) FindByDeploymentIDAndEnvVarID(ctx context.Context, deploymentID string, envVarID string) (*models.EnvVarMount, error) {
	var mountData models.EnvVarMount                                                                                                                         // マウント設定を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("deployment_id = ? AND env_var_id = ?", deploymentID, envVarID).First(&mountData).Error; err != nil {           // db から重複レコードを検索する
		return nil, err // 取得エラーを返す
	}
	return &mountData, nil // マウント設定を返す
}

// UpdateStatus はマウント設定のステータスを更新する
func (repo *envVarMountRepositoryImpl) UpdateStatus(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount, status models.EnvVarMountStatus) error {
	db := repo.db // tx が nil の場合は repo.db を使う
	if tx != nil {
		db = tx
	}
	mount.Status = status                        // ステータスを更新する
	return db.WithContext(ctx).Save(mount).Error // db を使って保存する
}

// Delete はマウント設定レコードを削除する
func (repo *envVarMountRepositoryImpl) Delete(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error {
	db := repo.db // tx が nil の場合は repo.db を使う
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Delete(mount).Error // db を使って削除する
}

// DeleteAllByDeploymentID は deploymentID に紐づくマウント設定を全件削除する
func (repo *envVarMountRepositoryImpl) DeleteAllByDeploymentID(ctx context.Context, tx *gorm.DB, deploymentID string) error {
	db := repo.db // tx が nil の場合は repo.db を使う
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Where("deployment_id = ?", deploymentID).Delete(&models.EnvVarMount{}).Error // db を使って一括削除する
}

// CountByEnvVarID は envVarID を参照しているマウント件数を返す（他デプロイメントからの参照有無判定用）
func (repo *envVarMountRepositoryImpl) CountByEnvVarID(ctx context.Context, envVarID string) (int64, error) {
	var count int64                                                                                                          // 件数格納用変数を定義する
	if err := repo.db.WithContext(ctx).Model(&models.EnvVarMount{}).Where("env_var_id = ?", envVarID).Count(&count).Error; err != nil { // db から件数を取得する
		return 0, err // 取得エラーを返す
	}
	return count, nil // 件数を返す
}
