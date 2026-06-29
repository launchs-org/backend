package router

import (
	"app/handler"
	"app/middlewares"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// RouterOptions はルーター生成に必要なハンドラーをまとめた構造体
type RouterOptions struct {
	UserQuotaHandler           *handler.UserQuotaHandler           // quota ハンドラー
	ProjectHandler             *handler.ProjectHandler             // project ハンドラー
	DeploymentHandler          *handler.DeploymentHandler          // deployment ハンドラー
	DeploymentTemplateHandler  *handler.DeploymentTemplateHandler  // deployment template ハンドラー
	IngressRouteHandler        *handler.IngressRouteHandler        // ingress_route ハンドラー
	EnvVarHandler              *handler.EnvVarHandler              // env_var ハンドラー
	VolumeHandler              *handler.VolumeHandler              // volume ハンドラー
	BuildHandler               *handler.BuildHandler               // build ハンドラー
	WebhookHandler             *handler.WebhookHandler             // webhook ハンドラー
	LogHandler                 *handler.LogHandler                 // log ハンドラー
	MetricsHandler             *handler.MetricsHandler             // metrics ハンドラー
}

// New はミドルウェアとルーティングを設定した Echo インスタンスを返す
func New(opts RouterOptions) *echo.Echo {
	router := echo.New() // Echo インスタンスを生成する

	router.Use(middleware.Logger())  // リクエストログミドルウェアを設定する
	router.Use(middleware.Recover()) // パニックリカバリミドルウェアを設定する

	// 認証必須の API グループを作成する
	apiGroup := router.Group("/api/v1", middlewares.RequireAuth)

	// 管理者専用の API グループを作成する
	adminGroup := router.Group("/api/v1", middlewares.RequireAuth, middlewares.RequireAdmin)

	// quota エンドポイントを登録する
	apiGroup.GET("/users/quota", opts.UserQuotaHandler.GetQuota)    // quota 取得エンドポイント
	apiGroup.PUT("/users/quota", opts.UserQuotaHandler.UpdateQuota) // quota 更新エンドポイント

	// project エンドポイントを登録する
	apiGroup.GET("/projects", opts.ProjectHandler.ListProjects)                   // project 一覧取得エンドポイント
	apiGroup.POST("/projects", opts.ProjectHandler.CreateProject)                 // project 作成エンドポイント
	apiGroup.GET("/projects/:id", opts.ProjectHandler.GetProject)                 // project 詳細取得エンドポイント
	apiGroup.PUT("/projects/:id", opts.ProjectHandler.UpdateProject)              // project 更新エンドポイント
	apiGroup.DELETE("/projects/:id", opts.ProjectHandler.DeleteProject)           // project 削除エンドポイント
	apiGroup.GET("/projects/:id/quota", opts.ProjectHandler.GetProjectQuota)      // project クォータ取得エンドポイント
	apiGroup.GET("/projects/:id/builds", opts.BuildHandler.ListBuildsByProject)              // project 単位ビルド一覧取得エンドポイント
	apiGroup.DELETE("/projects/:id/builds/:buildId", opts.BuildHandler.DeleteBuild)        // project 単位ビルド削除エンドポイント

	// deployment エンドポイントを登録する
	apiGroup.GET("/projects/:id/deployments", opts.DeploymentHandler.ListDeployments)    // deployment 一覧取得エンドポイント
	apiGroup.POST("/projects/:id/deployments", opts.DeploymentHandler.CreateDeployment)  // deployment 作成エンドポイント
	apiGroup.GET("/deployments/:id", opts.DeploymentHandler.GetDeployment)               // deployment 詳細取得エンドポイント
	apiGroup.PUT("/deployments/:id", opts.DeploymentHandler.UpdateDeployment)            // deployment 更新エンドポイント
	apiGroup.DELETE("/deployments/:id", opts.DeploymentHandler.DeleteDeployment)         // deployment 削除エンドポイント
	apiGroup.POST("/deployments/:id/apply", opts.DeploymentHandler.ApplyDeployment)                       // deployment apply エンドポイント
	apiGroup.POST("/deployments/:id/discard-pending", opts.DeploymentHandler.DiscardPending)              // deployment pending クリアエンドポイント
	apiGroup.GET("/deployments/:id/apply-histories", opts.DeploymentHandler.ListApplyHistories)   // apply 履歴一覧取得エンドポイント
	apiGroup.GET("/deployments/:id/service", opts.DeploymentHandler.GetService)                   // service 設定取得エンドポイント
	apiGroup.POST("/deployments/:id/service", opts.DeploymentHandler.CreateService)               // service 作成エンドポイント
	apiGroup.PUT("/deployments/:id/service", opts.DeploymentHandler.UpdateService)                // service 設定更新エンドポイント
	apiGroup.DELETE("/deployments/:id/service", opts.DeploymentHandler.DeleteService)             // service 削除エンドポイント

	// build エンドポイントを登録する
	apiGroup.GET("/deployments/:id/builds", opts.BuildHandler.ListBuilds)   // ビルド一覧取得エンドポイント
	apiGroup.POST("/deployments/:id/build", opts.BuildHandler.TriggerBuild) // ビルドトリガーエンドポイント
	apiGroup.GET("/builds/:id", opts.BuildHandler.GetBuild)                  // ビルド取得エンドポイント
	apiGroup.DELETE("/builds/:id", opts.BuildHandler.CancelBuild)           // ビルドキャンセルエンドポイント
	apiGroup.GET("/builds/:id/logs", opts.BuildHandler.GetBuildLogs)        // ビルドログ取得エンドポイント

	// log エンドポイントを登録する
	apiGroup.GET("/deployments/:id/logs", opts.LogHandler.GetPodLogs) // Pod ログ取得エンドポイント

	// metrics エンドポイントを登録する
	apiGroup.GET("/deployments/:id/metrics", opts.MetricsHandler.GetDeploymentMetrics) // Deployment メトリクス取得エンドポイント

	// webhook エンドポイントを登録する
	apiGroup.POST("/deployments/:id/webhooks", opts.WebhookHandler.CreateWebhook) // webhook 作成エンドポイント
	apiGroup.GET("/deployments/:id/webhooks", opts.WebhookHandler.GetWebhook)     // webhook 取得エンドポイント
	apiGroup.DELETE("/webhooks/:id", opts.WebhookHandler.DeleteWebhook)           // webhook 削除エンドポイント

	// 認証不要の Webhook レシーバーを登録する（GitHub からのリクエストには認証ヘッダーがないため）
	router.POST("/webhooks/:deployment_id/github", opts.WebhookHandler.ReceiveGithubWebhook) // GitHub push イベントレシーバーエンドポイント

	// ingress-route エンドポイントを登録する
	apiGroup.GET("/projects/:id/ingress-routes", opts.IngressRouteHandler.ListIngressRoutes)                                // ingress-route 一覧取得エンドポイント
	apiGroup.POST("/projects/:id/ingress-routes", opts.IngressRouteHandler.CreateIngressRoute)                              // ingress-route 作成エンドポイント
	apiGroup.DELETE("/ingress-routes/:id", opts.IngressRouteHandler.DeleteIngressRoute)                                     // ingress-route 削除エンドポイント
	apiGroup.PATCH("/ingress-routes/:id/name", opts.IngressRouteHandler.UpdateIngressRouteName)                             // ingress-route 名前変更エンドポイント
	apiGroup.POST("/projects/:id/apply", opts.IngressRouteHandler.ApplyProject)                                             // project 単位 IngressRoute apply エンドポイント
	apiGroup.GET("/ingress-routes/:id/path-rules", opts.IngressRouteHandler.ListPathRules)                                  // path-rule 一覧取得エンドポイント
	apiGroup.POST("/ingress-routes/:id/path-rules", opts.IngressRouteHandler.CreatePathRule)                                // path-rule 作成エンドポイント
	apiGroup.DELETE("/ingress-routes/:id/path-rules/:pathRuleID", opts.IngressRouteHandler.DeletePathRule)                  // path-rule 削除エンドポイント

	// env-vars エンドポイントを登録する
	apiGroup.GET("/projects/:id/env-vars", opts.EnvVarHandler.ListEnvVars)    // env_var 一覧取得エンドポイント
	apiGroup.POST("/projects/:id/env-vars", opts.EnvVarHandler.CreateEnvVar)  // env_var 作成エンドポイント
	apiGroup.PUT("/env-vars/:id", opts.EnvVarHandler.UpdateEnvVar)            // env_var 更新エンドポイント
	apiGroup.DELETE("/env-vars/:id", opts.EnvVarHandler.DeleteEnvVar)         // env_var 削除エンドポイント

	// env-var-mounts エンドポイントを登録する
	apiGroup.GET("/deployments/:id/env-var-mounts", opts.EnvVarHandler.ListEnvVarMounts)    // マウント設定一覧取得エンドポイント
	apiGroup.POST("/deployments/:id/env-var-mounts", opts.EnvVarHandler.CreateEnvVarMount)  // マウント設定作成エンドポイント
	apiGroup.DELETE("/env-var-mounts/:id", opts.EnvVarHandler.DeleteEnvVarMount)            // マウント設定削除エンドポイント

	// volumes エンドポイントを登録する
	apiGroup.GET("/projects/:id/volumes", opts.VolumeHandler.ListVolumes)    // volume 一覧取得エンドポイント
	apiGroup.POST("/projects/:id/volumes", opts.VolumeHandler.CreateVolume)  // volume 作成エンドポイント
	apiGroup.DELETE("/volumes/:id", opts.VolumeHandler.DeleteVolume)         // volume 削除エンドポイント

	// volume-mounts エンドポイントを登録する
	apiGroup.GET("/deployments/:id/volume-mounts", opts.VolumeHandler.ListVolumeMounts)    // volume-mount 一覧取得エンドポイント
	apiGroup.POST("/deployments/:id/volume-mounts", opts.VolumeHandler.CreateVolumeMount)  // volume-mount 作成エンドポイント
	apiGroup.DELETE("/volume-mounts/:id", opts.VolumeHandler.DeleteVolumeMount)            // volume-mount 削除エンドポイント

	// deployment-templates エンドポイントを登録する（全認証ユーザー向け）
	apiGroup.GET("/deployment-templates", opts.DeploymentTemplateHandler.ListTemplates)          // テンプレート一覧取得エンドポイント
	apiGroup.GET("/deployment-templates/:id", opts.DeploymentTemplateHandler.GetTemplate)        // テンプレート詳細取得エンドポイント
	apiGroup.POST("/projects/:id/deployments/from-template", opts.DeploymentTemplateHandler.CreateDeploymentFromTemplate) // テンプレートからデプロイメント作成エンドポイント

	// deployment-templates 管理者専用エンドポイントを登録する
	adminGroup.POST("/deployment-templates", opts.DeploymentTemplateHandler.CreateTemplate)         // テンプレート作成エンドポイント（管理者専用）
	adminGroup.PUT("/deployment-templates/:id", opts.DeploymentTemplateHandler.UpdateTemplate)      // テンプレート更新エンドポイント（管理者専用）
	adminGroup.DELETE("/deployment-templates/:id", opts.DeploymentTemplateHandler.DeleteTemplate)   // テンプレート削除エンドポイント（管理者専用）

	return router
}
