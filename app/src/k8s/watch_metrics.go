package k8s

import (
	"app/logger"
	"app/repository"
	"context"
	"time"

	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned"
)

// PollMetrics は 30 秒ごとに running 状態の全 Deployment のメトリクスを収集して DB に保存する
func PollMetrics(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	metricsClient metricsv1beta1.Interface,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
	metricsRepo repository.DeploymentMetricsRepository,
) {
	logger.Println("メトリクスポーリングを開始します") // 起動ログを出す

	ticker := time.NewTicker(30 * time.Second) // 30 秒ごとにポーリングするタイマーを生成する
	defer ticker.Stop()                        // 関数終了時にタイマーを停止する

	for {
		select {
		case <-ctx.Done(): // コンテキストがキャンセルされた場合
			logger.Println("メトリクスポーリングを停止します") // 停止ログを出す
			return
		case <-ticker.C: // 30 秒ごとに実行する
			collectAllMetrics(ctx, k8sClient, metricsClient, deploymentRepo, projectRepo, metricsRepo) // メトリクスを収集する
		}
	}
}

// collectAllMetrics は running 状態の全 Deployment のメトリクスを収集・保存・クリーンアップする
func collectAllMetrics(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	metricsClient metricsv1beta1.Interface,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
	metricsRepo repository.DeploymentMetricsRepository,
) {
	// running 状態の全 Deployment を取得する
	runningDeploymentList, err := deploymentRepo.FindAllRunning(ctx) // status=running の Deployment を全件取得する
	if err != nil {
		logger.PrintErr("メトリクス収集: running Deployment の取得に失敗しました: " + err.Error()) // エラーをログ出力する
		return
	}

	// 各 Deployment のメトリクスを収集して保存する
	for deploymentIndex := range runningDeploymentList { // 各 Deployment を処理する
		deploymentData := runningDeploymentList[deploymentIndex] // ループ変数のコピーを取る

		// プロジェクトを取得して Namespace を解決する
		projectData, projectErr := projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // プロジェクトを取得する
		if projectErr != nil {
			logger.PrintErr("メトリクス収集: namespace 解決に失敗しました（deploymentID=" + deploymentData.ID + "）: " + projectErr.Error()) // エラーをログ出力する
			continue // 他の Deployment の処理を継続する
		}

		// Metrics Server から Pod メトリクスを取得する
		metricsList, metricsErr := CollectDeploymentMetrics(
			ctx,
			k8sClient,
			metricsClient,
			projectData.Namespace, // Namespace を渡す
			deploymentData.ID,     // Deployment ID を渡す
			deploymentData.Name,   // Deployment 名を渡す
		)
		if metricsErr != nil {
			logger.PrintErr("メトリクス収集: メトリクスの取得に失敗しました（deploymentID=" + deploymentData.ID + "）: " + metricsErr.Error()) // エラーをログ出力する
			continue // 他の Deployment の処理を継続する
		}

		if len(metricsList) == 0 { // メトリクスが取得できなかった場合はスキップする
			continue
		}

		// メトリクスを DB に保存する
		if saveErr := metricsRepo.CreateBatch(ctx, metricsList); saveErr != nil { // メトリクスを一括保存する
			logger.PrintErr("メトリクス収集: DB への保存に失敗しました（deploymentID=" + deploymentData.ID + "）: " + saveErr.Error()) // エラーをログ出力する
		}
	}

	// 7 日以上古いメトリクスを削除する
	retentionLimit := time.Now().AddDate(0, 0, -7)                       // 7 日前の日時を計算する
	if deleteErr := metricsRepo.DeleteOlderThan(ctx, retentionLimit); deleteErr != nil { // 古いメトリクスを削除する
		logger.PrintErr("メトリクス収集: 古いメトリクスの削除に失敗しました: " + deleteErr.Error()) // エラーをログ出力する
	}
}
