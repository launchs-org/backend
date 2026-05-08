package watcher

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"launchs/shared/database"
	"launchs/shared/model"

	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// PodLogMessage は Redis に Publish するコンテナログの形式です
type PodLogMessage struct {
	PodName   string    `json:"pod_name"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// WatchDeployments は全 ns-* Namespace の Deployment を監視し、Container.Status を同期します
func WatchDeployments(ctx context.Context) {
	clientset := database.K8sClientset.(*kubernetes.Clientset)
	redisClient := database.RedisClient

	fmt.Println("[deploy-watcher] starting deployment watcher")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := runDeploymentWatch(ctx, clientset, redisClient); err != nil {
			fmt.Printf("[deploy-watcher] watch error: %v, restarting...\n", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func runDeploymentWatch(ctx context.Context, clientset *kubernetes.Clientset, redisClient *redis.Client) error {
	// 全 Namespace を対象に Watch
	watcher, err := clientset.AppsV1().Deployments("").Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to start deployment watch: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}
			if err := handleDeploymentEvent(ctx, clientset, redisClient, event); err != nil {
				fmt.Printf("[deploy-watcher] handle event error: %v\n", err)
			}
		}
	}
}

func handleDeploymentEvent(ctx context.Context, clientset *kubernetes.Clientset, redisClient *redis.Client, event watch.Event) error {
	deploy, ok := event.Object.(*appsv1.Deployment)
	if !ok {
		return nil
	}

	// ns-{projectID} 形式の Namespace のみ処理
	if !strings.HasPrefix(deploy.Namespace, "ns-") {
		return nil
	}

	containerName := deploy.Name
	namespace := deploy.Namespace

	status := determineDeploymentStatus(deploy)

	// DB から Container を名前+Namespace で検索
	var containers []model.Container
	if err := database.DB.
		Joins("JOIN projects ON projects.id = containers.project_id").
		Where("containers.name = ? AND projects.namespace = ?", containerName, namespace).
		Find(&containers).Error; err != nil || len(containers) == 0 {
		return nil
	}
	container := containers[0]

	// ステータスが変化した場合のみ更新
	if container.Status != status {
		model.UpdateContainerStatus(container.ID, status)
		// Redis キャッシュを削除
		database.RedisClient.Del(ctx, fmt.Sprintf("cache:container:%s", container.ID))
	}

	// Pod ログを Redis に配信 (Running 状態時)
	if status == "Running" {
		go streamDeploymentPodLogs(ctx, clientset, redisClient, namespace, containerName)
	}

	return nil
}

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
	if deploy.Status.UnavailableReplicas > 0 {
		return "Failed"
	}
	return "Deploying"
}

func streamDeploymentPodLogs(ctx context.Context, clientset *kubernetes.Clientset, redisClient *redis.Client, namespace, deploymentName string) {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", deploymentName),
	})
	if err != nil || len(pods.Items) == 0 {
		return
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		redisChannel := fmt.Sprintf("stream:pod:%s:%s", namespace, pod.Name)
		go streamPodContainerLogs(ctx, clientset, redisClient, namespace, pod.Name, deploymentName, redisChannel)
	}
}

func streamPodContainerLogs(ctx context.Context, clientset *kubernetes.Clientset, redisClient *redis.Client, namespace, podName, containerName, redisChannel string) {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     true,
		TailLines:  int64Ptr(100),
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
		msg := PodLogMessage{
			PodName:   podName,
			Message:   scanner.Text(),
			Timestamp: time.Now(),
		}
		payload, _ := json.Marshal(msg)
		redisClient.Publish(ctx, redisChannel, payload)
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}
