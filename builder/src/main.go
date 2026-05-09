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
	"launchs/shared/job_queue"
	"launchs/shared/model"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/riverqueue/river"
)

func main() {
	fmt.Println("[builder] starting...")

	database.Init()
	database.InitK8s()
	database.InitRedis()
	database.InitTaskDB()

	// DB マイグレーション実行
	if err := database.DB.AutoMigrate(
		&model.Project{},
		&model.Container{},
		&model.BuildJob{},
		&model.Image{},
		&model.Service{},
		&model.Ingress{},
		&model.Volume{},
	); err != nil {
		fmt.Printf("[builder] migration error: %v\n", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// River ワーカーを登録して起動
	workers := river.NewWorkers()
	river.AddWorker(workers, &worker.BuildWorker{})
	river.AddWorker(workers, &worker.DeleteImageWorker{})

	if err := job_queue.UseRiver(ctx, database.TaskDB, workers); err != nil {
		panic("[builder] failed to start job queue: " + err.Error())
	}

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
