package repository

import (
	"app/models"
	"context"

	"gorm.io/gorm"
)

// IngressRouteRouteRepository は ingress_route_routes テーブルへのアクセスを定義するインターフェース
type IngressRouteRouteRepository interface {
	Create(ctx context.Context, tx *gorm.DB, route *models.IngressRouteRoute) error                                            // ルートエントリを作成する
	FindByID(ctx context.Context, routeID string) (*models.IngressRouteRoute, error)                                           // ID に紐づくルートエントリを取得する
	FindByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.IngressRouteRoute, error)                      // IngressRoute ID に紐づく全ルートエントリを取得する
	FindActiveAndPendingByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.IngressRouteRoute, error)      // active・pending のルートエントリを取得する（apply 時に k8s へ反映する対象）
	Update(ctx context.Context, route *models.IngressRouteRoute) error                                                         // ルートエントリを更新する
	UpdateStatus(ctx context.Context, tx *gorm.DB, routeID string, status models.IngressRouteRouteStatus) error               // ルートエントリのステータスを更新する
	Delete(ctx context.Context, tx *gorm.DB, routeID string) error                                                             // ルートエントリを物理削除する
	DeleteByIngressRouteID(ctx context.Context, tx *gorm.DB, ingressRouteID string) error                                      // IngressRoute ID に紐づく全ルートエントリを物理削除する
}

// ingressRouteRouteRepositoryImpl は IngressRouteRouteRepository の GORM 実装
type ingressRouteRouteRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewIngressRouteRouteRepository は IngressRouteRouteRepository の実装を返す
func NewIngressRouteRouteRepository(db *gorm.DB) IngressRouteRouteRepository {
	return &ingressRouteRouteRepositoryImpl{db: db} // 実装を生成して返す
}

// dbOrTx は tx が nil でない場合は tx を、nil の場合は repo.db を返す
func (repo *ingressRouteRouteRepositoryImpl) dbOrTx(tx *gorm.DB) *gorm.DB {
	if tx != nil { // トランザクションが渡されている場合はそちらを使う
		return tx
	}
	return repo.db // トランザクションがない場合は通常の DB を使う
}

// Create はルートエントリレコードを作成する
func (repo *ingressRouteRouteRepositoryImpl) Create(ctx context.Context, tx *gorm.DB, route *models.IngressRouteRoute) error {
	return repo.dbOrTx(tx).WithContext(ctx).Create(route).Error // db を使って作成する
}

// FindByID は routeID に対応するルートエントリを返す
func (repo *ingressRouteRouteRepositoryImpl) FindByID(ctx context.Context, routeID string) (*models.IngressRouteRoute, error) {
	var routeData models.IngressRouteRoute                                                                         // ルートエントリを格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&routeData, "id = ?", routeID).Error; err != nil { // db からルートエントリを取得する
		return nil, err // 取得エラーを返す
	}
	return &routeData, nil // ルートエントリを返す
}

// FindByIngressRouteID は ingressRouteID に対応する全ルートエントリを返す
func (repo *ingressRouteRouteRepositoryImpl) FindByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.IngressRouteRoute, error) {
	var routeList []*models.IngressRouteRoute                                                                                                                 // ルートエントリ一覧を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("ingress_route_id = ?", ingressRouteID).Order("created_at asc").Find(&routeList).Error; err != nil { // db からルートエントリ一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return routeList, nil // ルートエントリ一覧を返す
}

// FindActiveAndPendingByIngressRouteID は active・pending 状態のルートエントリを返す（deleting は除外）
func (repo *ingressRouteRouteRepositoryImpl) FindActiveAndPendingByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.IngressRouteRoute, error) {
	var routeList []*models.IngressRouteRoute                                                                                                                                                                                              // ルートエントリ一覧を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("ingress_route_id = ? AND status IN ?", ingressRouteID, []string{string(models.IngressRouteRouteStatusActive), string(models.IngressRouteRouteStatusPending)}).Order("created_at asc").Find(&routeList).Error; err != nil { // active・pending のみ取得する
		return nil, err // 取得エラーを返す
	}
	return routeList, nil // ルートエントリ一覧を返す
}

// Update はルートエントリレコードを保存する
func (repo *ingressRouteRouteRepositoryImpl) Update(ctx context.Context, route *models.IngressRouteRoute) error {
	return repo.db.WithContext(ctx).Save(route).Error // db を使って保存する
}

// UpdateStatus は routeID に対応するルートエントリのステータスを更新する
func (repo *ingressRouteRouteRepositoryImpl) UpdateStatus(ctx context.Context, tx *gorm.DB, routeID string, status models.IngressRouteRouteStatus) error {
	result := repo.dbOrTx(tx).WithContext(ctx).Model(&models.IngressRouteRoute{}).Where("id = ?", routeID).Update("status", status) // ステータスを更新する
	if result.Error != nil {                                                                                                          // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// Delete は routeID に対応するルートエントリを物理削除する
func (repo *ingressRouteRouteRepositoryImpl) Delete(ctx context.Context, tx *gorm.DB, routeID string) error {
	result := repo.dbOrTx(tx).WithContext(ctx).Delete(&models.IngressRouteRoute{}, "id = ?", routeID) // ルートエントリを削除する
	if result.Error != nil {                                                                             // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 削除対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// DeleteByIngressRouteID は ingressRouteID に紐づく全ルートエントリを物理削除する
func (repo *ingressRouteRouteRepositoryImpl) DeleteByIngressRouteID(ctx context.Context, tx *gorm.DB, ingressRouteID string) error {
	return repo.dbOrTx(tx).WithContext(ctx).Delete(&models.IngressRouteRoute{}, "ingress_route_id = ?", ingressRouteID).Error // 全ルートエントリを削除する
}
