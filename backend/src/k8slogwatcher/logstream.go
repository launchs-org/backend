package k8slogwatcher

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// logStreamer は Deployment 配下の全 Pod のログをストリーミングします。
type logStreamer struct {
	k8sClient      *kubernetes.Clientset
	namespace      string
	deploymentName string
}

// newLogStreamer は新しい logStreamer を生成します。
func newLogStreamer(k8sClient *kubernetes.Clientset, namespace string, deploymentName string) *logStreamer {
	return &logStreamer{
		k8sClient:      k8sClient,
		namespace:      namespace,
		deploymentName: deploymentName,
	}
}

// streamAll は Deployment 配下の全 Pod のログをリアルタイムでストリーミングします。
// 新しい Pod が追加されてもポーリングにより自動検出します。
func (streamer *logStreamer) streamAll(ctx context.Context, sinceTime time.Time, output chan<- LogEntry) {
	// 現在実行中の Pod ストリームを追跡（Pod名 → cancel関数）
	activePods := make(map[string]context.CancelFunc)

	// Pod の追加・削除を検出するためのポーリングティッカー
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// 初回即時実行
	streamer.syncPodStreams(ctx, sinceTime, output, activePods)

	for {
		select {
		case <-ctx.Done():
			// コンテキストキャンセル時は全 Pod ストリームを停止
			for _, cancelPod := range activePods {
				cancelPod()
			}
			return
		case <-ticker.C:
			// 定期的に Pod リストを同期（スケールアウト/インへの対応）
			streamer.syncPodStreams(ctx, sinceTime, output, activePods)
		}
	}
}

// syncPodStreams は現在の Pod リストとアクティブなストリームを同期します。
// 新しい Pod にはストリームを開始し、削除された Pod のストリームを停止します。
func (streamer *logStreamer) syncPodStreams(
	ctx context.Context,
	sinceTime time.Time,
	output chan<- LogEntry,
	activePods map[string]context.CancelFunc,
) {
	// Deployment に紐づく Pod 一覧を取得
	pods, err := streamer.listDeploymentPods(ctx)
	if err != nil {
		return
	}

	// 現在の Pod 名のセットを作成
	currentPodNames := make(map[string]struct{}, len(pods))
	for _, pod := range pods {
		currentPodNames[pod.Name] = struct{}{}
	}

	// 削除された Pod のストリームを停止
	for podName, cancelPod := range activePods {
		if _, exists := currentPodNames[podName]; !exists {
			cancelPod()
			delete(activePods, podName)
		}
	}

	// 新しい Pod のストリームを開始
	for _, pod := range pods {
		if _, alreadyStreaming := activePods[pod.Name]; alreadyStreaming {
			continue
		}
		// Running 状態の Pod のみ対象
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		podCtx, cancelPod := context.WithCancel(ctx)
		activePods[pod.Name] = cancelPod

		// 各 Pod のログをゴルーチンで並列ストリーミング
		go streamer.streamPod(podCtx, pod, sinceTime, output)
	}
}

// streamPod は単一 Pod の全コンテナのログをストリーミングします。
func (streamer *logStreamer) streamPod(
	ctx context.Context,
	pod corev1.Pod,
	sinceTime time.Time,
	output chan<- LogEntry,
) {
	for _, container := range pod.Spec.Containers {
		// 各コンテナのログをゴルーチンで並列取得
		go streamer.streamContainer(ctx, pod.Name, container.Name, sinceTime, output)
	}
}

// streamContainer は単一コンテナのログをストリーミングします。
// ログの各行をパースして output チャンネルに送信します。
func (streamer *logStreamer) streamContainer(
	ctx context.Context,
	podName string,
	containerName string,
	sinceTime time.Time,
	output chan<- LogEntry,
) {
	// sinceTime をメタデータ型に変換
	metaSinceTime := metav1.NewTime(sinceTime)

	// ログストリームのオプション設定
	logOptions := &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     true,  // リアルタイムでストリーミング
		Timestamps: true,  // 各行にタイムスタンプを付与
		SinceTime:  &metaSinceTime,
	}

	// Kubernetes API からログストリームを取得
	req := streamer.k8sClient.CoreV1().Pods(streamer.namespace).GetLogs(podName, logOptions)
	stream, err := req.Stream(ctx)
	if err != nil {
		return
	}
	defer stream.Close()

	// 行ごとにログを読み取って output に送信
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		entry := parseLogLine(line, streamer.namespace, streamer.deploymentName, podName, containerName)

		select {
		case <-ctx.Done():
			return
		case output <- entry:
		}
	}
}

// listDeploymentPods は Deployment のラベルセレクターを使って Pod 一覧を取得します。
func (streamer *logStreamer) listDeploymentPods(ctx context.Context) ([]corev1.Pod, error) {
	// まず Deployment を取得してラベルセレクターを確認
	deployment, err := streamer.k8sClient.AppsV1().Deployments(streamer.namespace).Get(
		ctx, streamer.deploymentName, metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("deployment get failed: %w", err)
	}

	// ラベルセレクターを文字列に変換（例: "app=myapp,env=prod"）
	selector := metav1.FormatLabelSelector(deployment.Spec.Selector)

	// セレクターに一致する Pod 一覧を取得
	podList, err := streamer.k8sClient.CoreV1().Pods(streamer.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, fmt.Errorf("pod list failed: %w", err)
	}
	return podList.Items, nil
}

// parseLogLine はタイムスタンプ付きのログ行をパースして LogEntry を返します。
// Kubernetes のタイムスタンプ形式: "2006-01-02T15:04:05.000000000Z message text"
func parseLogLine(line, namespace, deploymentName, podName, containerName string) LogEntry {
	logTimestamp := time.Now() // パース失敗時のフォールバック
	message := line

	// スペースで分割してタイムスタンプとメッセージを分離
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 2 {
		if parsed, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
			logTimestamp = parsed
			message = parts[1]
		}
	}

	return LogEntry{
		Namespace:  namespace,
		Deployment: deploymentName,
		PodName:    podName,
		Container:  containerName,
		Message:    message,
		Timestamp:  logTimestamp,
	}
}
