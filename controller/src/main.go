package main

import (
	"app/shared/config"
	"app/shared/repository"
	"controller/activity"
	"controller/k8s"
	"controller/workflow"
	"log"
	"os"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

const taskQueue = "controller-queue" // Temporal タスクキュー名

func main() {
	log.Println("controller worker を起動します") // 起動ログを出力する

	// 設定を初期化する
	cfg := config.NewEnvConfig() // 設定を読み込む

	// データベース初期化・マイグレーション
	if err := repository.Init(); err != nil { // DB 初期化を実行する
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

	// Harbor クライアントを初期化する
	harborClient := k8s.NewHarborClient(
		cfg.GetHarborEndpoint(),    // Harbor エンドポイントを設定する
		cfg.GetHarborRobotName(),   // 管理用 robot アカウント名を設定する
		cfg.GetHarborRobotSecret(), // 管理用 robot アカウントのシークレットを設定する
	)

	// リポジトリを生成する
	deploymentRepo := repository.NewDeploymentRepository(repository.Database)                     // deployment リポジトリを生成する
	applyHistoryRepo := repository.NewApplyHistoryRepository(repository.Database)                 // apply_history リポジトリを生成する
	projectRepo := repository.NewProjectRepository(repository.Database)                           // project リポジトリを生成する
	serviceRepo := repository.NewServiceRepository(repository.Database)                           // service リポジトリを生成する
	ingressRouteRepo := repository.NewIngressRouteRepository(repository.Database)                 // ingress_route リポジトリを生成する
	pathRuleRepo := repository.NewPathRuleRepository(repository.Database)                         // path_rule リポジトリを生成する
	envVarRepo := repository.NewEnvVarRepository(repository.Database)                             // env_var リポジトリを生成する
	envVarMountRepo := repository.NewEnvVarMountRepository(repository.Database)                   // env_var_mount リポジトリを生成する
	volumeRepo := repository.NewVolumeRepository(repository.Database)                             // volume リポジトリを生成する
	volumeMountRepo := repository.NewVolumeMountRepository(repository.Database)                   // volume_mount リポジトリを生成する
	harborCredentialRepo := repository.NewHarborCredentialRepository(repository.Database)         // harbor credential リポジトリを生成する
	imageRepo := repository.NewImageRepository(repository.Database)                               // image リポジトリを生成する

	// Activity を生成する
	applyActivities := activity.NewApplyActivities( // Apply Activity を生成する
		repository.Database,
		k8sClient,
		dynamicClient,
		deploymentRepo,
		applyHistoryRepo,
		projectRepo,
		serviceRepo,
		ingressRouteRepo,
		pathRuleRepo,
		envVarRepo,
		envVarMountRepo,
		volumeRepo,
		volumeMountRepo,
		imageRepo,
	)
	deploymentActivities := activity.NewDeploymentActivities( // Deployment Activity を生成する
		k8sClient,
		deploymentRepo,
		projectRepo,
		volumeMountRepo,
	)
	projectActivities := activity.NewProjectActivities( // Project Activity を生成する
		k8sClient,
		projectRepo,
		harborCredentialRepo,
		deploymentRepo,
		harborClient,
		cfg.GetHarborStorageLimitBytes(),
	)
	volumeActivities := activity.NewVolumeActivities( // Volume Activity を生成する
		k8sClient,
		volumeRepo,
		projectRepo,
		cfg.GetStorageClassName(),
	)

	// Temporal クライアントを生成する
	temporalHost := os.Getenv("TEMPORAL_HOST") // Temporal ホストを環境変数から取得する
	if temporalHost == "" {
		temporalHost = "localhost:7233" // デフォルトのホストを設定する
	}
	temporalClient, err := client.Dial(client.Options{
		HostPort: temporalHost, // Temporal サーバーのホスト:ポートを設定する
	})
	if err != nil {
		log.Fatalf("Temporal クライアントの作成に失敗しました: %v", err) // Temporal 接続失敗時はアプリを終了する
	}
	defer temporalClient.Close() // 終了時にクライアントを閉じる

	// Temporal Worker を生成して Workflow と Activity を登録する
	temporalWorker := worker.New(temporalClient, taskQueue, worker.Options{}) // Worker を生成する

	// Workflow を登録する
	temporalWorker.RegisterWorkflow(workflow.ApplyWorkflow)                     // Apply Workflow を登録する
	temporalWorker.RegisterWorkflow(workflow.DeleteDeploymentWorkflow)          // Deployment 削除 Workflow を登録する
	temporalWorker.RegisterWorkflow(workflow.CreateProjectWorkflow)             // Project 作成 Workflow を登録する
	temporalWorker.RegisterWorkflow(workflow.DeleteProjectWorkflow)             // Project 削除 Workflow を登録する
	temporalWorker.RegisterWorkflow(workflow.CreateVolumeWorkflow)              // Volume 作成 Workflow を登録する
	temporalWorker.RegisterWorkflow(workflow.DeleteVolumeWorkflow)              // Volume 削除 Workflow を登録する

	// Activity を登録する
	temporalWorker.RegisterActivity(applyActivities)        // Apply Activity を登録する
	temporalWorker.RegisterActivity(deploymentActivities)   // Deployment Activity を登録する
	temporalWorker.RegisterActivity(projectActivities)      // Project Activity を登録する
	temporalWorker.RegisterActivity(volumeActivities)       // Volume Activity を登録する

	log.Printf("Temporal Worker を起動します (taskQueue=%s, host=%s)", taskQueue, temporalHost) // 起動ログを出力する
	if err := temporalWorker.Run(worker.InterruptCh()); err != nil {                        // Worker を起動してシグナルで停止を待つ
		log.Fatalf("Temporal Worker の起動に失敗しました: %v", err) // 起動失敗時はアプリを終了する
	}
}
