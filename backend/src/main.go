package main

import (
	"backend/database"    // データベース
	"backend/middlewares" // ミドルウェア
	"backend/model"       // モデル
	"backend/k8slogwatcher" // Kubernetes ログウォッチャー
	"net/http" // HTTP

	"github.com/labstack/echo/v5"            // Echo
	"github.com/labstack/echo/v5/middleware" // Echoミドルウェア
)

// main はアプリケーションのエントリーポイントです
func main() {
	// データベース接続の初期化
	database.Init()
	// Kubernetes クライアントの初期化
	database.InitK8s()
	// Redis の初期化
	database.InitRedis()
	// Kubernetes ログウォッチャーの初期化
	k8slogwatcher.Init()

	// データベースの自動マイグレーションを実行 (各種テーブルの作成・更新)
	if err := database.DB.AutoMigrate(
		&model.Project{},
		&model.Container{},
		&model.BuildJob{},
		&model.Image{},
		&model.Service{},
	); err != nil {
		// マイグレーション失敗時はパニック
		panic("failed to migrate database: " + err.Error())
	}

	// Echo インスタンスを作成
	router := echo.New()
	// リクエストログ出力ミドルウェアを適用
	router.Use(middleware.RequestLogger())
	// パニック復帰ミドルウェアを適用
	router.Use(middleware.Recover())

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
