package repository

import (
	"app/shared/models"
	"context"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// IngressRouteRepository は ingress_routes テーブルへのアクセスを定義するインターフェース
type IngressRouteRepository interface {
	Create(ctx context.Context, tx *gorm.DB, ingressRoute *models.IngressRoute) error                                                       // ingress_route を作成する
	FindByID(ctx context.Context, ingressRouteID string) (*models.IngressRoute, error)                                                      // ID に紐づく ingress_route を取得する
	FindByProjectID(ctx context.Context, projectID string) (*models.IngressRoute, error)                                                    // projectID に紐づく ingress_route を1件取得する
	FindAllByProjectID(ctx context.Context, projectID string) ([]*models.IngressRoute, error)                                               // projectID に紐づく ingress_route を全件取得する
	Update(ctx context.Context, tx *gorm.DB, ingressRoute *models.IngressRoute) error                                                       // ingress_route を更新する
	UpdateStatus(ctx context.Context, ingressRouteID string, status models.IngressRouteStatus, k8sStatus datatypes.JSON) error              // ingress_route の status と k8s_status を更新する
	Delete(ctx context.Context, tx *gorm.DB, ingressRouteID string) error                                                                   // ingress_route を削除する
	ExistsByHost(ctx context.Context, tx *gorm.DB, host string) (bool, error)                                                               // ホスト名がすでに存在するか確認する（グローバル衝突チェック用）
	ExistsByNameInProject(ctx context.Context, tx *gorm.DB, projectID string, name string) (bool, error)                                    // プロジェクト内で同一名が存在するか確認する
}

// ingressRouteRepositoryImpl は IngressRouteRepository の GORM 実装
type ingressRouteRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewIngressRouteRepository は IngressRouteRepository の実装を返す
func NewIngressRouteRepository(db *gorm.DB) IngressRouteRepository {
	return &ingressRouteRepositoryImpl{db: db} // 実装を生成して返す
}

// Create は ingress_route レコードを作成する
func (repo *ingressRouteRepositoryImpl) Create(ctx context.Context, tx *gorm.DB, ingressRoute *models.IngressRoute) error {
	db := repo.db // デフォルトは repo の db を使う
	if tx != nil { // トランザクションが渡された場合はそちらを使う
		db = tx
	}
	return db.WithContext(ctx).Create(ingressRoute).Error // db を使って作成する
}

// FindByID は ingressRouteID に対応する ingress_route を返す
func (repo *ingressRouteRepositoryImpl) FindByID(ctx context.Context, ingressRouteID string) (*models.IngressRoute, error) {
	var ingressRouteData models.IngressRoute                                                                             // ingress_route を格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&ingressRouteData, "id = ?", ingressRouteID).Error; err != nil {           // db から ingress_route を取得する
		return nil, err // 取得エラーを返す
	}
	return &ingressRouteData, nil // ingress_route を返す
}

// FindByProjectID は projectID に対応する ingress_route を返す
func (repo *ingressRouteRepositoryImpl) FindByProjectID(ctx context.Context, projectID string) (*models.IngressRoute, error) {
	var ingressRouteData models.IngressRoute                                                                               // ingress_route を格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&ingressRouteData, "project_id = ?", projectID).Error; err != nil {          // db から ingress_route を取得する
		return nil, err // 取得エラーを返す
	}
	return &ingressRouteData, nil // ingress_route を返す
}

// FindAllByProjectID は projectID に対応する全 ingress_route を返す
func (repo *ingressRouteRepositoryImpl) FindAllByProjectID(ctx context.Context, projectID string) ([]*models.IngressRoute, error) {
	var ingressRouteList []*models.IngressRoute                                                                                // ingress_route 一覧を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&ingressRouteList).Error; err != nil {         // db から ingress_route 一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return ingressRouteList, nil // ingress_route 一覧を返す
}

// Update は ingress_route レコードを保存する
func (repo *ingressRouteRepositoryImpl) Update(ctx context.Context, tx *gorm.DB, ingressRoute *models.IngressRoute) error {
	db := repo.db // デフォルトは repo の db を使う
	if tx != nil { // トランザクションが渡された場合はそちらを使う
		db = tx
	}
	return db.WithContext(ctx).Save(ingressRoute).Error // db を使って保存する
}

// UpdateStatus は ingressRouteID に対応する ingress_route の status と k8s_status を更新する
func (repo *ingressRouteRepositoryImpl) UpdateStatus(ctx context.Context, ingressRouteID string, status models.IngressRouteStatus, k8sStatus datatypes.JSON) error {
	result := repo.db.WithContext(ctx).Model(&models.IngressRoute{}).Where("id = ?", ingressRouteID).Updates(map[string]interface{}{ // status と k8s_status を更新する
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

// ExistsByHost はホスト名がすでに登録されているか確認する
func (repo *ingressRouteRepositoryImpl) ExistsByHost(ctx context.Context, tx *gorm.DB, host string) (bool, error) {
	db := repo.db  // デフォルトは repo の db を使う
	if tx != nil { // トランザクションが渡された場合はそちらを使う
		db = tx
	}
	var count int64                                                              // レコード件数を格納する変数を定義する
	err := db.WithContext(ctx).Model(&models.IngressRoute{}).Where("host = ?", host).Count(&count).Error // ホスト名が一致するレコード数を取得する
	if err != nil {                                                              // エラーが発生した場合
		return false, err // エラーを返す
	}
	return count > 0, nil // 存在する場合は true を返す
}

// ExistsByNameInProject はプロジェクト内に同一名の ingress_route が存在するか確認する
func (repo *ingressRouteRepositoryImpl) ExistsByNameInProject(ctx context.Context, tx *gorm.DB, projectID string, name string) (bool, error) {
	db := repo.db  // デフォルトは repo の db を使う
	if tx != nil { // トランザクションが渡された場合はそちらを使う
		db = tx
	}
	var count int64                                                                                                                                    // レコード件数を格納する変数を定義する
	err := db.WithContext(ctx).Model(&models.IngressRoute{}).Where("project_id = ? AND name = ?", projectID, name).Count(&count).Error // プロジェクト内で同一名のレコード数を取得する
	if err != nil {                                                                                                                                    // エラーが発生した場合
		return false, err // エラーを返す
	}
	return count > 0, nil // 存在する場合は true を返す
}

// Delete は ingressRouteID に対応する ingress_route レコードを削除する
func (repo *ingressRouteRepositoryImpl) Delete(ctx context.Context, tx *gorm.DB, ingressRouteID string) error {
	db := repo.db // デフォルトは repo の db を使う
	if tx != nil { // トランザクションが渡された場合はそちらを使う
		db = tx
	}
	result := db.WithContext(ctx).Delete(&models.IngressRoute{}, "id = ?", ingressRouteID) // ingress_route を削除する
	if result.Error != nil {                                                                 // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 削除対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}
