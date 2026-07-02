package main

import (
	"app/shared/config"
	"app/shared/repository"
	"builder/activity"
	builderk8s "builder/k8s"
	"builder/workflow"
	"log"
	"os"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

const taskQueue = "builder-queue" // Temporal タスクキュー名

func main() {
	log.Println("builder worker を起動します") // 起動ログを出力する

	// 設定を初期化する
	cfg := config.NewEnvConfig() // 設定を読み込む

	// データベース初期化・マイグレーション
	if err := repository.Init(); err != nil { // DB 初期化を実行する
		log.Fatalf("データベースの初期化に失敗しました: %v", err) // DB 初期化失敗時はアプリを終了する
	}

	// k8s クライアント初期化
	k8sClient, err := builderk8s.NewClient() // k8s クライアントを生成する
	if err != nil {
		log.Fatalf("k8s クライアントの作成に失敗しました: %v", err) // kubeconfig が存在しない場合などにエラーを出す
	}

	// Harbor クライアント初期化（イメージサイズ取得用）
	harborEndpoint := cfg.GetHarborEndpoint()                              // Harbor エンドポイントを取得する
	harborClient := builderk8s.NewHarborClient(harborEndpoint) // Harbor クライアントを生成する

	// リポジトリを生成する
	deploymentRepo := repository.NewDeploymentRepository(repository.Database)             // deployment リポジトリを生成する
	projectRepo := repository.NewProjectRepository(repository.Database)                   // project リポジトリを生成する
	buildRepo := repository.NewDeploymentBuildRepository(repository.Database)             // build リポジトリを生成する
	harborCredentialRepo := repository.NewHarborCredentialRepository(repository.Database) // harbor credential リポジトリを生成する
	logChunkRepo := repository.NewBuildLogChunkRepository(repository.Database)            // ビルドログチャンクリポジトリを生成する

	// Activity を生成する
	buildActivities := &activity.BuildActivities{ // Build Activity を生成する
		K8sClient:            k8sClient,             // k8s クライアントを注入する
		BuildRepo:            buildRepo,             // build リポジトリを注入する
		DeploymentRepo:       deploymentRepo,        // deployment リポジトリを注入する
		ProjectRepo:          projectRepo,           // project リポジトリを注入する
		HarborCredentialRepo: harborCredentialRepo,  // harbor credential リポジトリを注入する
		LogChunkRepo:         logChunkRepo,          // ビルドログチャンクリポジトリを注入する
		RegistryHost:         cfg.GetRegistryHost(), // Harbor ホスト名を設定する
		HarborClient:         harborClient,          // Harbor クライアントを注入する
		HarborEndpoint:       harborEndpoint,        // Harbor エンドポイントを注入する
	}
	cancelBuildActivities := &activity.CancelBuildActivities{ // CancelBuild Activity を生成する
		K8sClient: k8sClient, // k8s クライアントを注入する
		BuildRepo: buildRepo, // build リポジトリを注入する
	}

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
	temporalWorker.RegisterWorkflow(workflow.BuildWorkflow)       // Build Workflow を登録する
	temporalWorker.RegisterWorkflow(workflow.CancelBuildWorkflow) // CancelBuild Workflow を登録する

	// Activity を登録する
	temporalWorker.RegisterActivity(buildActivities)       // Build Activity を登録する
	temporalWorker.RegisterActivity(cancelBuildActivities) // CancelBuild Activity を登録する

	log.Printf("Temporal Worker を起動します (taskQueue=%s, host=%s)", taskQueue, temporalHost) // 起動ログを出力する
	if err := temporalWorker.Run(worker.InterruptCh()); err != nil {                        // Worker を起動してシグナルで停止を待つ
		log.Fatalf("Temporal Worker の起動に失敗しました: %v", err) // 起動失敗時はアプリを終了する
	}
}
