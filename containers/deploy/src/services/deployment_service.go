package services

import (
	repo "deploy/models"
	s_models "shared/models"
)

// DeploymentService デプロイメントに関するビジネスロジックを定義するインターフェース
type DeploymentService interface {
	CreateDeployment(deployment *s_models.Deployment) error
	GetDeploymentByID(projectID string, deploymentID string) (*s_models.Deployment, error)
	ListDeploymentsByProject(projectID string) ([]s_models.Deployment, error)
	DeleteDeployment(projectID string, deploymentID string) error
	
	UpdateReplicas(projectID string, deploymentID string, replicas int) error
	UpdateEnvVars(projectID string, deploymentID string, envVars []s_models.EnvVar) error
	UpdatePorts(projectID string, deploymentID string, ports []s_models.Port) error
}

// deploymentService DeploymentService の実装
type deploymentService struct {
}

// NewDeploymentService DeploymentService の新しいインスタンスを作成する
func NewDeploymentService() DeploymentService {
	return &deploymentService{}
}

// CreateDeployment 新しいデプロイメント定義を作成します。
func (service *deploymentService) CreateDeployment(deployment *s_models.Deployment) error {
	return repo.CreateDeployment(deployment)
}

// GetDeploymentByID 指定されたプロジェクトおよび ID のデプロイメント詳細を取得します。
func (service *deploymentService) GetDeploymentByID(projectID string, deploymentID string) (*s_models.Deployment, error) {
	deployment, err := repo.GetDeploymentByID(deploymentID)
	if err != nil {
		return nil, err
	}
	return deployment, nil
}

// ListDeploymentsByProject プロジェクトに属する全てのデプロイメント一覧を取得します。
func (service *deploymentService) ListDeploymentsByProject(projectID string) ([]s_models.Deployment, error) {
	return repo.GetDeploymentsByProject(projectID)
}

// DeleteDeployment 指定されたデプロイメント定義を削除します。
func (service *deploymentService) DeleteDeployment(projectID string, deploymentID string) error {
	return repo.DeleteDeployment(deploymentID)
}

// UpdateReplicas デプロイメントのレプリカ数を更新し、K8s 上の Pod 数を変更します。
func (service *deploymentService) UpdateReplicas(projectID string, deploymentID string, replicas int) error {
	deployment, err := repo.GetDeploymentByID(deploymentID)
	if err != nil {
		return err
	}
	deployment.Replicas = replicas
	return repo.UpdateDeployment(deployment)
}

// UpdateEnvVars デプロイメントに設定された環境変数を一括更新します。
func (service *deploymentService) UpdateEnvVars(projectID string, deploymentID string, envVars []s_models.EnvVar) error {
	// 関連する EnvVar の DeploymentID をセットして保存
	for i := range envVars {
		envVars[i].DeploymentID = deploymentID
	}
	return repo.SaveEnvVarsByDeployment(envVars)
}

// UpdatePorts デプロイメントの公開ポート設定を更新します。
func (service *deploymentService) UpdatePorts(projectID string, deploymentID string, ports []s_models.Port) error {
	// 関連する Port の DeploymentID をセットして保存
	for i := range ports {
		ports[i].DeploymentID = deploymentID
	}
	return repo.SavePortsByDeployment(ports)
}
