package router

import (
	"app/handler"
	"app/middlewares"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// RouterOptions はルーター生成に必要なハンドラーをまとめた構造体
type RouterOptions struct {
	UserQuotaHandler   *handler.UserQuotaHandler   // quota ハンドラー
	ProjectHandler     *handler.ProjectHandler     // project ハンドラー
	DeploymentHandler  *handler.DeploymentHandler  // deployment ハンドラー
	EnvVarHandler      *handler.EnvVarHandler      // env_var ハンドラー
	VolumeHandler      *handler.VolumeHandler      // volume ハンドラー
	BuildHandler       *handler.BuildHandler       // build ハンドラー
}

// New はミドルウェアとルーティングを設定した Echo インスタンスを返す
func New(opts RouterOptions) *echo.Echo {
	router := echo.New() // Echo インスタンスを生成する

	router.Use(middleware.Logger())  // リクエストログミドルウェアを設定する
	router.Use(middleware.Recover()) // パニックリカバリミドルウェアを設定する

	// 認証必須の API グループを作成する
	apiGroup := router.Group("/api/v1", middlewares.RequireAuth)

	// quota エンドポイントを登録する
	apiGroup.GET("/users/quota", opts.UserQuotaHandler.GetQuota)    // quota 取得エンドポイント
	apiGroup.PUT("/users/quota", opts.UserQuotaHandler.UpdateQuota) // quota 更新エンドポイント

	// project エンドポイントを登録する
	apiGroup.GET("/projects", opts.ProjectHandler.ListProjects)          // project 一覧取得エンドポイント
	apiGroup.POST("/projects", opts.ProjectHandler.CreateProject)        // project 作成エンドポイント
	apiGroup.GET("/projects/:id", opts.ProjectHandler.GetProject)        // project 詳細取得エンドポイント
	apiGroup.PUT("/projects/:id", opts.ProjectHandler.UpdateProject)     // project 更新エンドポイント
	apiGroup.DELETE("/projects/:id", opts.ProjectHandler.DeleteProject)  // project 削除エンドポイント

	// deployment エンドポイントを登録する
	apiGroup.GET("/projects/:id/deployments", opts.DeploymentHandler.ListDeployments)    // deployment 一覧取得エンドポイント
	apiGroup.POST("/projects/:id/deployments", opts.DeploymentHandler.CreateDeployment)  // deployment 作成エンドポイント
	apiGroup.GET("/deployments/:id", opts.DeploymentHandler.GetDeployment)               // deployment 詳細取得エンドポイント
	apiGroup.PUT("/deployments/:id", opts.DeploymentHandler.UpdateDeployment)            // deployment 更新エンドポイント
	apiGroup.DELETE("/deployments/:id", opts.DeploymentHandler.DeleteDeployment)         // deployment 削除エンドポイント
	apiGroup.POST("/deployments/:id/apply", opts.DeploymentHandler.ApplyDeployment)               // deployment apply エンドポイント
	apiGroup.GET("/deployments/:id/apply-histories", opts.DeploymentHandler.ListApplyHistories)   // apply 履歴一覧取得エンドポイント
	apiGroup.GET("/deployments/:id/service", opts.DeploymentHandler.GetService)                   // service 設定取得エンドポイント
	apiGroup.PUT("/deployments/:id/service", opts.DeploymentHandler.UpdateService)                // service 設定更新エンドポイント

	// build エンドポイントを登録する
	apiGroup.POST("/deployments/:id/build", opts.BuildHandler.TriggerBuild) // ビルドトリガーエンドポイント
	apiGroup.DELETE("/builds/:id", opts.BuildHandler.CancelBuild)           // ビルドキャンセルエンドポイント
	apiGroup.GET("/builds/:id/logs", opts.BuildHandler.GetBuildLogs)        // ビルドログ取得エンドポイント

	// ingress-route エンドポイントを登録する
	apiGroup.GET("/deployments/:id/ingress-route", opts.DeploymentHandler.GetIngressRoute)        // ingress-route 設定取得エンドポイント
	apiGroup.POST("/deployments/:id/ingress-route", opts.DeploymentHandler.CreateIngressRoute)    // ingress-route 作成エンドポイント
	apiGroup.PUT("/deployments/:id/ingress-route", opts.DeploymentHandler.UpdateIngressRoute)     // ingress-route 設定更新エンドポイント

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

	return router
}
