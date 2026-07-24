package repository

import (
	"handler/models"
	"context"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// VolumeRepository は volumes テーブルへのアクセスを定義するインターフェース
type VolumeRepository interface {
	Create(ctx context.Context, tx *gorm.DB, volume *models.Volume) error                                              // volume を作成する
	FindByID(ctx context.Context, volumeID string) (*models.Volume, error)                                             // volume を ID で取得する
	FindAllByProjectID(ctx context.Context, projectID string) ([]*models.Volume, error)                                // projectID に紐づく volume 一覧を取得する
	Delete(ctx context.Context, tx *gorm.DB, volume *models.Volume) error                                              // volume を削除する
	UpdateStatus(ctx context.Context, volumeID string, status models.VolumeStatus, k8sStatus datatypes.JSON) error     // volume の status と k8s_status を更新する
}

// volumeRepositoryImpl は VolumeRepository の GORM 実装
type volumeRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewVolumeRepository は VolumeRepository の実装を返す
func NewVolumeRepository(db *gorm.DB) VolumeRepository {
	return &volumeRepositoryImpl{db: db} // 実装を生成して返す
}

// Create は volume レコードを作成する
func (repo *volumeRepositoryImpl) Create(ctx context.Context, tx *gorm.DB, volume *models.Volume) error {
	return tx.WithContext(ctx).Create(volume).Error // tx を使って作成する
}

// FindByID は volumeID に対応する volume を返す
func (repo *volumeRepositoryImpl) FindByID(ctx context.Context, volumeID string) (*models.Volume, error) {
	var volumeData models.Volume                                                                    // volume を格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&volumeData, "id = ?", volumeID).Error; err != nil { // db から volume を取得する
		return nil, err // 取得エラーを返す
	}
	return &volumeData, nil // volume を返す
}

// FindAllByProjectID は projectID に紐づく volume 一覧を返す
func (repo *volumeRepositoryImpl) FindAllByProjectID(ctx context.Context, projectID string) ([]*models.Volume, error) {
	var volumeList []*models.Volume                                                                             // volume 一覧を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&volumeList).Error; err != nil { // db から一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return volumeList, nil // volume 一覧を返す
}

// Delete は volume レコードを削除する
func (repo *volumeRepositoryImpl) Delete(ctx context.Context, tx *gorm.DB, volume *models.Volume) error {
	return tx.WithContext(ctx).Delete(volume).Error // tx を使って削除する
}

// UpdateStatus は volumeID に対応する volume の status と k8s_status を更新する
func (repo *volumeRepositoryImpl) UpdateStatus(ctx context.Context, volumeID string, status models.VolumeStatus, k8sStatus datatypes.JSON) error {
	result := repo.db.WithContext(ctx).Model(&models.Volume{}).Where("id = ?", volumeID).Updates(map[string]interface{}{ // status と k8s_status を更新する
		"status":     status,
		"k8s_status": k8sStatus,
	})
	if result.Error != nil { // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}
