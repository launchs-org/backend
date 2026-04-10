package controllers

import (
	"net/http"

	"deploy/services"

	"github.com/labstack/echo/v4"
)

// DeploymentController デプロイメント関連のリクエストを処理するコントローラー
type DeploymentController struct {
	deploymentService services.DeploymentService
}

// NewDeploymentController DeploymentController の新しいインスタンスを作成する
func NewDeploymentController(service services.DeploymentService) *DeploymentController {
	return &DeploymentController{
		deploymentService: service,
	}
}

// CreateDeployment デプロイメント作成リクエストを受け取り、初期状態を返します。
func (controller *DeploymentController) CreateDeployment(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusCreated, echo.Map{"message": "create deployment logic here"})
}

// GetDeployment デプロイメントの詳細設定と現在のステータスを返します。
func (controller *DeploymentController) GetDeployment(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "get deployment logic here"})
}

// ListDeployments プロジェクト内の全てのデプロイメント一覧を返します。
func (controller *DeploymentController) ListDeployments(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "list deployments logic here"})
}

// DeleteDeployment デプロイメント定義を削除し、K8s 上のリソース（Deployment, Service 等）も削除します。
func (controller *DeploymentController) DeleteDeployment(echoContext echo.Context) error {
	return echoContext.NoContent(http.StatusNoContent)
}

// UpdateReplicas 指定された数にレプリカ数を更新します。
func (controller *DeploymentController) UpdateReplicas(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "update replicas logic here"})
}

// UpdateEnvVars 環境変数をリストで受け取り、全件置換します。
func (controller *DeploymentController) UpdateEnvVars(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "update env vars logic here"})
}

// UpdatePorts ポート設定をリストで受け取り、全件置換します。
func (controller *DeploymentController) UpdatePorts(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "update ports logic here"})
}
