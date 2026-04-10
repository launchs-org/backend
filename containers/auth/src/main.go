package main

import (
	"auth/controllers"
	"auth/pkg/database"
	"auth/services"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// データベース接続の初期化
	if err := database.Connect(); err != nil {
		log.Fatalf("データベース接続に失敗しました: %v", err)
	}

	echoServer := echo.New()
	echoServer.Use(middleware.Logger())
	echoServer.Use(middleware.Recover())

	// サービスの初期化
	authService := services.NewAuthService()

	// コントローラーの初期化
	authController := controllers.NewAuthController(authService)

	// ルート設定
	echoServer.GET("/", func(context echo.Context) error {
		return context.String(http.StatusOK, "Auth Service")
	})

	// V1 API グループ
	apiV1 := echoServer.Group("/v1")
	{
		apiV1.POST("/login", authController.Login)
		apiV1.GET("/validate", authController.ValidateToken)
	}

	echoServer.Logger.Fatal(echoServer.Start(":8080"))
}
