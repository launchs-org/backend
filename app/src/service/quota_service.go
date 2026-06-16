package service

import (
	"app/models"
	"app/repository"
	"context"
	"errors"
	"fmt"
)

// ErrProjectQuotaExceeded はプロジェクト数が上限に達している場合のエラー
var ErrProjectQuotaExceeded = errors.New("project quota exceeded")

// ErrDeploymentQuotaExceeded はデプロイメント数が上限に達している場合のエラー
var ErrDeploymentQuotaExceeded = errors.New("deployment quota exceeded")

// ErrReplicasQuotaExceeded はレプリカ数が上限を超えている場合のエラー
var ErrReplicasQuotaExceeded = errors.New("replicas quota exceeded")

// ErrVolumeQuotaExceeded はボリューム容量が上限に達している場合のエラー
var ErrVolumeQuotaExceeded = errors.New("volume quota exceeded")

// QuotaService は quota 取得・更新のビジネスロジックを定義するインターフェース
type QuotaService interface {
	GetQuota(ctx context.Context, userID string) (*QuotaResponse, error)                        // quota と現在使用量を取得する
	UpdateQuota(ctx context.Context, userID string, req UpdateQuotaRequest) (*QuotaResponse, error) // quota を部分更新する
}

// QuotaResponse は GET /users/:user_id/quota のレスポンス構造体
type QuotaResponse struct {
	UserID                   string `json:"user_id"`                     // ユーザーID
	MaxProjects              int    `json:"max_projects"`                // プロジェクト上限数
	MaxDeployments           int    `json:"max_deployments"`             // デプロイメント上限数
	MaxReplicasPerDeployment int    `json:"max_replicas_per_deployment"` // デプロイメントあたりのレプリカ上限
	MaxVolumeMB              int    `json:"max_volume_mb"`               // ボリューム上限 MB
	CurrentProjects          int    `json:"current_projects"`            // 現在のプロジェクト数
	CurrentDeployments       int    `json:"current_deployments"`         // 現在のデプロイメント数
	CurrentVolumeMB          int    `json:"current_volume_mb"`           // 現在のボリューム使用量 MB
}

// UpdateQuotaRequest は PUT /users/:user_id/quota のリクエスト構造体
type UpdateQuotaRequest struct {
	MaxProjects              *int `json:"max_projects"`                // nil の場合は更新しない
	MaxDeployments           *int `json:"max_deployments"`             // nil の場合は更新しない
	MaxReplicasPerDeployment *int `json:"max_replicas_per_deployment"` // nil の場合は更新しない
	MaxVolumeMB              *int `json:"max_volume_mb"`               // nil の場合は更新しない
}

// quotaServiceImpl は QuotaService の実装
type quotaServiceImpl struct {
	userQuotaRepository repository.UserQuotaRepository // quota リポジトリのインターフェース
}

// NewQuotaService は QuotaService の実装を返す
func NewQuotaService(userQuotaRepository repository.UserQuotaRepository) QuotaService {
	return &quotaServiceImpl{
		userQuotaRepository: userQuotaRepository, // 依存を注入する
	}
}

// GetQuota は userID に対応する quota と現在使用量を返す
func (svc *quotaServiceImpl) GetQuota(ctx context.Context, userID string) (*QuotaResponse, error) {
	quotaData, err := svc.userQuotaRepository.GetOrCreate(ctx, userID) // quota を取得または作成する
	if err != nil {
		return nil, err // DB エラーを返す
	}

	currentProjects, err := svc.userQuotaRepository.CountProjects(ctx, userID) // 現在のプロジェクト数を集計する
	if err != nil {
		return nil, err // 集計エラーを返す
	}

	currentDeployments, err := svc.userQuotaRepository.CountDeployments(ctx, userID) // 現在のデプロイメント数を集計する
	if err != nil {
		return nil, err // 集計エラーを返す
	}

	currentVolumeMB, err := svc.userQuotaRepository.SumVolumeMB(ctx, userID) // 現在のボリューム使用量を集計する
	if err != nil {
		return nil, err // 集計エラーを返す
	}

	return buildQuotaResponse(quotaData, currentProjects, currentDeployments, currentVolumeMB), nil // レスポンスを組み立てて返す
}

// UpdateQuota は userID の quota を部分更新して更新後のデータを返す
func (svc *quotaServiceImpl) UpdateQuota(ctx context.Context, userID string, req UpdateQuotaRequest) (*QuotaResponse, error) {
	updates := buildUpdateMap(req) // リクエストから更新マップを構築する

	quotaData, err := svc.userQuotaRepository.Update(ctx, userID, updates) // quota を部分更新する
	if err != nil {
		return nil, err // 更新エラーを返す
	}

	currentProjects, err := svc.userQuotaRepository.CountProjects(ctx, userID) // 更新後の現在プロジェクト数を集計する
	if err != nil {
		return nil, err // 集計エラーを返す
	}

	currentDeployments, err := svc.userQuotaRepository.CountDeployments(ctx, userID) // 更新後の現在デプロイメント数を集計する
	if err != nil {
		return nil, err // 集計エラーを返す
	}

	currentVolumeMB, err := svc.userQuotaRepository.SumVolumeMB(ctx, userID) // 更新後のボリューム使用量を集計する
	if err != nil {
		return nil, err // 集計エラーを返す
	}

	return buildQuotaResponse(quotaData, currentProjects, currentDeployments, currentVolumeMB), nil // レスポンスを組み立てて返す
}

// buildQuotaResponse は UserQuota モデルと集計値から QuotaResponse を組み立てる
func buildQuotaResponse(quotaData *models.UserQuota, currentProjects, currentDeployments, currentVolumeMB int) *QuotaResponse {
	return &QuotaResponse{
		UserID:                   quotaData.UserID,                   // ユーザーID
		MaxProjects:              quotaData.MaxProjects,              // プロジェクト上限
		MaxDeployments:           quotaData.MaxDeployments,           // デプロイメント上限
		MaxReplicasPerDeployment: quotaData.MaxReplicasPerDeployment, // レプリカ上限
		MaxVolumeMB:              quotaData.MaxVolumeMB,              // ボリューム上限
		CurrentProjects:          currentProjects,                    // 現在のプロジェクト数
		CurrentDeployments:       currentDeployments,                 // 現在のデプロイメント数
		CurrentVolumeMB:          currentVolumeMB,                    // 現在のボリューム使用量
	}
}

// buildUpdateMap はリクエストから nil でないフィールドだけを更新マップに変換する
func buildUpdateMap(req UpdateQuotaRequest) map[string]interface{} {
	updates := map[string]interface{}{} // 更新対象のフィールドマップ
	if req.MaxProjects != nil {
		updates["max_projects"] = *req.MaxProjects // プロジェクト上限を更新対象に追加する
	}
	if req.MaxDeployments != nil {
		updates["max_deployments"] = *req.MaxDeployments // デプロイメント上限を更新対象に追加する
	}
	if req.MaxReplicasPerDeployment != nil {
		updates["max_replicas_per_deployment"] = *req.MaxReplicasPerDeployment // レプリカ上限を更新対象に追加する
	}
	if req.MaxVolumeMB != nil {
		updates["max_volume_mb"] = *req.MaxVolumeMB // ボリューム上限を更新対象に追加する
	}
	return updates // 更新マップを返す
}

// CheckProjectQuota はユーザーのプロジェクト数が上限に達していないか確認する
func CheckProjectQuota(ctx context.Context, userQuotaRepo repository.UserQuotaRepository, userID string) error {
	quotaData, err := userQuotaRepo.GetOrCreate(ctx, userID) // quota を取得する
	if err != nil {
		return fmt.Errorf("quota 取得エラー: %w", err) // 取得エラーを返す
	}
	currentProjects, err := userQuotaRepo.CountProjects(ctx, userID) // 現在のプロジェクト数を集計する
	if err != nil {
		return fmt.Errorf("プロジェクト数集計エラー: %w", err) // 集計エラーを返す
	}
	if currentProjects >= quotaData.MaxProjects { // 上限に達している場合はエラーを返す
		return ErrProjectQuotaExceeded // プロジェクト数超過エラーを返す
	}
	return nil // チェック通過を返す
}

// CheckDeploymentQuota はユーザーのデプロイメント数が上限に達していないか確認する
func CheckDeploymentQuota(ctx context.Context, userQuotaRepo repository.UserQuotaRepository, userID string) error {
	quotaData, err := userQuotaRepo.GetOrCreate(ctx, userID) // quota を取得する
	if err != nil {
		return fmt.Errorf("quota 取得エラー: %w", err) // 取得エラーを返す
	}
	currentDeployments, err := userQuotaRepo.CountDeployments(ctx, userID) // 現在のデプロイメント数を集計する
	if err != nil {
		return fmt.Errorf("デプロイメント数集計エラー: %w", err) // 集計エラーを返す
	}
	if currentDeployments >= quotaData.MaxDeployments { // 上限に達している場合はエラーを返す
		return ErrDeploymentQuotaExceeded // デプロイメント数超過エラーを返す
	}
	return nil // チェック通過を返す
}

// CheckReplicasQuota はレプリカ数が上限を超えていないか確認する
func CheckReplicasQuota(ctx context.Context, userQuotaRepo repository.UserQuotaRepository, userID string, replicas int32) error {
	quotaData, err := userQuotaRepo.GetOrCreate(ctx, userID) // quota を取得する
	if err != nil {
		return fmt.Errorf("quota 取得エラー: %w", err) // 取得エラーを返す
	}
	if int(replicas) > quotaData.MaxReplicasPerDeployment { // 上限を超えている場合はエラーを返す
		return ErrReplicasQuotaExceeded // レプリカ数超過エラーを返す
	}
	return nil // チェック通過を返す
}

// CheckVolumeQuota はユーザーのボリューム使用量が上限に達していないか確認する
func CheckVolumeQuota(ctx context.Context, userQuotaRepo repository.UserQuotaRepository, userID string, additionalMB int) error {
	quotaData, err := userQuotaRepo.GetOrCreate(ctx, userID) // quota を取得する
	if err != nil {
		return fmt.Errorf("quota 取得エラー: %w", err) // 取得エラーを返す
	}
	currentVolumeMB, err := userQuotaRepo.SumVolumeMB(ctx, userID) // 現在のボリューム使用量を集計する
	if err != nil {
		return fmt.Errorf("ボリューム使用量集計エラー: %w", err) // 集計エラーを返す
	}
	if currentVolumeMB+additionalMB > quotaData.MaxVolumeMB { // 追加後に上限を超える場合はエラーを返す
		return ErrVolumeQuotaExceeded // ボリューム容量超過エラーを返す
	}
	return nil // チェック通過を返す
}
