package main

import (
	"context"
	"launchs/shared/database"
	"launchs/shared/job_queue"
	"backend/middlewares"
	"launchs/shared/model"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

// main はアプリケーションのエントリーポイントです
func main() {
	// データベース接続の初期化
	database.Init()
	// Redis の初期化
	database.InitRedis()

	// k8s クライアントを初期化
	database.InitK8s()

	// River ジョブキュー用 DB の初期化（挿入専用クライアント）
	database.InitTaskDB()
	if err := job_queue.UseRiver(context.Background(), database.TaskDB, nil); err != nil {
		panic("failed to initialize job queue: " + err.Error())
	}

	// データベースの自動マイグレーションを実行 (各種テーブルの作成・更新)
	// Task テーブルは init.sql で作成済みなため除外
	if err := database.DB.AutoMigrate(
		&model.Project{},
		&model.Container{},
		&model.BuildJob{},
		&model.Image{},
		&model.Service{},
		&model.Ingress{},
		&model.Volume{},
	); err != nil {
		panic("failed to migrate database: " + err.Error())
	}

	// Echo インスタンスを作成
	router := echo.New()
	// リクエストログ出力ミドルウェアを適用
	router.Use(middleware.RequestLogger())
	// パニック復帰ミドルウェアを適用
	// router.Use(middleware.Recover())

	// ミドルウェアのグローバル初期化
	middlewares.Init()

	// ルーティング設定の初期化
	InitRouter(router)

	// ルートパスのエンドポイント
	router.GET("/", func(ctx *echo.Context) error {
		// Welcomeメッセージを返す
		return (*ctx).String(http.StatusOK, "Hello, World!")
	})

	// ポート 8090 でサーバーを起動
	if err := router.Start("0.0.0.0:8090"); err != nil {
		// 起動失敗時はログ出力
		router.Logger.Error("failed to start server", "error", err)
	}
}
