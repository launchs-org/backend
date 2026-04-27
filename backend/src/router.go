package main

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// ルーターを初期化する関数
func InitRouter(router *echo.Echo) {
	// V1グループを作成する
	v1Group := router.Group("/v1")
	// V1グループ内のルーティングを定義する
	{
		// プロジェクトグループを作成する
		projectsGroup := v1Group.Group("/projects")
		{
			// プロジェクト一覧を取得する
			projectsGroup.GET("/", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})

			// プロジェクトを新規作成する
			projectsGroup.POST("/", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})

			// プロジェクトの詳細取得
			projectsGroup.GET("/:id", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})

			// プロジェクトの削除
			projectsGroup.DELETE("/:id", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})

			// コンテナの作成とビルドを行うエンドポイント
			projectsGroup.POST("/:id/containers", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})

			// プロジェクトのスナップショット一覧の取得
			projectsGroup.GET("/:id/histories", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})

			// 特定のスナップショットにロールバックを行う
			projectsGroup.POST("/:id/rollback/:phid", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})
		}

		// コンテナ関連のグループ
		containersGroup := v1Group.Group("/containers")
		{
			// コンテナの設定更新と再ビルドのキューを行う
			containersGroup.PATCH("/:id", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})

			// コンテナの削除 (関連リソースの削除)
			containersGroup.DELETE("/:id", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})

			// コンテナのビルド履歴一覧 (ログは無)
			containersGroup.GET("/:id/build-jobs", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})

			// コンテナ自身の実行ログを取得
			containersGroup.GET("/:id/logs", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})

			// コンテナのサービスを更新する (丸ごと差し替え)
			containersGroup.PATCH("/:id/service", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})

			// ingress で外部に公開する (ドメインの払い出しも)
			containersGroup.POST("/:id/ingress", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})

			// Ingressの公開停止
			containersGroup.DELETE("/:id/ingress", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})
		}

		// ビルドジョブ系のAPI
		buildJobsGroup := v1Group.Group("/build-jobs")
		{
			// ビルドジョブをキャンセルする
			buildJobsGroup.POST("/:id/cancel", func(c *echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			})
		}
	}
}