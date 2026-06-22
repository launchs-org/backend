package repository

import (
	"app/models"
	"context"

	"gorm.io/gorm"
)

// UserQuotaRepository は user_quotas テーブルへのアクセスを定義するインターフェース
type UserQuotaRepository interface {
	GetOrCreate(ctx context.Context, userID string, defaultPlanID string) (*models.UserQuota, error)                    // quota を取得し存在しなければ作成する
	GetResolvedQuota(ctx context.Context, userID string) (*models.ResolvedQuota, error)                                // COALESCE で実効上限値を解決して返す
	Update(ctx context.Context, userID string, updates map[string]interface{}) (*models.UserQuota, error)              // quota を部分更新する
	CountProjects(ctx context.Context, userID string) (int, error)                                                     // ユーザーのプロジェクト数を集計する
	CountDeployments(ctx context.Context, userID string) (int, error)                                                  // ユーザーのデプロイメント数を集計する
	CountVolumes(ctx context.Context, userID string) (int, error)                                                      // ユーザーのボリューム数を集計する
	CountInstancesBySize(ctx context.Context, userID string, instanceSize string) (int, error)                         // インスタンスサイズ別のデプロイメント数を集計する
	SumVolumeMB(ctx context.Context, userID string) (int, error)                                                       // ユーザーの合計ボリューム容量を集計する
}

// userQuotaRepositoryImpl は UserQuotaRepository の GORM 実装
type userQuotaRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewUserQuotaRepository は UserQuotaRepository の実装を返す
func NewUserQuotaRepository(db *gorm.DB) UserQuotaRepository {
	return &userQuotaRepositoryImpl{db: db} // 実装を生成して返す
}

// GetOrCreate は userID に対応する quota を返す。存在しない場合は defaultPlanID で作成する
func (repo *userQuotaRepositoryImpl) GetOrCreate(ctx context.Context, userID string, defaultPlanID string) (*models.UserQuota, error) {
	quotaData := &models.UserQuota{
		UserID: userID,        // upsert のキーとして userID を設定する
		PlanID: defaultPlanID, // 新規作成時のデフォルトプランを設定する
	}
	result := repo.db.WithContext(ctx).
		Where(models.UserQuota{UserID: userID}).
		Attrs(models.UserQuota{PlanID: defaultPlanID}). // レコードが存在しない場合のみデフォルトプランを適用する
		FirstOrCreate(quotaData)                        // 存在すれば取得、なければ作成する
	if result.Error != nil {
		return nil, result.Error // DB エラーを返す
	}
	return quotaData, nil // quota データを返す
}

// GetResolvedQuota はプランの上限値とユーザー個別上書きを COALESCE で統合した実効上限値を返す
func (repo *userQuotaRepositoryImpl) GetResolvedQuota(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
	type resolvedRow struct {
		MaxProjects              int
		MaxDeployments           int
		MaxReplicasPerDeployment int
		MaxVolumes               int
		MaxVolumeSizeMB          int
		MaxTotalVolumeMB         int
	}

	var row resolvedRow
	result := repo.db.WithContext(ctx).
		Table("user_quotas").
		Joins("JOIN plans ON plans.id = user_quotas.plan_id").
		Where("user_quotas.user_id = ?", userID).
		Select(`
			COALESCE(user_quotas.override_max_projects,               plans.max_projects)                AS max_projects,
			COALESCE(user_quotas.override_max_deployments,            plans.max_deployments)             AS max_deployments,
			COALESCE(user_quotas.override_max_replicas_per_deployment, plans.max_replicas_per_deployment) AS max_replicas_per_deployment,
			COALESCE(user_quotas.override_max_volumes,                plans.max_volumes)                 AS max_volumes,
			COALESCE(user_quotas.override_max_volume_size_mb,         plans.max_volume_size_mb)          AS max_volume_size_mb,
			COALESCE(user_quotas.override_max_total_volume_mb,        plans.max_total_volume_mb)         AS max_total_volume_mb
		`).
		Scan(&row) // COALESCE で実効値を解決する
	if result.Error != nil {
		return nil, result.Error // DB エラーを返す
	}

	// スペック別デプロイメント上限を取得する
	type instanceLimitRow struct {
		InstanceSize string
		MaxCount     int
	}
	var instanceLimitList []instanceLimitRow
	instanceResult := repo.db.WithContext(ctx).
		Table("user_quotas").
		Joins("JOIN plans ON plans.id = user_quotas.plan_id").
		Joins("JOIN plan_instance_limits ON plan_instance_limits.plan_id = plans.id").
		Where("user_quotas.user_id = ?", userID).
		Select("plan_instance_limits.instance_size, plan_instance_limits.max_count").
		Scan(&instanceLimitList) // インスタンスサイズ別の上限を取得する
	if instanceResult.Error != nil {
		return nil, instanceResult.Error // DB エラーを返す
	}

	instanceLimitMap := make(map[string]int) // インスタンスサイズ -> 上限数のマップを生成する
	for _, limitRow := range instanceLimitList {
		instanceLimitMap[limitRow.InstanceSize] = limitRow.MaxCount // マップに格納する
	}

	return &models.ResolvedQuota{
		MaxProjects:              row.MaxProjects,              // 実効プロジェクト上限
		MaxDeployments:           row.MaxDeployments,           // 実効デプロイメント上限
		MaxReplicasPerDeployment: row.MaxReplicasPerDeployment, // 実効レプリカ上限
		MaxVolumes:               row.MaxVolumes,               // 実効ボリューム数上限
		MaxVolumeSizeMB:          row.MaxVolumeSizeMB,          // 実効 1 ボリューム最大サイズ
		MaxTotalVolumeMB:         row.MaxTotalVolumeMB,         // 実効ボリューム総容量上限
		InstanceLimits:           instanceLimitMap,             // 実効スペック別上限マップ
	}, nil
}

// Update は userID の quota を部分更新して更新後のレコードを返す
func (repo *userQuotaRepositoryImpl) Update(ctx context.Context, userID string, updates map[string]interface{}) (*models.UserQuota, error) {
	result := repo.db.WithContext(ctx).
		Model(&models.UserQuota{}).
		Where("user_id = ?", userID).
		Updates(updates) // 指定フィールドのみ更新する
	if result.Error != nil {
		return nil, result.Error // DB エラーを返す
	}

	var quotaData models.UserQuota
	if err := repo.db.WithContext(ctx).Where("user_id = ?", userID).First(&quotaData).Error; err != nil {
		return nil, err // 取得エラーを返す
	}
	return &quotaData, nil // 更新後の quota データを返す
}

// CountProjects はユーザーが所有するプロジェクト数を返す
func (repo *userQuotaRepositoryImpl) CountProjects(ctx context.Context, userID string) (int, error) {
	var projectCount int64
	result := repo.db.WithContext(ctx).
		Model(&models.Project{}).
		Where("user_id = ? AND status != ?", userID, models.ProjectStatusDeleting).
		Count(&projectCount) // 削除中以外のプロジェクト数を集計する
	if result.Error != nil {
		return 0, result.Error // DB エラーを返す
	}
	return int(projectCount), nil // プロジェクト数を返す
}

// CountDeployments はユーザーが所有するデプロイメント数を返す
func (repo *userQuotaRepositoryImpl) CountDeployments(ctx context.Context, userID string) (int, error) {
	var deploymentCount int64
	result := repo.db.WithContext(ctx).
		Model(&models.Deployment{}).
		Joins("JOIN projects ON projects.id = deployments.project_id").
		Where("projects.user_id = ? AND deployments.status != ?", userID, models.DeploymentStatusDeleting).
		Count(&deploymentCount) // 削除中以外のデプロイメント数を集計する
	if result.Error != nil {
		return 0, result.Error // DB エラーを返す
	}
	return int(deploymentCount), nil // デプロイメント数を返す
}

// CountVolumes はユーザーが所有するボリューム数を返す
func (repo *userQuotaRepositoryImpl) CountVolumes(ctx context.Context, userID string) (int, error) {
	var volumeCount int64
	result := repo.db.WithContext(ctx).
		Model(&models.Volume{}).
		Joins("JOIN projects ON projects.id = volumes.project_id").
		Where("projects.user_id = ? AND volumes.status != ?", userID, models.VolumeStatusDeleting).
		Count(&volumeCount) // 削除中以外のボリューム数を集計する
	if result.Error != nil {
		return 0, result.Error // DB エラーを返す
	}
	return int(volumeCount), nil // ボリューム数を返す
}

// CountInstancesBySize はユーザーが使用中の特定インスタンスサイズのデプロイメント数を返す
func (repo *userQuotaRepositoryImpl) CountInstancesBySize(ctx context.Context, userID string, instanceSize string) (int, error) {
	var deploymentCount int64
	result := repo.db.WithContext(ctx).
		Model(&models.Deployment{}).
		Joins("JOIN projects ON projects.id = deployments.project_id").
		Where("projects.user_id = ? AND deployments.instance_size = ? AND deployments.status != ?", userID, instanceSize, models.DeploymentStatusDeleting).
		Count(&deploymentCount) // 指定サイズの削除中以外のデプロイメント数を集計する
	if result.Error != nil {
		return 0, result.Error // DB エラーを返す
	}
	return int(deploymentCount), nil // デプロイメント数を返す
}

// SumVolumeMB はユーザーが使用中の合計ボリューム容量（MB）を返す
func (repo *userQuotaRepositoryImpl) SumVolumeMB(ctx context.Context, userID string) (int, error) {
	var totalMB *int64
	result := repo.db.WithContext(ctx).
		Model(&models.Volume{}).
		Joins("JOIN projects ON projects.id = volumes.project_id").
		Where("projects.user_id = ?", userID).
		Select("COALESCE(SUM(volumes.size_mb), 0)").
		Scan(&totalMB) // ボリューム容量の合計を集計する
	if result.Error != nil {
		return 0, result.Error // DB エラーを返す
	}
	if totalMB == nil {
		return 0, nil // ボリュームが存在しない場合は 0 を返す
	}
	return int(*totalMB), nil // 合計容量を返す
}
