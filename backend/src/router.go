package main

import (
	"backend/controller" // コントローラーパッケージ
	"backend/middlewares" // ミドルウェアパッケージ
	"net/http"            // HTTPステータスコード
	"github.com/labstack/echo/v5" // Echoフレームワーク
)

// InitRouter はルーターを初期化する関数です
func InitRouter(router *echo.Echo) {
	// API V1用のグループを作成する
	v1Group := router.Group("/v1")
	// V1グループ全体に認証ミドルウェアを適用する
	v1Group.Use(middlewares.RequireAuth)
	
	// V1グループ内のルーティングを定義する
	{
		// プロジェクト関連のリソースグループを作成する
		projectsGroup := v1Group.Group("/projects")
		{
			// GET /v1/projects - プロジェクト一覧を取得する
			projectsGroup.GET("", controller.ListProjects)

			// POST /v1/projects - プロジェクトを新規作成する
			projectsGroup.POST("", controller.CreateProject)

			// GET /v1/projects/:id - プロジェクトの詳細取得
			projectsGroup.GET("/:id", controller.GetProject)

			// DELETE /v1/projects/:id - プロジェクトの削除 (未実装)
			projectsGroup.DELETE("/:id", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})

			// POST /v1/projects/:id/containers - コンテナの作成とビルド (未実装)
			projectsGroup.POST("/:id/containers", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})

			// GET /v1/projects/:id/histories - プロジェクトのスナップショット一覧 (未実装)
			projectsGroup.GET("/:id/histories", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})

			// POST /v1/projects/:id/rollback/:phid - ロールバック実行 (未実装)
			projectsGroup.POST("/:id/rollback/:phid", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})
		}

		// コンテナ関連のグループ
		containersGroup := v1Group.Group("/containers")
		{
			// PATCH /v1/containers/:id - コンテナ設定更新 (未実装)
			containersGroup.PATCH("/:id", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})

			// DELETE /v1/containers/:id - コンテナの削除 (未実装)
			containersGroup.DELETE("/:id", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})

			// GET /v1/containers/:id/build-jobs - ビルド履歴一覧 (未実装)
			containersGroup.GET("/:id/build-jobs", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})

			// GET /v1/containers/:id/logs - 実行ログ取得 (未実装)
			containersGroup.GET("/:id/logs", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})

			// PATCH /v1/containers/:id/service - サービス設定更新 (未実装)
			containersGroup.PATCH("/:id/service", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})

			// POST /v1/containers/:id/ingress - Ingress作成 (未実装)
			containersGroup.POST("/:id/ingress", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})

			// DELETE /v1/containers/:id/ingress - Ingress削除 (未実装)
			containersGroup.DELETE("/:id/ingress", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})
		}

		// ビルドジョブ関連のグループ
		buildJobsGroup := v1Group.Group("/build-jobs")
		{
			// POST /v1/build-jobs/:id/cancel - ビルドキャンセル (未実装)
			buildJobsGroup.POST("/:id/cancel", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})
		}
	}

}