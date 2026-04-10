package main

import (
	"deploy/controllers"
	"deploy/services"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	echoServer := echo.New()
	echoServer.Use(middleware.Logger())
	echoServer.Use(middleware.Recover())

	// サービスの初期化
	projectService := services.NewProjectService()
	deploymentService := services.NewDeploymentService()

	// コントローラーの初期化
	projectController := controllers.NewProjectController(projectService)
	deploymentController := controllers.NewDeploymentController(deploymentService)

	// ルート設定
	echoServer.GET("/", func(context echo.Context) error {
		return context.String(http.StatusOK, "Deploy Service")
	})

	// V1 API グループ
	apiV1 := echoServer.Group("/v1")

	// プロジェクト関連
	projects := apiV1.Group("/projects")
	{
		projects.POST("", projectController.CreateProject)
		projects.GET("", projectController.ListProjects)
		projects.GET("/:project_id", projectController.GetProject)
		projects.DELETE("/:project_id", projectController.DeleteProject)

		// デプロイメント関連 (プロジェクトに紐づく)
		deployments := projects.Group("/:project_id/deployments")
		{
			deployments.POST("", deploymentController.CreateDeployment)
			deployments.GET("", deploymentController.ListDeployments)
			deployments.GET("/:id", deploymentController.GetDeployment)
			deployments.DELETE("/:id", deploymentController.DeleteDeployment)
			deployments.PATCH("/:id/replicas", deploymentController.UpdateReplicas)
			deployments.PUT("/:id/env", deploymentController.UpdateEnvVars)
			deployments.PUT("/:id/ports", deploymentController.UpdatePorts)
		}
	}

	echoServer.Logger.Fatal(echoServer.Start(":8080"))
}
