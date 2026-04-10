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

func (service *deploymentService) CreateDeployment(deployment *models.Deployment) error {
	// 実装はユーザーが行う
	return nil
}

func (service *deploymentService) GetDeploymentByID(projectID string, deploymentID string) (*models.Deployment, error) {
	// 実装はユーザーが行う
	return nil, nil
}

func (service *deploymentService) ListDeploymentsByProject(projectID string) ([]models.Deployment, error) {
	// 実装はユーザーが行う
	return nil, nil
}

func (service *deploymentService) DeleteDeployment(projectID string, deploymentID string) error {
	// 実装はユーザーが行う
	return nil
}

func (service *deploymentService) UpdateReplicas(projectID string, deploymentID string, replicas int) error {
	// 実装はユーザーが行う
	return nil
}

func (service *deploymentService) UpdateEnvVars(projectID string, deploymentID string, envVars []models.EnvVar) error {
	// 実装はユーザーが行う
	return nil
}

func (service *deploymentService) UpdatePorts(projectID string, deploymentID string, ports []models.Port) error {
	// 実装はユーザーが行う
	return nil
}
