package main

import (
	"app/shared/repository"
	"context"
	"log"
	"watcher/k8s"
	"watcher/leader"
)

func main() {
	log.Println("watcher を起動します") // 起動ログを出力する

	// データベース初期化・マイグレーション
	err := repository.Init() // DB 初期化を実行する
	if err != nil {
		log.Fatalf("データベースの初期化に失敗しました: %v", err) // DB 初期化失敗時はアプリを終了する
	}

	// k8s クライアント初期化
	k8sClient, err := k8s.NewClient() // k8s クライアントを生成する
	if err != nil {
		log.Fatalf("k8s クライアントの作成に失敗しました: %v", err) // kubeconfig が存在しない場合などにエラーを出す
	}

	// dynamic クライアント初期化（Traefik CRD 用）
	dynamicClient, err := k8s.NewDynamicClient() // dynamic クライアントを生成する
	if err != nil {
		log.Fatalf("dynamic クライアントの作成に失敗しました: %v", err) // dynamic クライアント作成失敗時にエラーを出す
	}

	// Metrics Server クライアント初期化
	metricsClient, err := k8s.NewMetricsClient() // Metrics Server クライアントを生成する
	if err != nil {
		log.Fatalf("Metrics Server クライアントの作成に失敗しました: %v", err) // Metrics クライアント作成失敗時にエラーを出す
	}

	// リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(repository.Database)                           // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(repository.Database)                 // ingress_route リポジトリを生成する
	volumeRepo := repository.NewVolumeRepository(repository.Database)                             // volume リポジトリを生成する
	projectRepo := repository.NewProjectRepository(repository.Database)                           // project リポジトリを生成する
	deploymentRepo := repository.NewDeploymentRepository(repository.Database)                     // deployment リポジトリを生成する
	buildRepo := repository.NewDeploymentBuildRepository(repository.Database)                     // build リポジトリを生成する
	logChunkRepo := repository.NewBuildLogChunkRepository(repository.Database)                    // build ログチャンクリポジトリを生成する
	harborCredentialRepo := repository.NewHarborCredentialRepository(repository.Database)         // harbor credential リポジトリを生成する
	envVarMountRepo := repository.NewEnvVarMountRepository(repository.Database)                   // env_var_mount リポジトリを生成する
	volumeMountRepo := repository.NewVolumeMountRepository(repository.Database)                   // volume_mount リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(repository.Database)                 // apply_history リポジトリを生成する
	podLogChunkRepo := repository.NewPodLogChunkRepository(repository.Database)                   // pod ログチャンクリポジトリを生成する
	deploymentMetricsRepo := repository.NewDeploymentMetricsRepository(repository.Database)       // deployment metrics リポジトリを生成する
	imageRepo := repository.NewImageRepository(repository.Database)                               // image リポジトリを生成する

	// Harbor クライアントを初期化する（ビルドログ収集に使用）
	harborClient := k8s.NewHarborClient(
		"",  // Harbor エンドポイント（環境変数から取得する場合はここで設定する）
		"",  // 管理用 robot アカウント名
		"",  // 管理用 robot アカウントのシークレット
	) // Harbor クライアントを生成する

	// リーダーエレクション経由で各 Watcher を起動する
	leader.RunAsLeader(context.Background(), repository.Database, func(ctx context.Context) { // リーダーになったインスタンスのみ Watcher を起動する
		go k8s.WatchServices(ctx, k8sClient, serviceRepo)                                    // k8s Service の状態変化を監視して DB を自動更新する
		go k8s.WatchIngressRoutes(ctx, dynamicClient, ingressRouteRepo)                      // Traefik IngressRoute の状態変化を監視して DB を自動更新する
		go k8s.WatchPVCs(ctx, k8sClient, volumeRepo)                                         // PVC の Bound 状態を監視して DB を自動更新する
		go k8s.WatchNamespaces(ctx, k8sClient, projectRepo)                                  // Namespace の削除イベントを監視して DB の Project レコードを削除する
		go k8s.WatchBuildJobs(ctx, k8sClient, buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo, imageRepo, harborClient, "harbor.main-harbor") // Build Job の完了・失敗を監視して DB を自動更新する
		go k8s.PollMetrics(ctx, k8sClient, metricsClient, deploymentRepo, projectRepo, deploymentMetricsRepo) // 30 秒ごとに Pod メトリクスを収集して DB に保存する
		k8s.WatchDeployments(ctx, k8sClient, deploymentRepo, envVarMountRepo, volumeMountRepo, applyHistoryRepo, podLogChunkRepo, projectRepo) // k8s Deployment の状態変化を監視して DB を自動更新する
	})
}
