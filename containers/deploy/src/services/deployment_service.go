package services

import (
	s_models "github.com/launchs-org/backend/shared/models"

	"gorm.io/gorm"
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
	db *s_models.Database
}

// NewDeploymentService DeploymentService の新しいインスタンスを作成する
func NewDeploymentService(db *s_models.Database) DeploymentService {
	return &deploymentService{
		db: db,
	}
}

// CreateDeployment 新しいデプロイメント定義を作成し、データベースに保存します。
func (service *deploymentService) CreateDeployment(deployment *s_models.Deployment) error {
	// 指定されたデプロイメント情報を保存
	return service.db.Conn.Create(deployment).Error
}

// GetDeploymentByID 指定された ID のデプロイメント詳細を取得します。
func (service *deploymentService) GetDeploymentByID(projectID string, deploymentID string) (*s_models.Deployment, error) {
	var deployment s_models.Deployment
	// IDに基づいてデプロイメント情報を取得
	if err := service.db.Conn.First(&deployment, "id = ?", deploymentID).Error; err != nil {
		return nil, err
	}
	return &deployment, nil
}

// ListDeploymentsByProject プロジェクトに属する全てのデプロイメント一覧を取得します。
func (service *deploymentService) ListDeploymentsByProject(projectID string) ([]s_models.Deployment, error) {
	var deployments []s_models.Deployment
	// プロジェクトIDに紐付くデプロイメントを全件取得
	if err := service.db.Conn.Find(&deployments, "project_id = ?", projectID).Error; err != nil {
		return nil, err
	}
	return deployments, nil
}

// DeleteDeployment 指定されたデプロイメント定義を削除します。
func (service *deploymentService) DeleteDeployment(projectID string, deploymentID string) error {
	// デプロイメント定義の削除
	return service.db.Conn.Delete(&s_models.Deployment{}, "id = ?", deploymentID).Error
}

// UpdateReplicas デプロイメントのレプリカ数を更新します。
func (service *deploymentService) UpdateReplicas(projectID string, deploymentID string, replicas int) error {
	// 指定されたIDのレプリカ数カラムのみを更新
	return service.db.Conn.Model(&s_models.Deployment{}).Where("id = ?", deploymentID).Update("replicas", replicas).Error
}

// UpdateEnvVars デプロイメントに設定された環境変数を一括更新します。
func (service *deploymentService) UpdateEnvVars(projectID string, deploymentID string, envVars []s_models.EnvVar) error {
	// トランザクションを利用して古い環境変数を削除し、新しいものを保存する例
	return service.db.Conn.Transaction(func(tx *gorm.DB) error {
		// 古い設定を削除
		if err := tx.Delete(&s_models.EnvVar{}, "deployment_id = ?", deploymentID).Error; err != nil {
			return err
		}
		// 新しい設定をバルクインサート
		for i := range envVars {
			envVars[i].DeploymentID = deploymentID
		}
		return tx.Create(&envVars).Error
	})
}

// UpdatePorts デプロイメントの公開ポート設定を更新します。
func (service *deploymentService) UpdatePorts(projectID string, deploymentID string, ports []s_models.Port) error {
	return service.db.Conn.Transaction(func(tx *gorm.DB) error {
		// 古いポート設定を削除
		if err := tx.Delete(&s_models.Port{}, "deployment_id = ?", deploymentID).Error; err != nil {
			return err
		}
		// 新しいポート設定を保存
		for i := range ports {
			ports[i].DeploymentID = deploymentID
		}
		return tx.Create(&ports).Error
	})
}
