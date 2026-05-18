package watcher

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"launchs/shared/database"
	"launchs/shared/model"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// WatchDeployments は launchs-managed=true ラベルを持つ全 Namespace の Deployment を監視し、
// Container.Status を同期します。エラー時は 3 秒待機して自動再起動します。
func WatchDeployments(ctx context.Context) {
	clientset := database.K8sClientset.(*kubernetes.Clientset)

	fmt.Println("[deploy-watcher] starting deployment watcher (label: launchs-managed=true)")

	// Terminating済みPodの取りこぼしを防ぐ定期同期
	go periodicPodSync(ctx, clientset)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := runDeploymentWatch(ctx, clientset); err != nil {
			fmt.Printf("[deploy-watcher] watch error: %v, restarting...\n", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

// periodicPodSync は10秒ごとに全Deploymentに対してPodステータスを同期します。
// スケールダウン後にDeploymentイベントが来なくてもTerminatedPodを削除します。
func periodicPodSync(ctx context.Context, clientset *kubernetes.Clientset) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deploys, err := clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{
				LabelSelector: "launchs-managed=true",
			})
			if err != nil {
				fmt.Printf("[deploy-watcher] periodic sync: failed to list deployments: %v\n", err)
				continue
			}
			for i := range deploys.Items {
				deploy := &deploys.Items[i]
				containerID, ok := deploy.Labels["container-id"]
				if !ok || containerID == "" {
					continue
				}
				syncPodStatuses(ctx, clientset, deploy, containerID)
			}
		}
	}
}

// runDeploymentWatch は K8s Deployment の Watch セッションを 1 つ実行します。
// "launchs-managed=true" ラベルで絞り込み、全 Namespace を対象にします。
func runDeploymentWatch(ctx context.Context, clientset *kubernetes.Clientset) error {
	// launchs-managed=true が付いた Deployment のみ Watch。
	// backend の DeployWorker が Deployment 作成時にこのラベルを付与する。
	watcher, err := clientset.AppsV1().Deployments("").Watch(ctx, metav1.ListOptions{
		LabelSelector: "launchs-managed=true",
	})
	if err != nil {
		return fmt.Errorf("failed to start deployment watch: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.ResultChan():
			// チャンネルが閉じられたら上位ループで再接続する
			if !ok {
				return fmt.Errorf("watch channel closed")
			}
			if err := handleDeploymentEvent(ctx, clientset, event); err != nil {
				fmt.Printf("[deploy-watcher] handle event error: %v\n", err)
			}
		}
	}
}

// handleDeploymentEvent は Deployment の Watch イベントを処理します。
// container-id ラベルで DB の Container レコードを特定し、ステータスを同期します。
func handleDeploymentEvent(ctx context.Context, clientset *kubernetes.Clientset, event watch.Event) error {
	deploy, ok := event.Object.(*appsv1.Deployment)
	if !ok {
		return nil
	}

	// container-id ラベルから DB の Container ID を取得
	// (ラベルがない古い Deployment は無視する)
	containerID, ok := deploy.Labels["container-id"]
	if !ok || containerID == "" {
		return nil
	}

	status := determineDeploymentStatus(deploy)

	// ステータス変化時のログ出力
	logDeploymentEvent(event.Type, deploy, containerID, status)

	// DB から Container を取得してステータスを比較
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		// 削除済みコンテナなどは無視
		return nil
	}

	// デプロイ中・再デプロイ中は Failed に上書きしない（Pod 起動過渡期の誤検知を防ぐ）
	transitioning := container.Status == "Deploying" || container.Status == "Redeploying"
	if transitioning && status == "Failed" {
		return nil
	}

	// ステータスが変化した場合のみ DB 更新・キャッシュ削除
	if container.Status != status {
		fmt.Printf("[deploy-watcher] status changed: container=%s %s → %s\n",
			containerID, container.Status, status)
		model.UpdateContainerStatus(containerID, status)
	}

	// Pod ステータスを常に同期する
	syncPodStatuses(ctx, clientset, deploy, containerID)

	// Running 状態になったら古いログをクリアして Pod ログのストリームを開始
	if status == "Running" {
		model.ClearContainerLog(containerID)
		go streamDeploymentPodLogs(ctx, clientset, deploy.Namespace, deploy.Name, containerID)
	}

	return nil
}

// syncPodStatuses は Deployment に紐づく Pod 一覧を取得して DB に upsert します。
func syncPodStatuses(ctx context.Context, clientset *kubernetes.Clientset, deploy *appsv1.Deployment, containerID string) {
	pods, err := clientset.CoreV1().Pods(deploy.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", deploy.Name),
	})
	if err != nil {
		fmt.Printf("[deploy-watcher] failed to list pods for %s: %v\n", deploy.Name, err)
		return
	}

	statuses := make([]model.PodStatus, 0, len(pods.Items))
	activePodIDs := make([]string, 0, len(pods.Items))

	for _, pod := range pods.Items {
		// Terminating中のPodは除外する（DeletionTimestampが設定されている）
		if pod.DeletionTimestamp != nil {
			continue
		}

		phase := string(pod.Status.Phase)
		ready := false
		var restarts int32
		var message string
		var startedAt *time.Time

		// コンテナステータスから Ready・Restarts・Message を取得
		for _, cs := range pod.Status.ContainerStatuses {
			restarts += cs.RestartCount
			if cs.Ready {
				ready = true
			}
			if cs.State.Waiting != nil && cs.State.Waiting.Message != "" {
				message = cs.State.Waiting.Message
			}
			if cs.State.Terminated != nil && cs.State.Terminated.Message != "" {
				message = cs.State.Terminated.Message
			}
		}

		if pod.Status.StartTime != nil {
			t := pod.Status.StartTime.Time
			startedAt = &t
		}

		now := time.Now()
		statuses = append(statuses, model.PodStatus{
			ID:          string(pod.UID),
			ContainerID: containerID,
			Name:        pod.Name,
			Phase:       phase,
			Ready:       ready,
			Restarts:    restarts,
			Message:     message,
			StartedAt:   startedAt,
			UpdatedAt:   now,
		})
		activePodIDs = append(activePodIDs, string(pod.UID))
	}

	// DBにコンテナが存在しない場合はupsertをスキップ（削除済みコンテナのPodイベント）
	if _, err := model.GetContainerByID(containerID); err != nil {
		return
	}

	if len(statuses) > 0 {
		if err := model.UpsertPodStatuses(statuses); err != nil {
			fmt.Printf("[deploy-watcher] failed to upsert pod statuses: %v\n", err)
		}
	}
	// 消えた Pod のレコードを削除
	if err := model.DeleteStalePodStatuses(containerID, activePodIDs); err != nil {
		fmt.Printf("[deploy-watcher] failed to delete stale pod statuses: %v\n", err)
	}
}

// determineDeploymentStatus は Deployment の spec/status から
// アプリケーション用のステータス文字列を返します。
// UnavailableReplicas > 0 だけでは Failed にせず、K8s が明示的に失敗 Condition を
// 付けた場合のみ Failed とします（Pod 起動過渡期の誤検知を防ぐため）。
func determineDeploymentStatus(deploy *appsv1.Deployment) string {
	if deploy.Spec.Replicas == nil {
		return "Stopped"
	}
	desired := *deploy.Spec.Replicas
	if desired == 0 {
		return "Stopped"
	}
	if deploy.Status.ReadyReplicas >= desired {
		return "Running"
	}
	// Conditions を確認して明示的な失敗のみ Failed とする
	for _, c := range deploy.Status.Conditions {
		if c.Type == appsv1.DeploymentProgressing && c.Reason == "ProgressDeadlineExceeded" {
			return "Failed"
		}
		if c.Type == appsv1.DeploymentReplicaFailure && c.Status == corev1.ConditionTrue {
			return "Failed"
		}
	}
	return "Deploying"
}

// streamDeploymentPodLogs は Deployment に紐づく Pod のログを DB に保存します。
// 既存の Pod すべてに対してゴルーチンを起動します。
func streamDeploymentPodLogs(ctx context.Context, clientset *kubernetes.Clientset, namespace, deploymentName, containerID string) {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", deploymentName),
	})
	if err != nil || len(pods.Items) == 0 {
		return
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		go streamPodContainerLogs(ctx, clientset, namespace, pod.Name, deploymentName, containerID)
	}
}

// streamPodContainerLogs は 1 つの Pod のコンテナログを末尾 100 行から追従して読み込み、DB に追記します。
func streamPodContainerLogs(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, containerName, containerID string) {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
		Follow:    true,
		TailLines: int64Ptr(100),
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
		model.AppendContainerLog(containerID, append([]byte(line), '\n'))
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}
