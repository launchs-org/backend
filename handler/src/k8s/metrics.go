package k8s

import (
	"handler/models"
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned"
)

// CollectDeploymentMetrics は指定された Deployment の全 Pod のメトリクスを Metrics Server から取得して返す
func CollectDeploymentMetrics(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	metricsClient metricsv1beta1.Interface,
	namespace string,
	deploymentID string,
	deploymentName string,
) ([]*models.DeploymentMetrics, error) {
	// k8s Deployment を取得してレプリカ数を確認する
	k8sDeployment, err := k8sClient.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{}) // k8s から Deployment を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	readyReplicas := k8sDeployment.Status.ReadyReplicas // Ready 状態のレプリカ数を取得する
	totalReplicas := k8sDeployment.Status.Replicas      // 合計レプリカ数を取得する

	// Deployment のラベルセレクタを使って対象 Pod を特定する
	labelSelector := ""                                                            // ラベルセレクタを初期化する
	if k8sDeployment.Spec.Selector != nil && len(k8sDeployment.Spec.Selector.MatchLabels) > 0 { // ラベルセレクタが存在する場合
		for labelKey, labelValue := range k8sDeployment.Spec.Selector.MatchLabels { // ラベルセレクタを構築する
			if labelSelector != "" {
				labelSelector += "," // 複数のラベルをカンマで区切る
			}
			labelSelector += labelKey + "=" + labelValue // ラベルセレクタを追加する
		}
	}

	// Metrics Server から Pod のメトリクスを取得する
	podMetricsList, err := metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector, // ラベルセレクタで絞り込む
	})
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	recordedAt := time.Now() // 記録日時を設定する

	// 取得した Pod メトリクスを DeploymentMetrics スライスに変換する
	metricsList := make([]*models.DeploymentMetrics, 0, len(podMetricsList.Items)) // 結果スライスを初期化する
	for _, podMetrics := range podMetricsList.Items {                              // 各 Pod のメトリクスを処理する
		cpuMillicores := int64(0)  // CPU 使用量を初期化する
		memoryBytes := int64(0)    // メモリ使用量を初期化する

		for _, containerMetrics := range podMetrics.Containers { // コンテナごとのメトリクスを集計する
			cpuMillicores += containerMetrics.Usage.Cpu().MilliValue()    // CPU 使用量を加算する（ミリコア単位）
			memoryBytes += containerMetrics.Usage.Memory().Value()        // メモリ使用量を加算する（バイト単位）
		}

		metricsRecord := &models.DeploymentMetrics{
			DeploymentID:  deploymentID,  // Deployment ID を設定する
			PodName:       podMetrics.Name, // Pod 名を設定する
			CPUMillicores: cpuMillicores, // CPU 使用量を設定する
			MemoryBytes:   memoryBytes,   // メモリ使用量を設定する
			ReadyReplicas: readyReplicas, // Ready レプリカ数を設定する
			TotalReplicas: totalReplicas, // 合計レプリカ数を設定する
			RecordedAt:    recordedAt,    // 記録日時を設定する
		}
		metricsList = append(metricsList, metricsRecord) // スライスに追加する
	}

	return metricsList, nil // メトリクス一覧を返す
}
