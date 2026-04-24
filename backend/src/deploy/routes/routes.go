package routes

import (
	"backend/deploy/controllers"
	"backend/deploy/services"
	"backend/middlewares"

	"github.com/labstack/echo/v5"
)

// InitDeployRoutes はデプロイ管理プロジェクトのルーティングを設定します
func InitDeployRoutes(router *echo.Echo) {
	// サービスの初期化
	projectService := services.NewProjectService()
	containerService := services.NewContainerService()

	// コントローラーの初期化
	projectController := controllers.NewProjectController(projectService)
	containerController := controllers.NewContainerController(containerService)
	buildController := controllers.NewBuildController(containerService)

	// V1 API グループ
	v1 := router.Group("/v1")
	v1.Use(middlewares.RequireAuth) // 全ての V1 エンドポイントに認証を適用

	// プロジェクト関連
	v1.GET("/projects", projectController.GetAllProjects)
	v1.POST("/projects", projectController.CreateProject)
	v1.GET("/projects/:id", projectController.GetProject)
	v1.DELETE("/projects/:id", projectController.DeleteProject)

	// コンテナ関連
	v1.POST("/projects/:id/containers", containerController.CreateContainer)
	v1.PATCH("/containers/:id", containerController.UpdateContainer)
	v1.DELETE("/containers/:id", containerController.DeleteContainer)
	v1.GET("/containers/:id", containerController.GetContainer)

	// ビルドジョブ関連
	v1.POST("/build-jobs/:id/cancel", buildController.CancelBuild)
	v1.GET("/stream/build-jobs/:id", buildController.StreamBuildLogs)
}
