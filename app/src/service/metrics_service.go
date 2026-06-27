package service

import (
	"app/models"
	"app/repository"
	"context"
)

// MetricsService は Deployment メトリクス取得のビジネスロジックを定義するインターフェース
type MetricsService interface {
	GetDeploymentMetrics(ctx context.Context, userID string, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) // Deployment のメトリクス一覧を取得する
}

// metricsServiceImpl は MetricsService の実装
type metricsServiceImpl struct {
	metricsRepo    repository.DeploymentMetricsRepository // メトリクスリポジトリ
	deploymentRepo repository.DeploymentRepository        // deployment リポジトリ（認可チェックに使用する）
	projectRepo    repository.ProjectRepository            // project リポジトリ（認可チェックに使用する）
}

// NewMetricsService は MetricsService の実装を返す
func NewMetricsService(
	metricsRepo repository.DeploymentMetricsRepository,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
) MetricsService {
	return &metricsServiceImpl{
		metricsRepo:    metricsRepo,    // メトリクスリポジトリを注入する
		deploymentRepo: deploymentRepo, // deployment リポジトリを注入する
		projectRepo:    projectRepo,    // project リポジトリを注入する
	}
}

// GetDeploymentMetrics は userID が所有する Deployment のメトリクス一覧を返す
func (svc *metricsServiceImpl) GetDeploymentMetrics(ctx context.Context, userID string, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	projectData, projectErr := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得してオーナーを確認する
	if projectErr != nil {
		return nil, projectErr // 取得エラーを返す
	}

	if projectData.UserID != userID { // UserID が一致しない場合はアクセスを拒否する
		return nil, ErrForbidden // 禁止エラーを返す
	}

	return svc.metricsRepo.FindByDeploymentID(ctx, deploymentID, limit) // メトリクスを返す
}
