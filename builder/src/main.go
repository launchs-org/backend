package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"builder/controller"
	"builder/worker"
	"launchs/shared/database"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func main() {
	fmt.Println("[builder] starting...")

	database.Init()
	database.InitRedis()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ワーカー起動
	go worker.RunDeleteImageWorker(ctx)
	go worker.RunTimeoutWorker(ctx)

	// Echo HTTP サーバー起動
	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	// /internal/upload — K8s Job の tar-push コンテナからのアップロードを受け取る
	e.POST("/internal/upload", func(c *echo.Context) error {
		return controller.UploadTar(c)
	})

	e.GET("/", func(c *echo.Context) error {
		return (*c).String(http.StatusOK, "builder ok")
	})

	port := os.Getenv("BUILDER_PORT")
	if port == "" {
		port = "8091"
	}

	fmt.Printf("[builder] listening on :%s\n", port)
	if err := e.Start(":" + port); err != nil {
		e.Logger.Error("server error", "error", err)
	}
}
