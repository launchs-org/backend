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

			// DELETE /v1/projects/:id - プロジェクトの削除
			projectsGroup.DELETE("/:id", controller.DeleteProject)

			// POST /v1/projects/:id/containers - コンテナの作成とビルド
			projectsGroup.POST("/:id/containers", controller.CreateContainer)

			// GET /v1/projects/:id/histories - プロジェクトのスナップショット一覧 (未実装)
			projectsGroup.GET("/:id/histories", func(c *echo.Context) error {
				// モックレスポンス
				return c.JSON(http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"items": []map[string]interface{}{
							{
								"id":           "hist_dummy1",
								"version_name": "v1.0.0",
								"created_at":   "2026-05-01T10:00:00Z",
							},
							{
								"id":           "hist_dummy2",
								"version_name": "v0.9.0",
								"created_at":   "2026-04-20T08:30:00Z",
							},
						},
						"total": 2,
					},
				})
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
			// GET /v1/containers/:id - コンテナの詳細取得
			containersGroup.GET("/:id", controller.GetContainer)

			// PATCH /v1/containers/:id - コンテナ設定更新
			containersGroup.PATCH("/:id", controller.UpdateContainer)

			// POST /v1/containers/:id/rebuild - 再ビルド
			containersGroup.POST("/:id/rebuild", controller.RebuildContainer)

			// POST /v1/containers/:id/redeploy - 再デプロイ
			containersGroup.POST("/:id/redeploy", controller.RedeployContainer)

			// DELETE /v1/containers/:id - コンテナの削除
			containersGroup.DELETE("/:id", controller.DeleteContainer)

			// GET /v1/containers/:id/build-jobs - ビルド履歴一覧
			containersGroup.GET("/:id/build-jobs", controller.ListBuildJobs)

			// GET /v1/containers/:id/logs - 実行ログ取得 (未実装)
			containersGroup.GET("/:id/logs", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})

			// PATCH /v1/containers/:id/service - サービス設定更新
			containersGroup.PATCH("/:id/service", controller.UpdateService)

			// POST /v1/containers/:id/ingress - Ingress作成
			containersGroup.POST("/:id/ingress", controller.CreateIngress)

			// PATCH /v1/containers/:id/ingress - Ingress更新
			containersGroup.PATCH("/:id/ingress", controller.UpdateIngress)

			// DELETE /v1/containers/:id/ingress - Ingress削除
			containersGroup.DELETE("/:id/ingress", controller.DeleteIngress)

			// POST /v1/containers/:id/volumes - ボリューム作成
			containersGroup.POST("/:id/volumes", controller.CreateContainerVolume)
			// GET /v1/containers/:id/volumes - ボリューム一覧
			containersGroup.GET("/:id/volumes", controller.ListContainerVolumes)
		}

		// ビルドジョブ関連のグループ
		buildJobsGroup := v1Group.Group("/build-jobs")
		{
			// POST /v1/build-jobs/:id/cancel - ビルドキャンセル (未実装)
			buildJobsGroup.POST("/:id/cancel", func(c *echo.Context) error {
				// モックレスポンス
				return c.String(http.StatusOK, "Hello, World!")
			})

			// GET /v1/build-jobs/:id/logs - ビルドログの取得
			buildJobsGroup.GET("/:id/logs", controller.GetBuildJobLogs)
		}

		// ボリューム関連のグループ
		volumesGroup := v1Group.Group("/volumes")
		{
			// DELETE /v1/volumes/:id - ボリューム削除
			volumesGroup.DELETE("/:id", controller.DeleteVolume)
		}

		// stream関連のリソースグループを作成する
		streamGroup := v1Group.Group("/stream")
		{
			// GET /v1/stream/build-jobs/:id - ビルドログの取得 (ポーリング用)
			streamGroup.GET("/build-jobs/:id", controller.GetBuildJobLogs)
		}

	}

}