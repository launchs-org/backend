package repository

import (
	"app/models"
	"context"

	"gorm.io/gorm"
)

// VolumeMountRepository は volume_mounts テーブルへのアクセスを定義するインターフェース
type VolumeMountRepository interface {
	Create(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error                               // マウント設定を作成する
	FindByID(ctx context.Context, mountID string) (*models.VolumeMount, error)                              // ID でマウント設定を取得する
	FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.VolumeMount, error)          // deploymentID に紐づくマウント設定一覧を取得する
	FindAllByVolumeID(ctx context.Context, volumeID string) ([]*models.VolumeMount, error)                  // volumeID に紐づくマウント設定一覧を取得する
	FindByDeploymentIDAndMountPath(ctx context.Context, deploymentID string, mountPath string) (*models.VolumeMount, error) // deploymentID と mountPath でマウント設定を取得する（重複チェック用）
	UpdateStatus(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount, status models.VolumeMountStatus) error // ステータスを更新する
	Delete(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error                               // マウント設定を削除する
	DeleteAllByDeploymentID(ctx context.Context, tx *gorm.DB, deploymentID string) error                   // deploymentID に紐づくマウント設定を全件削除する
}

// volumeMountRepositoryImpl は VolumeMountRepository の GORM 実装
type volumeMountRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewVolumeMountRepository は VolumeMountRepository の実装を返す
func NewVolumeMountRepository(db *gorm.DB) VolumeMountRepository {
	return &volumeMountRepositoryImpl{db: db} // 実装を生成して返す
}

// Create は volume_mounts レコードを作成する
func (repo *volumeMountRepositoryImpl) Create(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error {
	return tx.WithContext(ctx).Create(mount).Error // tx を使って作成する
}

// FindByID は mountID に対応するマウント設定を返す
func (repo *volumeMountRepositoryImpl) FindByID(ctx context.Context, mountID string) (*models.VolumeMount, error) {
	var mountData models.VolumeMount                                                                         // マウント設定を格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&mountData, "id = ?", mountID).Error; err != nil {              // db からマウント設定を取得する
		return nil, err // 取得エラーを返す
	}
	return &mountData, nil // マウント設定を返す
}

// FindAllByDeploymentID は deploymentID に紐づくマウント設定一覧を返す
func (repo *volumeMountRepositoryImpl) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.VolumeMount, error) {
	var mountList []*models.VolumeMount                                                                                              // マウント設定一覧を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("deployment_id = ?", deploymentID).Find(&mountList).Error; err != nil {                 // db から一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return mountList, nil // マウント設定一覧を返す
}

// FindAllByVolumeID は volumeID に紐づくマウント設定一覧を返す
func (repo *volumeMountRepositoryImpl) FindAllByVolumeID(ctx context.Context, volumeID string) ([]*models.VolumeMount, error) {
	var mountList []*models.VolumeMount                                                                                          // マウント設定一覧を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("volume_id = ?", volumeID).Find(&mountList).Error; err != nil {                     // db から一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return mountList, nil // マウント設定一覧を返す
}

// FindByDeploymentIDAndMountPath は deploymentID と mountPath に対応するマウント設定を返す（重複チェック用）
func (repo *volumeMountRepositoryImpl) FindByDeploymentIDAndMountPath(ctx context.Context, deploymentID string, mountPath string) (*models.VolumeMount, error) {
	var mountData models.VolumeMount                                                                                                                               // マウント設定を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("deployment_id = ? AND mount_path = ?", deploymentID, mountPath).First(&mountData).Error; err != nil {               // db から重複レコードを検索する
		return nil, err // 取得エラーを返す
	}
	return &mountData, nil // マウント設定を返す
}

// UpdateStatus はマウント設定のステータスを更新する
func (repo *volumeMountRepositoryImpl) UpdateStatus(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount, status models.VolumeMountStatus) error {
	db := repo.db // tx が nil の場合は repo.db を使う
	if tx != nil {
		db = tx
	}
	mount.Status = status                        // ステータスをセットする
	return db.WithContext(ctx).Save(mount).Error // db を使って更新する
}

// DeleteAllByDeploymentID は deploymentID に紐づくマウント設定を全件削除する
func (repo *volumeMountRepositoryImpl) DeleteAllByDeploymentID(ctx context.Context, tx *gorm.DB, deploymentID string) error {
	db := repo.db // tx が nil の場合は repo.db を使う
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Where("deployment_id = ?", deploymentID).Delete(&models.VolumeMount{}).Error // deploymentID で一括削除する
}

// Delete はマウント設定レコードを削除する
func (repo *volumeMountRepositoryImpl) Delete(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error {
	db := repo.db // tx が nil の場合は repo.db を使う
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Delete(mount).Error // db を使って削除する
}
