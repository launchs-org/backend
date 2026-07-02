package repository

import (
	"handler/models"
	"context"

	"gorm.io/gorm"
)

// ProjectRepository は projects テーブルへのアクセスを定義するインターフェース
type ProjectRepository interface {
	Create(ctx context.Context, tx *gorm.DB, project *models.Project) error                                   // project を作成する
	FindByID(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error)                    // project を ID で取得する（トランザクション内用）
	FindByIDNoTx(ctx context.Context, projectID string) (*models.Project, error)                             // project を ID で取得する（トランザクション外用）
	FindByNamespace(ctx context.Context, namespace string) (*models.Project, error)                          // namespace 名に対応する project を取得する
	FindAllByUserID(ctx context.Context, userID string) ([]*models.Project, error)                            // userID に紐づく project 一覧を取得する
	UpdateStatus(ctx context.Context, tx *gorm.DB, project *models.Project, status models.ProjectStatus) error // project のステータスを更新する
	Save(ctx context.Context, project *models.Project) error                                                  // project を保存する
	Delete(ctx context.Context, tx *gorm.DB, project *models.Project) error                                   // project を削除する（トランザクション内用）
	DeleteNoTx(ctx context.Context, project *models.Project) error                                            // project を削除する（トランザクション外用）
}

// projectRepositoryImpl は ProjectRepository の GORM 実装
type projectRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewProjectRepository は ProjectRepository の実装を返す
func NewProjectRepository(db *gorm.DB) ProjectRepository {
	return &projectRepositoryImpl{db: db} // 実装を生成して返す
}

// Create は project レコードを作成する
func (repo *projectRepositoryImpl) Create(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return tx.WithContext(ctx).Create(project).Error // tx を使って作成する
}

// FindByID は projectID に対応する project を返す（トランザクション内用）
func (repo *projectRepositoryImpl) FindByID(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error) {
	var projectData models.Project
	err := tx.WithContext(ctx).First(&projectData, "id = ?", projectID).Error // tx を使って取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return &projectData, nil
}

// FindByIDNoTx は projectID に対応する project を返す（トランザクション外用）
func (repo *projectRepositoryImpl) FindByIDNoTx(ctx context.Context, projectID string) (*models.Project, error) {
	var projectData models.Project
	err := repo.db.WithContext(ctx).First(&projectData, "id = ?", projectID).Error // db を使って取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return &projectData, nil
}

// FindByNamespace は namespace 名に対応する project を返す
func (repo *projectRepositoryImpl) FindByNamespace(ctx context.Context, namespace string) (*models.Project, error) {
	var projectData models.Project
	err := repo.db.WithContext(ctx).First(&projectData, "namespace = ?", namespace).Error // db を使って namespace で取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return &projectData, nil
}

// FindAllByUserID は userID に紐づく削除中以外の project 一覧を返す
func (repo *projectRepositoryImpl) FindAllByUserID(ctx context.Context, userID string) ([]*models.Project, error) {
	var projectList []*models.Project
	err := repo.db.WithContext(ctx).
		Where("user_id = ? AND status != ?", userID, models.ProjectStatusDeleting). // 削除中を除外する
		Find(&projectList).Error
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return projectList, nil
}

// UpdateStatus は project のステータスを更新する
func (repo *projectRepositoryImpl) UpdateStatus(ctx context.Context, tx *gorm.DB, project *models.Project, status models.ProjectStatus) error {
	return tx.WithContext(ctx).Model(project).Update("status", status).Error // tx を使って更新する
}

// Save は project レコードを保存する（トランザクション外の更新に使用する）
func (repo *projectRepositoryImpl) Save(ctx context.Context, project *models.Project) error {
	return repo.db.WithContext(ctx).Save(project).Error // db を使って保存する
}

// Delete は project レコードを削除する（トランザクション内用）
func (repo *projectRepositoryImpl) Delete(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return tx.WithContext(ctx).Delete(project).Error // tx を使って削除する
}

// DeleteNoTx は project レコードを削除する（トランザクション外用）
func (repo *projectRepositoryImpl) DeleteNoTx(ctx context.Context, project *models.Project) error {
	return repo.db.WithContext(ctx).Delete(project).Error // db を使って削除する
}
