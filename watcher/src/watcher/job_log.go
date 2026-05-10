package watcher

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"launchs/shared/model"

	"github.com/redis/go-redis/v9"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// streamJobLogs は Job に紐づく Pod の全コンテナ（InitContainer 含む）のログを
// 順番にストリームします。InitContainer → 通常 Container の順で処理されます。
func streamJobLogs(ctx context.Context, clientset *kubernetes.Clientset, redisClient *redis.Client, namespace, jobName, buildJobID, redisChannel string) {
	// Pod が起動するまでポーリングして待機
	pod, err := waitForJobPod(ctx, clientset, namespace, jobName)
	if err != nil {
		fmt.Printf("[job-watcher] pod wait error for %s: %v\n", jobName, err)
		return
	}

	// InitContainer → Container の順にコンテナ名を列挙
	containers := collectContainerNames(pod)

	for _, containerName := range containers {
		// コンテナが Running または Terminated になるまで待機
		if err := waitForContainerRunning(ctx, clientset, namespace, pod.Name, containerName); err != nil {
			if ctx.Err() != nil {
				return // コンテキストキャンセル時はループを抜ける
			}
			continue
		}
		streamContainerLogs(ctx, clientset, redisClient, namespace, pod.Name, containerName, buildJobID, redisChannel)
	}
}

// collectContainerNames は Pod から InitContainer → Container の順でコンテナ名を返します。
func collectContainerNames(pod *corev1.Pod) []string {
	names := make([]string, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers))
	for _, c := range pod.Spec.InitContainers {
		names = append(names, c.Name)
	}
	for _, c := range pod.Spec.Containers {
		names = append(names, c.Name)
	}
	return names
}

// streamContainerLogs は 1 つのコンテナのログを最後まで読み込み、
// 各行を DB に保存しつつ Redis へリアルタイムに Publish します。
func streamContainerLogs(ctx context.Context, clientset *kubernetes.Clientset, redisClient *redis.Client, namespace, podName, containerName, buildJobID, redisChannel string) {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     true,  // ログを追従して読み続ける
		Timestamps: true,  // RFC3339 タイムスタンプを先頭に付与
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		// タイムスタンプ部分を除去してメッセージだけを取得
		_, message := parseTimestampedLine(line)
		logLine := fmt.Sprintf("[%s] %s\n", containerName, message)

		// ビルドログとして DB に追記
		model.AppendBuildLog(buildJobID, []byte(logLine))

		// フロントエンドにリアルタイム配信
		publishJobLog(ctx, redisClient, redisChannel, containerName, message)
	}
}

// publishJobLog は 1 行分のログを JSON にシリアライズして Redis チャンネルに Publish します。
func publishJobLog(ctx context.Context, redisClient *redis.Client, channel, containerName, message string) {
	msg := JobLogMessage{
		Container: containerName,
		Message:   message,
		Timestamp: time.Now(),
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	redisClient.Publish(ctx, channel, payload)
}

// waitForJobPod は Job に紐づく Pod が Running/Succeeded/Failed になるまで
// 最大 10 分間ポーリングします。
func waitForJobPod(ctx context.Context, clientset *kubernetes.Clientset, namespace, jobName string) (*corev1.Pod, error) {
	deadline := time.Now().Add(10 * time.Minute)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for pod")
		}

		// 先に待機してから取得（初回のすぐ存在しないケースを考慮）
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}

		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", jobName),
		})
		if err != nil {
			continue
		}

		for i := range pods.Items {
			p := &pods.Items[i]
			phase := p.Status.Phase
			if phase == corev1.PodRunning || phase == corev1.PodSucceeded || phase == corev1.PodFailed {
				return p, nil
			}
		}
	}
}

// waitForContainerRunning は指定コンテナが Running または Terminated になるまで
// 1 秒ごとにポーリングします。
func waitForContainerRunning(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, containerName string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}

		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		// InitContainer と通常 Container を合わせて確認
		all := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
		for _, cs := range all {
			if cs.Name == containerName && (cs.State.Running != nil || cs.State.Terminated != nil) {
				return nil
			}
		}
	}
}
