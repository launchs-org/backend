package repository

import (
	"handler/models"
	"context"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// DeploymentRepository は deployments テーブルへのアクセスを定義するインターフェース
type DeploymentRepository interface {
	Create(ctx context.Context, deployment *models.Deployment) error                                              // deployment を作成する
	CreateWithTx(ctx context.Context, tx *gorm.DB, deployment *models.Deployment) error                           // トランザクション内で deployment を作成する
	FindByID(ctx context.Context, deploymentID string) (*models.Deployment, error)                                // deployment を ID で取得する
	FindByIDForUpdate(ctx context.Context, tx *gorm.DB, deploymentID string) (*models.Deployment, error)          // SELECT FOR UPDATE で deployment を取得する
	FindAllByProjectID(ctx context.Context, projectID string) ([]models.Deployment, error)                        // projectID に紐づく deployment 一覧を取得する
	FindAllRunning(ctx context.Context) ([]models.Deployment, error)                                              // status=running の deployment を全件取得する（watcher 起動時復旧用）
	Save(ctx context.Context, deployment *models.Deployment) error                                                // deployment を保存する
	Updates(ctx context.Context, tx *gorm.DB, deployment *models.Deployment, values map[string]interface{}) error // deployment を map で部分更新する
	UpdateAppStatus(ctx context.Context, deploymentID string, appStatus models.AppStatus) error                   // app_status を更新する
	UpdateK8sStatus(ctx context.Context, deploymentID string, k8sStatus datatypes.JSON) error                     // k8s_status を更新する
	UpdateDeleteProgress(ctx context.Context, deploymentID string, progress string) error                         // delete_progress を更新する（削除中のステップ名を保持する）
	UpdatePendingImageURL(ctx context.Context, deploymentID string, imageURL string) error                        // ビルド成功時に pending_image_url を更新する
	UpdatePendingGithubCommitSHA(ctx context.Context, deploymentID string, commitSHA string) error               // GitHub push 時に pending_github_commit_sha を更新する
	UpdatePendingGithubBuildFields(ctx context.Context, deploymentID string, repoURL string, branch string, commitSHA string, directory string) error // ビルド成功時に pending_github_* フィールドをまとめて更新する
	UpdateDeploymentStatus(ctx context.Context, deploymentID string, status models.DeploymentStatus) error       // deployment の status を更新する
	UpdateCurrentBuildID(ctx context.Context, deploymentID string, buildID string) error                          // current_build_id を更新する
	ClearCurrentBuildID(ctx context.Context, deploymentID string) error                                           // current_build_id を NULL にクリアする
	Delete(ctx context.Context, deploymentID string) error                                                        // deployment を削除する
}

// deploymentRepositoryImpl は DeploymentRepository の GORM 実装
type deploymentRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewDeploymentRepository は DeploymentRepository の実装を返す
func NewDeploymentRepository(db *gorm.DB) DeploymentRepository {
	return &deploymentRepositoryImpl{db: db} // 実装を生成して返す
}

// Create は deployment レコードを作成する
func (repo *deploymentRepositoryImpl) Create(ctx context.Context, deployment *models.Deployment) error {
	return repo.db.WithContext(ctx).Create(deployment).Error // db を使って作成する
}

// CreateWithTx はトランザクション内で deployment レコードを作成する
func (repo *deploymentRepositoryImpl) CreateWithTx(ctx context.Context, tx *gorm.DB, deployment *models.Deployment) error {
	return tx.WithContext(ctx).Create(deployment).Error // tx を使って作成する
}

// FindByID は deploymentID に対応する deployment を返す
func (repo *deploymentRepositoryImpl) FindByID(ctx context.Context, deploymentID string) (*models.Deployment, error) {
	var deploymentData models.Deployment                                                          // deployment を格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&deploymentData, "id = ?", deploymentID).Error; err != nil { // db から deployment を取得する
		return nil, err // 取得エラーを返す
	}
	return &deploymentData, nil // deployment を返す
}

// FindAllByProjectID は projectID に紐づく deployment 一覧を返す
func (repo *deploymentRepositoryImpl) FindAllByProjectID(ctx context.Context, projectID string) ([]models.Deployment, error) {
	var deploymentList []models.Deployment                                                                              // deployment 一覧を格納するスライスを定義する
	if err := repo.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&deploymentList).Error; err != nil { // db から deployment 一覧を取得する
		return nil, err // 取得エラーを返す
	}
	return deploymentList, nil // deployment 一覧を返す
}

// FindAllRunning は status=running の deployment を全件返す
func (repo *deploymentRepositoryImpl) FindAllRunning(ctx context.Context) ([]models.Deployment, error) {
	var deploymentList []models.Deployment                                                                                           // deployment 一覧を格納するスライスを定義する
	if err := repo.db.WithContext(ctx).Where("status = ?", models.DeploymentStatusRunning).Find(&deploymentList).Error; err != nil { // status=running の deployment を全件取得する
		return nil, err // 取得エラーを返す
	}
	return deploymentList, nil // deployment 一覧を返す
}

// FindByIDForUpdate は SELECT FOR UPDATE で deploymentID に対応する deployment を取得する
func (repo *deploymentRepositoryImpl) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, deploymentID string) (*models.Deployment, error) {
	var deploymentData models.Deployment                                                                                                              // deployment を格納する変数を定義する
	if err := tx.WithContext(ctx).First(&deploymentData, "id = ?", deploymentID).Error; err != nil { // FOR UPDATE ロックを取得しながら取得する
		return nil, err // 取得エラーを返す
	}
	return &deploymentData, nil // deployment を返す
}

// Save は deployment レコードを保存する
func (repo *deploymentRepositoryImpl) Save(ctx context.Context, deployment *models.Deployment) error {
	return repo.db.WithContext(ctx).Save(deployment).Error // db を使って保存する
}

// Updates は deployment レコードを map の値で部分更新する
func (repo *deploymentRepositoryImpl) Updates(ctx context.Context, tx *gorm.DB, deployment *models.Deployment, values map[string]interface{}) error {
	return tx.WithContext(ctx).Model(deployment).Updates(values).Error // tx を使って部分更新する
}

// UpdateAppStatus は deploymentID に対応する deployment の app_status を更新する
func (repo *deploymentRepositoryImpl) UpdateAppStatus(ctx context.Context, deploymentID string, appStatus models.AppStatus) error {
	result := repo.db.WithContext(ctx).Model(&models.Deployment{}).Where("id = ?", deploymentID).Update("app_status", appStatus) // app_status を更新する
	if result.Error != nil {                                                                                                      // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// UpdateK8sStatus は deploymentID に対応する deployment の k8s_status を更新する
func (repo *deploymentRepositoryImpl) UpdateK8sStatus(ctx context.Context, deploymentID string, k8sStatus datatypes.JSON) error {
	result := repo.db.WithContext(ctx).Model(&models.Deployment{}).Where("id = ?", deploymentID).Update("k8s_status", k8sStatus) // k8s_status を更新する
	if result.Error != nil {                                                                                                       // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// UpdateDeleteProgress は deploymentID に対応する deployment の delete_progress を更新する
func (repo *deploymentRepositoryImpl) UpdateDeleteProgress(ctx context.Context, deploymentID string, progress string) error {
	result := repo.db.WithContext(ctx).Model(&models.Deployment{}).Where("id = ?", deploymentID).Update("delete_progress", progress) // delete_progress を更新する
	if result.Error != nil {                                                                                                           // エラーが発生した場合
		return result.Error // エラーを返す
	}
	return nil // 正常終了（RowsAffected=0 は物理削除済みの場合があるため無視する）
}

// UpdatePendingImageURL は deploymentID に対応する deployment の pending_image_url を更新する
func (repo *deploymentRepositoryImpl) UpdatePendingImageURL(ctx context.Context, deploymentID string, imageURL string) error {
	result := repo.db.WithContext(ctx).Model(&models.Deployment{}).Where("id = ?", deploymentID).Update("pending_image_url", imageURL) // pending_image_url を更新する
	if result.Error != nil {                                                                                                             // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// UpdatePendingGithubCommitSHA は deploymentID に対応する deployment の pending_github_commit_sha を更新する
func (repo *deploymentRepositoryImpl) UpdatePendingGithubCommitSHA(ctx context.Context, deploymentID string, commitSHA string) error {
	result := repo.db.WithContext(ctx).Model(&models.Deployment{}).Where("id = ?", deploymentID).Update("pending_github_commit_sha", commitSHA) // pending_github_commit_sha を更新する
	if result.Error != nil {                                                                                                                       // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// UpdatePendingGithubBuildFields はビルド成功時に pending_github_* フィールドをまとめて更新する
func (repo *deploymentRepositoryImpl) UpdatePendingGithubBuildFields(ctx context.Context, deploymentID string, repoURL string, branch string, commitSHA string, directory string) error {
	result := repo.db.WithContext(ctx).Model(&models.Deployment{}).Where("id = ?", deploymentID).Updates(map[string]interface{}{ // pending_github_* フィールドを一括更新する
		"pending_github_repo_url":       repoURL,   // pending_github_repo_url を更新する
		"pending_github_branch":         branch,    // pending_github_branch を更新する
		"pending_github_commit_sha":     commitSHA, // pending_github_commit_sha を更新する
		"pending_github_repo_directory": directory, // pending_github_repo_directory を更新する
	})
	if result.Error != nil { // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// UpdateDeploymentStatus は deploymentID に対応する deployment の status を更新する
func (repo *deploymentRepositoryImpl) UpdateDeploymentStatus(ctx context.Context, deploymentID string, status models.DeploymentStatus) error {
	result := repo.db.WithContext(ctx).Model(&models.Deployment{}).Where("id = ?", deploymentID).Update("status", status) // status を更新する
	if result.Error != nil {                                                                                                // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// UpdateCurrentBuildID は deploymentID に対応する deployment の current_build_id を更新する
func (repo *deploymentRepositoryImpl) UpdateCurrentBuildID(ctx context.Context, deploymentID string, buildID string) error {
	result := repo.db.WithContext(ctx).Model(&models.Deployment{}).Where("id = ?", deploymentID).Update("current_build_id", buildID) // current_build_id を更新する
	if result.Error != nil {                                                                                                           // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// ClearCurrentBuildID は deploymentID に対応する deployment の current_build_id を NULL にクリアする
func (repo *deploymentRepositoryImpl) ClearCurrentBuildID(ctx context.Context, deploymentID string) error {
	result := repo.db.WithContext(ctx).Model(&models.Deployment{}).Where("id = ?", deploymentID).Update("current_build_id", nil) // current_build_id を NULL にする
	if result.Error != nil {                                                                                                      // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// Delete は deploymentID に対応する deployment を削除する
func (repo *deploymentRepositoryImpl) Delete(ctx context.Context, deploymentID string) error {
	result := repo.db.WithContext(ctx).Delete(&models.Deployment{}, "id = ?", deploymentID) // deployment を削除する
	if result.Error != nil {                                                                  // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 削除対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// ServiceRepository は services テーブルへのアクセスを定義するインターフェース
type ServiceRepository interface {
	Create(ctx context.Context, service *models.Service) error                                                   // service を作成する
	CreateWithTx(ctx context.Context, tx *gorm.DB, service *models.Service) error                                // トランザクション内で service を作成する
	FindByDeploymentID(ctx context.Context, deploymentID string) (*models.Service, error)                        // deploymentID に紐づく service を取得する
	FindByServiceID(ctx context.Context, serviceID string) (*models.Service, error)                              // serviceID に紐づく service を取得する
	Update(ctx context.Context, service *models.Service) error                                                   // service を更新する
	UpdateStatus(ctx context.Context, serviceID string, status models.ServiceStatus, k8sStatus datatypes.JSON) error // service の status と k8s_status を更新する
	UpdateClusterIP(ctx context.Context, serviceID string, clusterIP string) error                                   // service の cluster_ip を更新する
	Delete(ctx context.Context, serviceID string) error                                                              // service を削除する
}

// serviceRepositoryImpl は ServiceRepository の GORM 実装
type serviceRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewServiceRepository は ServiceRepository の実装を返す
func NewServiceRepository(db *gorm.DB) ServiceRepository {
	return &serviceRepositoryImpl{db: db} // 実装を生成して返す
}

// Create は service レコードを作成する
func (repo *serviceRepositoryImpl) Create(ctx context.Context, service *models.Service) error {
	return repo.db.WithContext(ctx).Create(service).Error // db を使って作成する
}

// CreateWithTx はトランザクション内で service レコードを作成する
func (repo *serviceRepositoryImpl) CreateWithTx(ctx context.Context, tx *gorm.DB, service *models.Service) error {
	return tx.WithContext(ctx).Create(service).Error // tx を使って作成する
}

// FindByDeploymentID は deploymentID に対応する service を返す
func (repo *serviceRepositoryImpl) FindByDeploymentID(ctx context.Context, deploymentID string) (*models.Service, error) {
	var serviceData models.Service                                                                                          // service を格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&serviceData, "deployment_id = ?", deploymentID).Error; err != nil { // db から service を取得する
		return nil, err // 取得エラーを返す
	}
	return &serviceData, nil // service を返す
}

// FindByServiceID は serviceID に対応する service を返す
func (repo *serviceRepositoryImpl) FindByServiceID(ctx context.Context, serviceID string) (*models.Service, error) {
	var serviceData models.Service                                                                                    // service を格納する変数を定義する
	if err := repo.db.WithContext(ctx).First(&serviceData, "id = ?", serviceID).Error; err != nil { // db から service を取得する
		return nil, err // 取得エラーを返す
	}
	return &serviceData, nil // service を返す
}

// Update は service レコードを保存する
func (repo *serviceRepositoryImpl) Update(ctx context.Context, service *models.Service) error {
	return repo.db.WithContext(ctx).Save(service).Error // db を使って保存する
}

// UpdateStatus は serviceID に対応する service の status と k8s_status を更新する
func (repo *serviceRepositoryImpl) UpdateStatus(ctx context.Context, serviceID string, status models.ServiceStatus, k8sStatus datatypes.JSON) error {
	result := repo.db.WithContext(ctx).Model(&models.Service{}).Where("id = ?", serviceID).Updates(map[string]interface{}{ // status と k8s_status を更新する
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

// UpdateClusterIP は serviceID に対応する service の cluster_ip を更新する
func (repo *serviceRepositoryImpl) UpdateClusterIP(ctx context.Context, serviceID string, clusterIP string) error {
	result := repo.db.WithContext(ctx).Model(&models.Service{}).Where("id = ?", serviceID).Update("cluster_ip", clusterIP) // cluster_ip を更新する
	if result.Error != nil {                                                                                                 // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 更新対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}

// Delete は serviceID に対応する service レコードを削除する
func (repo *serviceRepositoryImpl) Delete(ctx context.Context, serviceID string) error {
	result := repo.db.WithContext(ctx).Delete(&models.Service{}, "id = ?", serviceID) // service を削除する
	if result.Error != nil {                                                           // エラーが発生した場合
		return result.Error // エラーを返す
	}
	if result.RowsAffected == 0 { // 削除対象が存在しない場合
		return gorm.ErrRecordNotFound // レコードなしエラーを返す
	}
	return nil // 正常終了
}
