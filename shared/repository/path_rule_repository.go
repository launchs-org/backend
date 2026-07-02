package repository

import (
	"app/shared/models"
	"context"

	"gorm.io/gorm"
)

// PathRuleRepository は path_rules テーブルへのアクセスを定義するインターフェース
type PathRuleRepository interface {
	Create(ctx context.Context, tx *gorm.DB, pathRule *models.PathRule) error                                              // path_rule を作成する
	FindByID(ctx context.Context, pathRuleID string) (*models.PathRule, error)                                             // ID に紐づく path_rule を取得する
	FindByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.PathRule, error)                           // IngressRoute に紐づく path_rule 一覧を取得する
	FindActiveAndPendingByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.PathRule, error)           // apply 時に使用する active/pending のみを取得する
	FindPendingByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.PathRule, error)                    // apply 後の昇格処理に使用する pending のみを取得する
	FindDeletingByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.PathRule, error)                   // apply 後の物理削除に使用する deleting のみを取得する
	UpdateStatus(ctx context.Context, tx *gorm.DB, pathRuleID string, status models.PathRuleStatus) error                 // path_rule の status を更新する
	Delete(ctx context.Context, tx *gorm.DB, pathRuleID string) error                                                     // path_rule を物理削除する
	DeleteByIngressRouteID(ctx context.Context, tx *gorm.DB, ingressRouteID string) error                                 // IngressRoute に紐づく path_rule を全件削除する
}

// pathRuleRepositoryImpl は PathRuleRepository の GORM 実装
type pathRuleRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewPathRuleRepository は PathRuleRepository の実装を返す
func NewPathRuleRepository(db *gorm.DB) PathRuleRepository {
	return &pathRuleRepositoryImpl{db: db} // 実装を生成して返す
}

// Create は path_rule レコードを作成する
func (repo *pathRuleRepositoryImpl) Create(ctx context.Context, tx *gorm.DB, pathRule *models.PathRule) error {
	db := repo.db // デフォルトは repo の db を使う
	if tx != nil { // トランザクションが渡された場合はそちらを使う
		db = tx
	}
	return db.WithContext(ctx).Create(pathRule).Error // db を使って作成する
}

// FindByID は pathRuleID に対応する path_rule を返す
func (repo *pathRuleRepositoryImpl) FindByID(ctx context.Context, pathRuleID string) (*models.PathRule, error) {
	var pathRuleData models.PathRule                                                                        // path_rule を格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&pathRuleData, "id = ?", pathRuleID).Error; err != nil {      // db から path_rule を取得する
		return nil, err // 取得エラーを返す
	}
	return &pathRuleData, nil // path_rule を返す
}

// FindByIngressRouteID は IngressRoute に紐づく path_rule 一覧を返す
func (repo *pathRuleRepositoryImpl) FindByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.PathRule, error) {
	var pathRuleList []*models.PathRule                                                                                    // path_rule 一覧を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("ingress_route_id = ?", ingressRouteID).Find(&pathRuleList).Error; err != nil { // db から一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return pathRuleList, nil // path_rule 一覧を返す
}

// FindActiveAndPendingByIngressRouteID は status が active または pending の path_rule 一覧を返す
func (repo *pathRuleRepositoryImpl) FindActiveAndPendingByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.PathRule, error) {
	var pathRuleList []*models.PathRule                                                       // path_rule 一覧を格納する変数を定義する
	err := repo.db.WithContext(ctx).
		Where("ingress_route_id = ? AND status IN ?", ingressRouteID, []string{
			string(models.PathRuleStatusActive),
			string(models.PathRuleStatusPending),
		}).
		Find(&pathRuleList).Error // active / pending のみを取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return pathRuleList, nil // path_rule 一覧を返す
}

// FindPendingByIngressRouteID は status が pending の path_rule 一覧を返す
func (repo *pathRuleRepositoryImpl) FindPendingByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.PathRule, error) {
	var pathRuleList []*models.PathRule                                                                                                                      // path_rule 一覧を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("ingress_route_id = ? AND status = ?", ingressRouteID, models.PathRuleStatusPending).Find(&pathRuleList).Error; err != nil { // pending のみを取得する
		return nil, err // 取得エラーを返す
	}
	return pathRuleList, nil // path_rule 一覧を返す
}

// FindDeletingByIngressRouteID は status が deleting の path_rule 一覧を返す
func (repo *pathRuleRepositoryImpl) FindDeletingByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.PathRule, error) {
	var pathRuleList []*models.PathRule                                                                                                                       // path_rule 一覧を格納する変数を定義する
	if err := repo.db.WithContext(ctx).Where("ingress_route_id = ? AND status = ?", ingressRouteID, models.PathRuleStatusDeleting).Find(&pathRuleList).Error; err != nil { // deleting のみを取得する
		return nil, err // 取得エラーを返す
	}
	return pathRuleList, nil // path_rule 一覧を返す
}

// UpdateStatus は pathRuleID に対応する path_rule の status を更新する
func (repo *pathRuleRepositoryImpl) UpdateStatus(ctx context.Context, tx *gorm.DB, pathRuleID string, status models.PathRuleStatus) error {
	db := repo.db // デフォルトは repo の db を使う
	if tx != nil { // トランザクションが渡された場合はそちらを使う
		db = tx
	}
	result := db.WithContext(ctx).Model(&models.PathRule{}).Where("id = ?", pathRuleID).Update("status", status) // status を更新する
	if result.Error != nil {                                                                                       // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// Delete は pathRuleID に対応する path_rule レコードを物理削除する
func (repo *pathRuleRepositoryImpl) Delete(ctx context.Context, tx *gorm.DB, pathRuleID string) error {
	db := repo.db // デフォルトは repo の db を使う
	if tx != nil { // トランザクションが渡された場合はそちらを使う
		db = tx
	}
	result := db.WithContext(ctx).Delete(&models.PathRule{}, "id = ?", pathRuleID) // path_rule を削除する
	if result.Error != nil {                                                         // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 削除対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// DeleteByIngressRouteID は IngressRoute に紐づく path_rule を全件物理削除する
func (repo *pathRuleRepositoryImpl) DeleteByIngressRouteID(ctx context.Context, tx *gorm.DB, ingressRouteID string) error {
	db := repo.db // デフォルトは repo の db を使う
	if tx != nil { // トランザクションが渡された場合はそちらを使う
		db = tx
	}
	return db.WithContext(ctx).Delete(&models.PathRule{}, "ingress_route_id = ?", ingressRouteID).Error // IngressRoute に紐づく path_rule を全件削除する
}
