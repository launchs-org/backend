package repository

import (
	"handler/models"
	"context"

	"gorm.io/gorm"
)

// DeploymentTemplateRepository は deployment_templates テーブルへのアクセスを定義するインターフェース
type DeploymentTemplateRepository interface {
	FindAll(ctx context.Context) ([]*models.DeploymentTemplate, error)               // テンプレート一覧を取得する
	FindByID(ctx context.Context, templateID string) (*models.DeploymentTemplate, error) // テンプレートを ID で取得する
	Create(ctx context.Context, template *models.DeploymentTemplate) error           // テンプレートを作成する
	Update(ctx context.Context, template *models.DeploymentTemplate) error           // テンプレートを更新する
	Delete(ctx context.Context, templateID string) error                             // テンプレートを削除する
	Upsert(ctx context.Context, template *models.DeploymentTemplate) error           // 起動時シード用 upsert（name が一致すれば更新、なければ作成）
}

// deploymentTemplateRepositoryImpl は DeploymentTemplateRepository の GORM 実装
type deploymentTemplateRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewDeploymentTemplateRepository は DeploymentTemplateRepository の実装を返す
func NewDeploymentTemplateRepository(db *gorm.DB) DeploymentTemplateRepository {
	return &deploymentTemplateRepositoryImpl{db: db} // 実装を生成して返す
}

// FindAll は deployment_templates テーブルの全レコードを返す
func (repo *deploymentTemplateRepositoryImpl) FindAll(ctx context.Context) ([]*models.DeploymentTemplate, error) {
	var templateList []*models.DeploymentTemplate                                        // テンプレート一覧を格納するスライスを定義する
	if err := repo.db.WithContext(ctx).Find(&templateList).Error; err != nil {           // db からテンプレート一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return templateList, nil // テンプレート一覧を返す
}

// FindByID は templateID に対応するテンプレートを返す
func (repo *deploymentTemplateRepositoryImpl) FindByID(ctx context.Context, templateID string) (*models.DeploymentTemplate, error) {
	var templateData models.DeploymentTemplate                                                                 // テンプレートを格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&templateData, "id = ?", templateID).Error; err != nil {         // db からテンプレートを取得する
		return nil, err // 取得エラーを返す
	}
	return &templateData, nil // テンプレートを返す
}

// Create はテンプレートレコードを作成する
func (repo *deploymentTemplateRepositoryImpl) Create(ctx context.Context, template *models.DeploymentTemplate) error {
	return repo.db.WithContext(ctx).Create(template).Error // db を使って作成する
}

// Update はテンプレートレコードを更新する
func (repo *deploymentTemplateRepositoryImpl) Update(ctx context.Context, template *models.DeploymentTemplate) error {
	return repo.db.WithContext(ctx).Save(template).Error // db を使って保存する
}

// Delete は templateID に対応するテンプレートを削除する
func (repo *deploymentTemplateRepositoryImpl) Delete(ctx context.Context, templateID string) error {
	return repo.db.WithContext(ctx).Delete(&models.DeploymentTemplate{}, "id = ?", templateID).Error // db から削除する
}

// Upsert は name が一致するレコードを更新し、存在しなければ作成する（起動時シード用）
func (repo *deploymentTemplateRepositoryImpl) Upsert(ctx context.Context, template *models.DeploymentTemplate) error {
	return repo.db.WithContext(ctx).
		Where("name = ?", template.Name).
		Assign(template).
		FirstOrCreate(template).Error // name が一致すれば更新、なければ作成する
}
