package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"

	"handler/config"
	"handler/fileio"
	"handler/handler"
	"handler/k8s"
	"handler/middlewares"
	"handler/repository"
	"handler/router"
	"handler/service"

	temporalclient "go.temporal.io/sdk/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	// 設定を初期化する
	cfg := config.NewEnvConfig()

	// ミドルウェア初期化（JWT公開鍵の読み込み）
	middlewares.Init()

	// CLIトークン用の鍵ペアの読み込み
	middlewares.InitCliToken()

	// アーカイブアップロードトークン用の共有シークレットの読み込み
	middlewares.InitArchiveUploadToken()

	// データベース初期化・マイグレーション
	err := repository.Init()
	if err != nil {
		log.Fatalf("データベースの初期化に失敗しました: %v", err) // DB 初期化失敗時はアプリを終了する
	}

	// k8s クライアント初期化
	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("k8s クライアントの作成に失敗しました: %v", err) // kubeconfig が存在しない場合などにエラーを出す
	}

	// dynamic クライアント初期化（Traefik CRD 用）
	dynamicClient, err := k8s.NewDynamicClient()
	if err != nil {
		log.Fatalf("dynamic クライアントの作成に失敗しました: %v", err) // dynamic クライアント作成失敗時にエラーを出す
	}

	// namespace 一覧を取得して k8s 接続確認を行う
	namespaceList, err := k8sClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("k8s クラスターへの接続に失敗しました: %v", err) // クラスター疎通に失敗した場合エラーを出す
	}
	log.Printf("k8s に接続しました: %d 個の namespace が見つかりました", len(namespaceList.Items)) // 接続確認ログを出す

	// quota ハンドラーを DI 組み立てする
	userQuotaRepo := repository.NewUserQuotaRepository(repository.Database)           // quota リポジトリを生成する
	quotaServiceImpl := service.NewQuotaService(userQuotaRepo, repository.FreePlanID) // quota サービスを生成する（free プラン ID を注入する）
	userQuotaHandler := handler.NewUserQuotaHandler(quotaServiceImpl)                 // quota ハンドラーを生成する

	// Harbor クライアントを初期化する
	harborClient := k8s.NewHarborClient(
		cfg.GetHarborEndpoint(),    // Harbor エンドポイントを設定する
		cfg.GetHarborRobotName(),   // 管理用 robot アカウント名を設定する
		cfg.GetHarborRobotSecret(), // 管理用 robot アカウントのシークレットを設定する
	)

	// Temporal クライアントを初期化する（Apply/Project/Deployment/Volume Workflow を起動するために使用する）
	temporalHost := os.Getenv("TEMPORAL_HOST") // Temporal ホストを環境変数から取得する
	if temporalHost == "" {
		temporalHost = "localhost:7233" // デフォルトのホストを設定する
	}
	temporalClient, temporalErr := temporalclient.Dial(temporalclient.Options{
		HostPort: temporalHost, // Temporal サーバーのホスト:ポートを設定する
	})
	if temporalErr != nil {
		log.Fatalf("Temporal クライアントの作成に失敗しました: %v", temporalErr) // Temporal 接続失敗時はアプリを終了する
	}
	defer temporalClient.Close() // 終了時にクライアントを閉じる

	// リポジトリを生成する（project・deployment ハンドラーで共有する）
	projectRepo := repository.NewProjectRepository(repository.Database)                   // project リポジトリを生成する
	harborCredentialRepo := repository.NewHarborCredentialRepository(repository.Database) // harbor credential リポジトリを生成する
	deploymentRepo := repository.NewDeploymentRepository(repository.Database)             // deployment リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(repository.Database)         // ingress_route リポジトリを生成する
	buildRepo := repository.NewDeploymentBuildRepository(repository.Database)             // build リポジトリを生成する

	// project ハンドラーを DI 組み立てする
	projectServiceImpl := service.NewProjectService(repository.Database, projectRepo, harborCredentialRepo, deploymentRepo, buildRepo, ingressRouteRepo, userQuotaRepo, k8sClient, dynamicClient, harborClient, cfg.GetHarborStorageLimitBytes(), temporalClient) // project サービスを生成する
	projectHandler := handler.NewProjectHandler(projectServiceImpl)                                                                                                                                                                                               // project ハンドラーを生成する

	// deployment ハンドラーを DI 組み立てする
	serviceRepo := repository.NewServiceRepository(repository.Database)                       // service リポジトリを生成する
	envVarRepo := repository.NewEnvVarRepository(repository.Database)                         // env_var リポジトリを生成する
	envVarMountRepo := repository.NewEnvVarMountRepository(repository.Database)               // env_var_mount リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(repository.Database)             // apply_history リポジトリを生成する
	applyProgressRepo := repository.NewDeploymentApplyProgressRepository(repository.Database) // deployment_apply_progress リポジトリを生成する
	volumeRepo := repository.NewVolumeRepository(repository.Database)                         // volume リポジトリを生成する
	volumeMountRepo := repository.NewVolumeMountRepository(repository.Database)               // volume_mount リポジトリを生成する
	baseDomain := os.Getenv("BASE_DOMAIN")                                                    // ベースドメインを環境変数から取得する
	pathRuleRepo := repository.NewPathRuleRepository(repository.Database)                     // path_rule リポジトリを生成する

	imageRepo := repository.NewImageRepository(repository.Database) // image リポジトリを生成する

	deploymentServiceImpl := service.NewDeploymentService(deploymentRepo, serviceRepo, projectRepo, envVarRepo, envVarMountRepo, volumeMountRepo, applyHistoryRepo, applyProgressRepo, buildRepo, imageRepo, userQuotaRepo, k8sClient, temporalClient) // deployment サービスを生成する
	applyServiceImpl := service.NewApplyService(repository.Database, k8sClient, dynamicClient, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, pathRuleRepo, userQuotaRepo, temporalClient, baseDomain)                  // apply サービスを生成する
	deploymentHandler := handler.NewDeploymentHandler(deploymentServiceImpl, applyServiceImpl)                                                                                                                                                         // deployment ハンドラーを生成する

	// ingress_route ハンドラーを DI 組み立てする
	ingressRouteServiceImpl := service.NewIngressRouteService(ingressRouteRepo, pathRuleRepo, projectRepo, baseDomain) // ingress_route サービスを生成する
	ingressRouteHandler := handler.NewIngressRouteHandler(ingressRouteServiceImpl, applyServiceImpl)                   // ingress_route ハンドラーを生成する（apply サービスも注入する）

	// build ハンドラーを DI 組み立てする
	logChunkRepo := repository.NewBuildLogChunkRepository(repository.Database)                                                                                                             // build ログチャンクリポジトリを生成する
	fileIOClient := fileio.NewUploaderFromEnv()                                                                                                                                            // ARCHIVE_STORAGE_MODEに応じてアップロードクライアントを生成する
	buildServiceImpl := service.NewBuildService(deploymentRepo, buildRepo, projectRepo, harborCredentialRepo, logChunkRepo, k8sClient, "harbor.main-harbor", temporalClient, fileIOClient) // build サービスを生成する（Workflow は builder Worker に委譲する）
	buildHandler := handler.NewBuildHandler(buildServiceImpl)                                                                                                                              // build ハンドラーを生成する

	// image ハンドラーを DI 組み立てする
	imageServiceImpl := service.NewImageService(imageRepo, deploymentRepo, projectRepo, harborCredentialRepo, buildRepo, harborClient) // image サービスを生成する
	imageHandler := handler.NewImageHandler(imageServiceImpl)                                                                          // image ハンドラーを生成する

	// log ハンドラーを DI 組み立てする
	podLogChunkRepo := repository.NewPodLogChunkRepository(repository.Database)                      // pod ログチャンクリポジトリを生成する
	logServiceImpl := service.NewLogService(deploymentRepo, projectRepo, podLogChunkRepo, k8sClient) // log サービスを生成する
	logHandler := handler.NewLogHandler(logServiceImpl)                                              // log ハンドラーを生成する

	// env_var ハンドラーを DI 組み立てする
	envVarServiceImpl := service.NewEnvVarService(repository.Database, envVarRepo, projectRepo)                                // env_var サービスを生成する
	envVarMountServiceImpl := service.NewEnvVarMountService(repository.Database, envVarMountRepo, deploymentRepo, projectRepo) // env_var_mount サービスを生成する
	envVarHandler := handler.NewEnvVarHandler(envVarServiceImpl, envVarMountServiceImpl)                                       // env_var ハンドラーを生成する

	// volume ハンドラーを DI 組み立てする
	volumeServiceImpl := service.NewVolumeService(repository.Database, volumeRepo, volumeMountRepo, deploymentRepo, projectRepo, userQuotaRepo, k8sClient, cfg.GetStorageClassName(), temporalClient) // volume サービスを生成する
	volumeHandler := handler.NewVolumeHandler(volumeServiceImpl)                                                                                                                                      // volume ハンドラーを生成する

	// webhook ハンドラーを DI 組み立てする
	webhookRepo := repository.NewWebhookRepository(repository.Database)                                                                      // webhook リポジトリを生成する
	webhookServiceImpl := service.NewWebhookService(webhookRepo, deploymentRepo, projectRepo, imageRepo, applyServiceImpl, buildServiceImpl) // webhook サービスを生成する
	webhookHandler := handler.NewWebhookHandler(webhookServiceImpl)                                                                          // webhook ハンドラーを生成する

	// deployment-template ハンドラーを DI 組み立てする
	deploymentTemplateRepo := repository.NewDeploymentTemplateRepository(repository.Database)                                                                                                                                                        // deployment template リポジトリを生成する
	deploymentTemplateServiceImpl := service.NewDeploymentTemplateService(repository.Database, deploymentTemplateRepo, deploymentRepo, serviceRepo, envVarRepo, envVarMountRepo, volumeRepo, volumeMountRepo, projectRepo, imageRepo, userQuotaRepo) // deployment template サービスを生成する
	deploymentTemplateHandler := handler.NewDeploymentTemplateHandler(deploymentTemplateServiceImpl)                                                                                                                                                 // deployment template ハンドラーを生成する

	// metrics ハンドラーを DI 組み立てする
	deploymentMetricsRepo := repository.NewDeploymentMetricsRepository(repository.Database)             // deployment metrics リポジトリを生成する
	metricsServiceImpl := service.NewMetricsService(deploymentMetricsRepo, deploymentRepo, projectRepo) // metrics サービスを生成する
	metricsHandler := handler.NewMetricsHandler(metricsServiceImpl)                                     // metrics ハンドラーを生成する

	// cli-token ハンドラーを DI 組み立てする
	cliTokenRepo := repository.NewCliTokenRepository(repository.Database) // cli_token リポジトリを生成する
	middlewares.SetCliTokenRepository(cliTokenRepo)                       // RequireAuth が使う照合用リポジトリを設定する
	cliTokenServiceImpl := service.NewCliTokenService(cliTokenRepo)       // cli_token サービスを生成する
	cliTokenHandler := handler.NewCliTokenHandler(cliTokenServiceImpl)    // cli_token ハンドラーを生成する

	// ルーターを生成してサーバーを起動する
	echoRouter := router.New(router.RouterOptions{
		UserQuotaHandler:          userQuotaHandler,          // quota ハンドラーを注入する
		ProjectHandler:            projectHandler,            // project ハンドラーを注入する
		DeploymentHandler:         deploymentHandler,         // deployment ハンドラーを注入する
		DeploymentTemplateHandler: deploymentTemplateHandler, // deployment template ハンドラーを注入する
		IngressRouteHandler:       ingressRouteHandler,       // ingress_route ハンドラーを注入する
		EnvVarHandler:             envVarHandler,             // env_var ハンドラーを注入する
		VolumeHandler:             volumeHandler,             // volume ハンドラーを注入する
		BuildHandler:              buildHandler,              // build ハンドラーを注入する
		ImageHandler:              imageHandler,              // image ハンドラーを注入する
		WebhookHandler:            webhookHandler,            // webhook ハンドラーを注入する
		LogHandler:                logHandler,                // log ハンドラーを注入する
		MetricsHandler:            metricsHandler,            // metrics ハンドラーを注入する
		CliTokenHandler:           cliTokenHandler,           // cli-token ハンドラーを注入する
	})
	if err := echoRouter.Start(":" + cfg.GetServerPort()); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("サーバーの起動に失敗しました", "error", err) // サーバー起動失敗時にエラーログを出す
	}
}
