package services

import (
	"deploy/models"
)

// DeploymentService デプロイメントに関するビジネスロジックを定義するインターフェース
type DeploymentService interface {
	CreateDeployment(deployment *models.Deployment) error
	GetDeploymentByID(projectID string, deploymentID string) (*models.Deployment, error)
	ListDeploymentsByProject(projectID string) ([]models.Deployment, error)
	DeleteDeployment(projectID string, deploymentID string) error
	
	UpdateReplicas(projectID string, deploymentID string, replicas int) error
	UpdateEnvVars(projectID string, deploymentID string, envVars []models.EnvVar) error
	UpdatePorts(projectID string, deploymentID string, ports []models.Port) error
}

// deploymentService DeploymentService の実装
type deploymentService struct {
	// DB インスタンスや K8s クライアントなどをここに保持する
}

// NewDeploymentService DeploymentService の新しいインスタンスを作成する
func NewDeploymentService() DeploymentService {
	return &deploymentService{}
}

// CreateDeployment 新しいデプロイメント定義を作成します。
func (service *deploymentService) CreateDeployment(deployment *models.Deployment) error {
	// 実装はユーザーが行う
	return nil
}

// GetDeploymentByID 指定されたプロジェクトおよび ID のデプロイメント詳細を取得します。
func (service *deploymentService) GetDeploymentByID(projectID string, deploymentID string) (*models.Deployment, error) {
	// 実装はユーザーが行う
	return nil, nil
}

// ListDeploymentsByProject プロジェクトに属する全てのデプロイメント一覧を取得します。
func (service *deploymentService) ListDeploymentsByProject(projectID string) ([]models.Deployment, error) {
	// 実装はユーザーが行う
	return nil, nil
}

// DeleteDeployment 指定されたデプロイメント定義を削除します。
func (service *deploymentService) DeleteDeployment(projectID string, deploymentID string) error {
	// 実装はユーザーが行う
	return nil
}

// UpdateReplicas デプロイメントのレプリカ数を更新し、K8s 上の Pod 数を変更します。
func (service *deploymentService) UpdateReplicas(projectID string, deploymentID string, replicas int) error {
	// 実装はユーザーが行う
	return nil
}

// UpdateEnvVars デプロイメントに設定された環境変数を一括更新します。
func (service *deploymentService) UpdateEnvVars(projectID string, deploymentID string, envVars []models.EnvVar) error {
	// 実装はユーザーが行う
	return nil
}

// UpdatePorts デプロイメントの公開ポート設定を更新します。
func (service *deploymentService) UpdatePorts(projectID string, deploymentID string, ports []models.Port) error {
	// 実装はユーザーが行う
	return nil
}
