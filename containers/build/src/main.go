package main

import (
	"build/controllers"
	"build/services"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	echoServer := echo.New()
	echoServer.Use(middleware.Logger())
	echoServer.Use(middleware.Recover())

	// サービスの初期化
	buildService := services.NewBuildService()

	// コントローラーの初期化
	buildController := controllers.NewBuildController(buildService)

	// ルート設定
	echoServer.GET("/", func(context echo.Context) error {
		return context.String(http.StatusOK, "Build Service")
	})

	// V1 API グループ
	apiV1 := echoServer.Group("/v1")
	{
		// ビルド関連 (設計に基づき、Project/Deployment の階層下に置くことも検討可能)
		builds := apiV1.Group("/builds") 
		{
			builds.POST("", buildController.TriggerBuild)
			builds.GET("/:container_id/status", buildController.GetBuildStatus)
			builds.GET("/:container_id/logs", buildController.GetBuildLogs)
		}
	}

	echoServer.Logger.Fatal(echoServer.Start(":8080"))
}
