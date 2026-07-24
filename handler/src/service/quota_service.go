package service

import (
	"handler/models"
	"handler/repository"
	"context"
	"fmt"
)

// QuotaService は quota 取得・更新のビジネスロジックを定義するインターフェース
type QuotaService interface {
	GetQuota(ctx context.Context, userID string) (*QuotaResponse, error)                           // quota と現在使用量を取得する
	UpdateQuota(ctx context.Context, userID string, req UpdateQuotaRequest) (*QuotaResponse, error) // quota を部分更新する
}

// QuotaResponse は GET /users/:user_id/quota のレスポンス構造体
type QuotaResponse struct {
	UserID                   string         `json:"user_id"`                     // ユーザーID
	PlanID                   string         `json:"plan_id"`                     // プランID
	MaxProjects              int            `json:"max_projects"`                // プロジェクト上限数
	MaxDeployments           int            `json:"max_deployments"`             // デプロイメント上限数
	MaxReplicasPerDeployment int            `json:"max_replicas_per_deployment"` // デプロイメントあたりのレプリカ上限
	MaxVolumes               int            `json:"max_volumes"`                 // ボリューム数上限
	MaxVolumeSizeMB          int            `json:"max_volume_size_mb"`          // 1ボリュームあたりの最大サイズ（MB）
	MaxTotalVolumeMB         int            `json:"max_total_volume_mb"`         // ボリューム総容量上限（MB）
	InstanceLimits           map[string]int `json:"instance_limits"`             // スペック別デプロイメント上限
	CurrentInstances         map[string]int `json:"current_instances"`           // スペック別現在デプロイメント数
	CurrentProjects          int            `json:"current_projects"`            // 現在のプロジェクト数
	CurrentDeployments       int            `json:"current_deployments"`         // 現在のデプロイメント数
	CurrentVolumes           int            `json:"current_volumes"`             // 現在のボリューム数
	CurrentTotalVolumeMB     int            `json:"current_total_volume_mb"`     // 現在のボリューム使用量 MB
}

// UpdateQuotaRequest は PUT /users/:user_id/quota のリクエスト構造体
type UpdateQuotaRequest struct {
	PlanID                           *string `json:"plan_id"`                            // プラン変更（nil の場合は更新しない）
	OverrideMaxProjects              *int    `json:"override_max_projects"`              // プロジェクト上限の個別上書き
	OverrideMaxDeployments           *int    `json:"override_max_deployments"`           // デプロイメント上限の個別上書き
	OverrideMaxReplicasPerDeployment *int    `json:"override_max_replicas_per_deployment"` // レプリカ上限の個別上書き
	OverrideMaxVolumes               *int    `json:"override_max_volumes"`               // ボリューム数上限の個別上書き
	OverrideMaxVolumeSizeMB          *int    `json:"override_max_volume_size_mb"`        // 1ボリューム最大サイズの個別上書き（MB）
	OverrideMaxTotalVolumeMB         *int    `json:"override_max_total_volume_mb"`       // ボリューム総容量上限の個別上書き（MB）
}

// quotaServiceImpl は QuotaService の実装
type quotaServiceImpl struct {
	userQuotaRepository repository.UserQuotaRepository // quota リポジトリのインターフェース
	defaultPlanID       string                         // 新規ユーザー作成時に割り当てるデフォルトプラン ID
}

// NewQuotaService は QuotaService の実装を返す
func NewQuotaService(userQuotaRepository repository.UserQuotaRepository, defaultPlanID string) QuotaService {
	return &quotaServiceImpl{
		userQuotaRepository: userQuotaRepository, // 依存を注入する
		defaultPlanID:       defaultPlanID,        // デフォルトプラン ID を注入する
	}
}

// GetQuota は userID に対応する実効 quota と現在使用量を返す
func (svc *quotaServiceImpl) GetQuota(ctx context.Context, userID string) (*QuotaResponse, error) {
	quotaData, err := svc.userQuotaRepository.GetOrCreate(ctx, userID, svc.defaultPlanID) // quota を取得または作成する
	if err != nil {
		return nil, err // DB エラーを返す
	}

	resolvedQuota, err := svc.userQuotaRepository.GetResolvedQuota(ctx, userID) // 実効上限値を解決する
	if err != nil {
		return nil, err // 解決エラーを返す
	}

	currentProjects, err := svc.userQuotaRepository.CountProjects(ctx, userID) // 現在のプロジェクト数を集計する
	if err != nil {
		return nil, err // 集計エラーを返す
	}

	currentDeployments, err := svc.userQuotaRepository.CountDeployments(ctx, userID) // 現在のデプロイメント数を集計する
	if err != nil {
		return nil, err // 集計エラーを返す
	}

	currentVolumes, err := svc.userQuotaRepository.CountVolumes(ctx, userID) // 現在のボリューム数を集計する
	if err != nil {
		return nil, err // 集計エラーを返す
	}

	currentTotalVolumeMB, err := svc.userQuotaRepository.SumVolumeMB(ctx, userID) // 現在のボリューム使用量を集計する
	if err != nil {
		return nil, err // 集計エラーを返す
	}

	currentInstances := make(map[string]int) // スペック別現在デプロイメント数マップを初期化する
	for instanceSize := range resolvedQuota.InstanceLimits {
		count, countErr := svc.userQuotaRepository.CountInstancesBySize(ctx, userID, instanceSize) // サイズごとの現在数を集計する
		if countErr != nil {
			return nil, countErr // 集計エラーを返す
		}
		currentInstances[instanceSize] = count // マップに格納する
	}

	return &QuotaResponse{
		UserID:                   userID,
		PlanID:                   quotaData.PlanID,
		MaxProjects:              resolvedQuota.MaxProjects,
		MaxDeployments:           resolvedQuota.MaxDeployments,
		MaxReplicasPerDeployment: resolvedQuota.MaxReplicasPerDeployment,
		MaxVolumes:               resolvedQuota.MaxVolumes,
		MaxVolumeSizeMB:          resolvedQuota.MaxVolumeSizeMB,
		MaxTotalVolumeMB:         resolvedQuota.MaxTotalVolumeMB,
		InstanceLimits:           resolvedQuota.InstanceLimits,
		CurrentInstances:         currentInstances,
		CurrentProjects:          currentProjects,
		CurrentDeployments:       currentDeployments,
		CurrentVolumes:           currentVolumes,
		CurrentTotalVolumeMB:     currentTotalVolumeMB,
	}, nil // レスポンスを組み立てて返す
}

// UpdateQuota は userID の quota を部分更新して更新後のデータを返す
func (svc *quotaServiceImpl) UpdateQuota(ctx context.Context, userID string, req UpdateQuotaRequest) (*QuotaResponse, error) {
	updates := buildUpdateMap(req) // リクエストから更新マップを構築する

	if _, err := svc.userQuotaRepository.Update(ctx, userID, updates); err != nil { // quota を部分更新する
		return nil, err // 更新エラーを返す
	}

	return svc.GetQuota(ctx, userID) // 更新後の実効値を取得して返す
}

// buildUpdateMap はリクエストから nil でないフィールドだけを更新マップに変換する
func buildUpdateMap(req UpdateQuotaRequest) map[string]interface{} {
	updates := map[string]interface{}{} // 更新対象のフィールドマップ
	if req.PlanID != nil {
		updates["plan_id"] = *req.PlanID // プラン ID を更新対象に追加する
	}
	if req.OverrideMaxProjects != nil {
		updates["override_max_projects"] = *req.OverrideMaxProjects // プロジェクト上限の個別上書きを更新対象に追加する
	}
	if req.OverrideMaxDeployments != nil {
		updates["override_max_deployments"] = *req.OverrideMaxDeployments // デプロイメント上限の個別上書きを更新対象に追加する
	}
	if req.OverrideMaxReplicasPerDeployment != nil {
		updates["override_max_replicas_per_deployment"] = *req.OverrideMaxReplicasPerDeployment // レプリカ上限の個別上書きを更新対象に追加する
	}
	if req.OverrideMaxVolumes != nil {
		updates["override_max_volumes"] = *req.OverrideMaxVolumes // ボリューム数上限の個別上書きを更新対象に追加する
	}
	if req.OverrideMaxVolumeSizeMB != nil {
		updates["override_max_volume_size_mb"] = *req.OverrideMaxVolumeSizeMB // 1ボリューム最大サイズの個別上書きを更新対象に追加する
	}
	if req.OverrideMaxTotalVolumeMB != nil {
		updates["override_max_total_volume_mb"] = *req.OverrideMaxTotalVolumeMB // ボリューム総容量上限の個別上書きを更新対象に追加する
	}
	return updates // 更新マップを返す
}

// CheckProjectQuota はユーザーのプロジェクト数が上限に達していないか確認する
func CheckProjectQuota(ctx context.Context, userQuotaRepo repository.UserQuotaRepository, userID string) error {
	resolvedQuota, err := userQuotaRepo.GetResolvedQuota(ctx, userID) // 実効上限値を取得する
	if err != nil {
		return fmt.Errorf("quota 取得エラー: %w", err) // 取得エラーを返す
	}
	currentProjects, err := userQuotaRepo.CountProjects(ctx, userID) // 現在のプロジェクト数を集計する
	if err != nil {
		return fmt.Errorf("プロジェクト数集計エラー: %w", err) // 集計エラーを返す
	}
	if currentProjects >= resolvedQuota.MaxProjects { // 上限に達している場合はエラーを返す
		return &QuotaExceededError{Resource: "projects", Current: currentProjects, Limit: resolvedQuota.MaxProjects} // プロジェクト数超過エラーを返す
	}
	return nil // チェック通過を返す
}

// CheckDeploymentQuota はユーザーのデプロイメント数が上限に達していないか確認する
func CheckDeploymentQuota(ctx context.Context, userQuotaRepo repository.UserQuotaRepository, userID string) error {
	resolvedQuota, err := userQuotaRepo.GetResolvedQuota(ctx, userID) // 実効上限値を取得する
	if err != nil {
		return fmt.Errorf("quota 取得エラー: %w", err) // 取得エラーを返す
	}
	currentDeployments, err := userQuotaRepo.CountDeployments(ctx, userID) // 現在のデプロイメント数を集計する
	if err != nil {
		return fmt.Errorf("デプロイメント数集計エラー: %w", err) // 集計エラーを返す
	}
	if currentDeployments >= resolvedQuota.MaxDeployments { // 上限に達している場合はエラーを返す
		return &QuotaExceededError{Resource: "deployments", Current: currentDeployments, Limit: resolvedQuota.MaxDeployments} // デプロイメント数超過エラーを返す
	}
	return nil // チェック通過を返す
}

// CheckReplicasQuota はレプリカ数が上限を超えていないか確認する
func CheckReplicasQuota(ctx context.Context, userQuotaRepo repository.UserQuotaRepository, userID string, replicas int32) error {
	resolvedQuota, err := userQuotaRepo.GetResolvedQuota(ctx, userID) // 実効上限値を取得する
	if err != nil {
		return fmt.Errorf("quota 取得エラー: %w", err) // 取得エラーを返す
	}
	if int(replicas) > resolvedQuota.MaxReplicasPerDeployment { // 上限を超えている場合はエラーを返す
		return &QuotaExceededError{Resource: "replicas", Current: int(replicas), Limit: resolvedQuota.MaxReplicasPerDeployment} // レプリカ数超過エラーを返す
	}
	return nil // チェック通過を返す
}

// CheckVolumeCountQuota はユーザーのボリューム数が上限に達していないか確認する
func CheckVolumeCountQuota(ctx context.Context, userQuotaRepo repository.UserQuotaRepository, userID string) error {
	resolvedQuota, err := userQuotaRepo.GetResolvedQuota(ctx, userID) // 実効上限値を取得する
	if err != nil {
		return fmt.Errorf("quota 取得エラー: %w", err) // 取得エラーを返す
	}
	currentVolumes, err := userQuotaRepo.CountVolumes(ctx, userID) // 現在のボリューム数を集計する
	if err != nil {
		return fmt.Errorf("ボリューム数集計エラー: %w", err) // 集計エラーを返す
	}
	if currentVolumes >= resolvedQuota.MaxVolumes { // 上限に達している場合はエラーを返す
		return &QuotaExceededError{Resource: "volumes", Current: currentVolumes, Limit: resolvedQuota.MaxVolumes} // ボリューム数超過エラーを返す
	}
	return nil // チェック通過を返す
}

// CheckVolumeSizeQuota は作成するボリュームのサイズが 1 ボリューム最大サイズ以内か確認する
func CheckVolumeSizeQuota(ctx context.Context, userQuotaRepo repository.UserQuotaRepository, userID string, sizeMB int) error {
	resolvedQuota, err := userQuotaRepo.GetResolvedQuota(ctx, userID) // 実効上限値を取得する
	if err != nil {
		return fmt.Errorf("quota 取得エラー: %w", err) // 取得エラーを返す
	}
	if sizeMB > resolvedQuota.MaxVolumeSizeMB { // 1ボリューム最大サイズを超えている場合はエラーを返す
		return &QuotaExceededError{Resource: "volume_size_mb", Current: sizeMB, Limit: resolvedQuota.MaxVolumeSizeMB} // ボリュームサイズ超過エラーを返す
	}
	return nil // チェック通過を返す
}

// CheckTotalVolumeQuota はユーザーのボリューム総容量が上限に達していないか確認する
func CheckTotalVolumeQuota(ctx context.Context, userQuotaRepo repository.UserQuotaRepository, userID string, additionalMB int) error {
	resolvedQuota, err := userQuotaRepo.GetResolvedQuota(ctx, userID) // 実効上限値を取得する
	if err != nil {
		return fmt.Errorf("quota 取得エラー: %w", err) // 取得エラーを返す
	}
	currentVolumeMB, err := userQuotaRepo.SumVolumeMB(ctx, userID) // 現在のボリューム使用量を集計する
	if err != nil {
		return fmt.Errorf("ボリューム使用量集計エラー: %w", err) // 集計エラーを返す
	}
	if currentVolumeMB+additionalMB > resolvedQuota.MaxTotalVolumeMB { // 追加後に上限を超える場合はエラーを返す
		return &QuotaExceededError{Resource: "total_volume_mb", Current: currentVolumeMB + additionalMB, Limit: resolvedQuota.MaxTotalVolumeMB} // ボリューム総容量超過エラーを返す
	}
	return nil // チェック通過を返す
}

// CheckInstanceQuota は指定インスタンスサイズのデプロイメント数が上限に達していないか確認する
func CheckInstanceQuota(ctx context.Context, userQuotaRepo repository.UserQuotaRepository, userID string, instanceSize string) error {
	resolvedQuota, err := userQuotaRepo.GetResolvedQuota(ctx, userID) // 実効上限値を取得する
	if err != nil {
		return fmt.Errorf("quota 取得エラー: %w", err) // 取得エラーを返す
	}
	maxCount, exists := resolvedQuota.InstanceLimits[instanceSize] // スペック別上限を取得する
	if !exists {
		return nil // 上限が設定されていないサイズはチェックしない
	}
	currentCount, err := userQuotaRepo.CountInstancesBySize(ctx, userID, instanceSize) // 現在の使用数を集計する
	if err != nil {
		return fmt.Errorf("インスタンス数集計エラー: %w", err) // 集計エラーを返す
	}
	if currentCount >= maxCount { // 上限に達している場合はエラーを返す
		return &QuotaExceededError{Resource: "instance:" + instanceSize, Current: currentCount, Limit: maxCount} // インスタンス数超過エラーを返す
	}
	return nil // チェック通過を返す
}

// resolvedQuotaFromModel は models.UserQuota からプランを JOIN せずに ResolvedQuota を組み立てるヘルパー（テスト用途）
func resolvedQuotaFromModel(quotaData *models.UserQuota) *models.ResolvedQuota {
	planData := quotaData.Plan // プランの値をベースにする
	resolved := &models.ResolvedQuota{
		MaxProjects:              planData.MaxProjects,              // プランのプロジェクト上限
		MaxDeployments:           planData.MaxDeployments,           // プランのデプロイメント上限
		MaxReplicasPerDeployment: planData.MaxReplicasPerDeployment, // プランのレプリカ上限
		MaxVolumes:               planData.MaxVolumes,               // プランのボリューム数上限
		MaxVolumeSizeMB:          planData.MaxVolumeSizeMB,          // プランの 1 ボリューム最大サイズ
		MaxTotalVolumeMB:         planData.MaxTotalVolumeMB,         // プランのボリューム総容量上限
		InstanceLimits:           make(map[string]int),              // スペック別上限マップを初期化する
	}
	// 個別上書きを適用する
	if quotaData.OverrideMaxProjects != nil {
		resolved.MaxProjects = *quotaData.OverrideMaxProjects
	}
	if quotaData.OverrideMaxDeployments != nil {
		resolved.MaxDeployments = *quotaData.OverrideMaxDeployments
	}
	if quotaData.OverrideMaxReplicasPerDeployment != nil {
		resolved.MaxReplicasPerDeployment = *quotaData.OverrideMaxReplicasPerDeployment
	}
	if quotaData.OverrideMaxVolumes != nil {
		resolved.MaxVolumes = *quotaData.OverrideMaxVolumes
	}
	if quotaData.OverrideMaxVolumeSizeMB != nil {
		resolved.MaxVolumeSizeMB = *quotaData.OverrideMaxVolumeSizeMB
	}
	if quotaData.OverrideMaxTotalVolumeMB != nil {
		resolved.MaxTotalVolumeMB = *quotaData.OverrideMaxTotalVolumeMB
	}
	for _, limitData := range planData.InstanceLimits {
		resolved.InstanceLimits[limitData.InstanceSize] = limitData.MaxCount // スペック別上限をマップに格納する
	}
	return resolved
}
