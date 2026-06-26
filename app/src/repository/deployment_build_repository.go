package repository

import (
	"app/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// DeploymentBuildRepository は deployment_builds テーブルへのアクセスを定義するインターフェース
type DeploymentBuildRepository interface {
	Create(ctx context.Context, build *models.DeploymentBuild) error                                                                                                  // ビルドレコードを作成する
	FindByID(ctx context.Context, buildID string) (*models.DeploymentBuild, error)                                                                                    // ID でビルドレコードを取得する
	FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]models.DeploymentBuild, error)                                                                 // deploymentID に紐づくビルド一覧を取得する
	FindAllByProjectID(ctx context.Context, projectID string) ([]models.DeploymentBuild, error)                                                                       // projectID に紐づくビルド一覧を取得する
	FindAllBuilding(ctx context.Context) ([]models.DeploymentBuild, error)                                                                                            // building 状態のビルドを全件取得する（Watcher 起動時リカバリ用）
	UpdateStatus(ctx context.Context, buildID string, status models.BuildStatus) error                                                                                // ビルドのステータスを更新する
	UpdateK8sJobName(ctx context.Context, buildID string, jobName string) error                                                                                       // k8s Job 名を更新する
	UpdateBuildResult(ctx context.Context, buildID string, status models.BuildStatus, builtImageURL string, imageSizeBytes int64, finishedAt time.Time) error         // 完了時に status / built_image_url / image_size_bytes / finished_at を一括更新する
	Delete(ctx context.Context, build *models.DeploymentBuild) error                                                                                                  // ビルドレコードを1件削除する
	DeleteAllByDeploymentID(ctx context.Context, deploymentID string) error                                                                                           // deploymentID に紐づくビルドを全件削除する
	DeleteAllByProjectID(ctx context.Context, db *gorm.DB, projectID string) error                                                                                    // projectID に紐づくビルドを全件削除する（Project 削除時に使用）
}

// deploymentBuildRepositoryImpl は DeploymentBuildRepository の GORM 実装
type deploymentBuildRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewDeploymentBuildRepository は DeploymentBuildRepository の実装を返す
func NewDeploymentBuildRepository(db *gorm.DB) DeploymentBuildRepository {
	return &deploymentBuildRepositoryImpl{db: db} // 実装を生成して返す
}

// Create はビルドレコードを作成する
func (repo *deploymentBuildRepositoryImpl) Create(ctx context.Context, build *models.DeploymentBuild) error {
	return repo.db.WithContext(ctx).Create(build).Error // db を使って作成する
}

// FindByID は buildID に対応するビルドレコードを返す
func (repo *deploymentBuildRepositoryImpl) FindByID(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
	var buildData models.DeploymentBuild                                                               // ビルドレコードを格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&buildData, "id = ?", buildID).Error; err != nil { // db からビルドレコードを取得する
		return nil, err // 取得エラーを返す
	}
	return &buildData, nil // ビルドレコードを返す
}

// FindAllByDeploymentID は deploymentID に紐づくビルド一覧を返す
func (repo *deploymentBuildRepositoryImpl) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]models.DeploymentBuild, error) {
	var buildList []models.DeploymentBuild                                                                                    // ビルド一覧を格納するスライスを定義する
	if err := repo.db.WithContext(ctx).Where("deployment_id = ?", deploymentID).Find(&buildList).Error; err != nil { // db からビルド一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return buildList, nil // ビルド一覧を返す
}

// UpdateStatus は buildID に対応するビルドのステータスを更新する
func (repo *deploymentBuildRepositoryImpl) UpdateStatus(ctx context.Context, buildID string, status models.BuildStatus) error {
	result := repo.db.WithContext(ctx).Model(&models.DeploymentBuild{}).Where("id = ?", buildID).Update("status", status) // ステータスを更新する
	if result.Error != nil {                                                                                                // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// UpdateK8sJobName は buildID に対応するビルドの k8s Job 名を更新する
func (repo *deploymentBuildRepositoryImpl) UpdateK8sJobName(ctx context.Context, buildID string, jobName string) error {
	result := repo.db.WithContext(ctx).Model(&models.DeploymentBuild{}).Where("id = ?", buildID).Update("k8s_job_name", jobName) // Job 名を更新する
	if result.Error != nil {                                                                                                       // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// FindAllBuilding は status が building のビルドレコードを全件返す
func (repo *deploymentBuildRepositoryImpl) FindAllBuilding(ctx context.Context) ([]models.DeploymentBuild, error) {
	var buildList []models.DeploymentBuild                                                                                                    // ビルド一覧を格納するスライスを定義する
	if err := repo.db.WithContext(ctx).Where("status = ?", models.BuildStatusBuilding).Find(&buildList).Error; err != nil { // building 状態のビルドを取得する
		return nil, err // 取得エラーを返す
	}
	return buildList, nil // ビルド一覧を返す
}

// DeleteAllByDeploymentID は deploymentID に紐づくビルドレコードを全件削除する
func (repo *deploymentBuildRepositoryImpl) DeleteAllByDeploymentID(ctx context.Context, deploymentID string) error {
	return repo.db.WithContext(ctx).Where("deployment_id = ?", deploymentID).Delete(&models.DeploymentBuild{}).Error // 全件削除する
}

// FindAllByProjectID は projectID に紐づくビルド一覧を返す
func (repo *deploymentBuildRepositoryImpl) FindAllByProjectID(ctx context.Context, projectID string) ([]models.DeploymentBuild, error) {
	var buildList []models.DeploymentBuild                                                                                  // ビルド一覧を格納するスライスを定義する
	if err := repo.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Find(&buildList).Error; err != nil { // db からビルド一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return buildList, nil // ビルド一覧を返す
}

// DeleteAllByProjectID は projectID に紐づくビルドレコードを全件削除する（Project 削除時に使用）
func (repo *deploymentBuildRepositoryImpl) DeleteAllByProjectID(ctx context.Context, db *gorm.DB, projectID string) error {
	return db.WithContext(ctx).Where("project_id = ?", projectID).Delete(&models.DeploymentBuild{}).Error // 全件削除する
}

// Delete はビルドレコードを1件削除する
func (repo *deploymentBuildRepositoryImpl) Delete(ctx context.Context, build *models.DeploymentBuild) error {
	return repo.db.WithContext(ctx).Delete(build).Error // レコードを削除する
}

// UpdateBuildResult は buildID に対応するビルドの status / built_image_url / image_size_bytes / finished_at を一括更新する
func (repo *deploymentBuildRepositoryImpl) UpdateBuildResult(ctx context.Context, buildID string, status models.BuildStatus, builtImageURL string, imageSizeBytes int64, finishedAt time.Time) error {
	result := repo.db.WithContext(ctx).Model(&models.DeploymentBuild{}).Where("id = ?", buildID).Updates(map[string]interface{}{ // 複数フィールドを一括更新する
		"status":           status,          // ビルドステータスを更新する
		"built_image_url":  builtImageURL,   // ビルド済みイメージURLを更新する
		"image_size_bytes": imageSizeBytes,  // イメージサイズを更新する
		"finished_at":      finishedAt,      // 完了日時を更新する
	})
	if result.Error != nil { // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}
