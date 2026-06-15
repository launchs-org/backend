package main

import (
	"app/config"
	"app/handler"
	"app/k8s"
	"app/leader"
	"app/middlewares"
	"app/repository"
	"app/router"
	"app/service"
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	// 設定を初期化する
	cfg := config.NewEnvConfig()

	// ミドルウェア初期化（JWT公開鍵の読み込み）
	middlewares.Init()

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
	userQuotaRepo := repository.NewUserQuotaRepository(repository.Database) // quota リポジトリを生成する
	quotaServiceImpl := service.NewQuotaService(userQuotaRepo)              // quota サービスを生成する
	userQuotaHandler := handler.NewUserQuotaHandler(quotaServiceImpl)       // quota ハンドラーを生成する

	// Harbor クライアントを初期化する
	harborClient := k8s.NewHarborClient(
		cfg.GetHarborEndpoint(),    // Harbor エンドポイントを設定する
		cfg.GetHarborRobotName(),   // 管理用 robot アカウント名を設定する
		cfg.GetHarborRobotSecret(), // 管理用 robot アカウントのシークレットを設定する
	)

	// リポジトリを生成する（project・deployment ハンドラーで共有する）
	projectRepo := repository.NewProjectRepository(repository.Database)             // project リポジトリを生成する
	harborCredentialRepo := repository.NewHarborCredentialRepository(repository.Database) // harbor credential リポジトリを生成する
	deploymentRepo := repository.NewDeploymentRepository(repository.Database)       // deployment リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(repository.Database)   // ingress_route リポジトリを生成する

	// project ハンドラーを DI 組み立てする
	projectServiceImpl := service.NewProjectService(repository.Database, projectRepo, harborCredentialRepo, deploymentRepo, ingressRouteRepo, k8sClient, dynamicClient, harborClient) // project サービスを生成する
	projectHandler := handler.NewProjectHandler(projectServiceImpl)                 // project ハンドラーを生成する

	// deployment ハンドラーを DI 組み立てする
	serviceRepo := repository.NewServiceRepository(repository.Database)                                                    // service リポジトリを生成する
	envVarRepo := repository.NewEnvVarRepository(repository.Database)                                                      // env_var リポジトリを生成する
	envVarMountRepo := repository.NewEnvVarMountRepository(repository.Database)                                            // env_var_mount リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(repository.Database)                                          // apply_history リポジトリを生成する
	volumeRepo := repository.NewVolumeRepository(repository.Database)                                                      // volume リポジトリを生成する
	volumeMountRepo := repository.NewVolumeMountRepository(repository.Database)                                            // volume_mount リポジトリを生成する
	buildRepo := repository.NewDeploymentBuildRepository(repository.Database)                                              // build リポジトリを生成する
	deploymentServiceImpl := service.NewDeploymentService(deploymentRepo, serviceRepo, projectRepo, ingressRouteRepo, envVarMountRepo, volumeMountRepo, k8sClient, dynamicClient) // deployment サービスを生成する
	applyServiceImpl := service.NewApplyService(repository.Database, k8sClient, dynamicClient, deploymentRepo, applyHistoryRepo, projectRepo, serviceRepo, ingressRouteRepo, envVarRepo, envVarMountRepo, volumeRepo, volumeMountRepo) // apply サービスを生成する
	deploymentHandler := handler.NewDeploymentHandler(deploymentServiceImpl, applyServiceImpl)                             // deployment ハンドラーを生成する

	// build ハンドラーを DI 組み立てする
	logChunkRepo := repository.NewBuildLogChunkRepository(repository.Database)                                                                           // build ログチャンクリポジトリを生成する
	buildServiceImpl := service.NewBuildService(deploymentRepo, buildRepo, projectRepo, harborCredentialRepo, logChunkRepo, k8sClient)                  // build サービスを生成する
	buildHandler := handler.NewBuildHandler(buildServiceImpl)                                                                                            // build ハンドラーを生成する

	// env_var ハンドラーを DI 組み立てする
	envVarServiceImpl := service.NewEnvVarService(repository.Database, envVarRepo, projectRepo)                                                // env_var サービスを生成する
	envVarMountServiceImpl := service.NewEnvVarMountService(repository.Database, envVarMountRepo, deploymentRepo, projectRepo)                 // env_var_mount サービスを生成する
	envVarHandler := handler.NewEnvVarHandler(envVarServiceImpl, envVarMountServiceImpl)                                                       // env_var ハンドラーを生成する

	// volume ハンドラーを DI 組み立てする
	volumeServiceImpl := service.NewVolumeService(repository.Database, volumeRepo, volumeMountRepo, deploymentRepo, projectRepo)  // volume サービスを生成する
	volumeHandler := handler.NewVolumeHandler(volumeServiceImpl)                                                                  // volume ハンドラーを生成する

	// webhook ハンドラーを DI 組み立てする
	webhookRepo := repository.NewWebhookRepository(repository.Database)                                                                              // webhook リポジトリを生成する
	webhookServiceImpl := service.NewWebhookService(webhookRepo, deploymentRepo, projectRepo, applyServiceImpl)                                       // webhook サービスを生成する
	webhookHandler := handler.NewWebhookHandler(webhookServiceImpl)                                                                                  // webhook ハンドラーを生成する

	// 各 Watcher をリーダーエレクション経由でバックグラウンドで起動する
	go leader.RunAsLeader(context.Background(), repository.Database, func(ctx context.Context) { // リーダーになった Pod のみ Watcher を起動する
		go k8s.WatchServices(ctx, k8sClient, serviceRepo)                               // k8s Service の状態変化を監視して DB を自動更新する
		go k8s.WatchIngressRoutes(ctx, dynamicClient, ingressRouteRepo)                 // Traefik IngressRoute の状態変化を監視して DB を自動更新する
		go k8s.WatchPVCs(ctx, k8sClient, volumeRepo)                                    // PVC の Bound 状態を監視して DB を自動更新する
		go k8s.WatchNamespaces(ctx, k8sClient, projectRepo)                             // Namespace の削除イベントを監視して DB の Project レコードを削除する
		go k8s.WatchBuildJobs(ctx, k8sClient, buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo) // Build Job の完了・失敗を監視して DB を自動更新する
		k8s.WatchDeployments(ctx, k8sClient, deploymentRepo, envVarMountRepo, volumeMountRepo, applyHistoryRepo, buildRepo) // k8s Deployment の状態変化を監視して DB を自動更新する
	})

	// ルーターを生成してサーバーを起動する
	echoRouter := router.New(router.RouterOptions{
		UserQuotaHandler:  userQuotaHandler,  // quota ハンドラーを注入する
		ProjectHandler:    projectHandler,    // project ハンドラーを注入する
		DeploymentHandler: deploymentHandler, // deployment ハンドラーを注入する
		EnvVarHandler:     envVarHandler,     // env_var ハンドラーを注入する
		VolumeHandler:     volumeHandler,     // volume ハンドラーを注入する
		BuildHandler:      buildHandler,      // build ハンドラーを注入する
		WebhookHandler:    webhookHandler,    // webhook ハンドラーを注入する
	})
	if err := echoRouter.Start(":" + cfg.GetServerPort()); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("サーバーの起動に失敗しました", "error", err) // サーバー起動失敗時にエラーログを出す
	}
}
