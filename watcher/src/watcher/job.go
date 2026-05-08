package watcher

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"launchs/shared/database"
	"launchs/shared/model"

	"github.com/redis/go-redis/v9"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// JobLogMessage は Redis に Publish するログメッセージの形式です
type JobLogMessage struct {
	Container string    `json:"container"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// WatchJobs は K8s Job を監視し、BuildJob DB と Redis Pub/Sub を更新します
func WatchJobs(ctx context.Context) {
	namespace := os.Getenv("BUILD_NAMESPACE")
	if namespace == "" {
		namespace = "buildkit"
	}

	clientset := database.K8sClientset.(*kubernetes.Clientset)
	redisClient := database.RedisClient

	fmt.Println("[job-watcher] starting job watcher in namespace:", namespace)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := runJobWatch(ctx, clientset, redisClient, namespace); err != nil {
			fmt.Printf("[job-watcher] watch error: %v, restarting...\n", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func runJobWatch(ctx context.Context, clientset *kubernetes.Clientset, redisClient *redis.Client, namespace string) error {
	watcher, err := clientset.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "app=railpack",
	})
	if err != nil {
		return fmt.Errorf("failed to start job watch: %w", err)
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
			if err := handleJobEvent(ctx, clientset, redisClient, namespace, event); err != nil {
				fmt.Printf("[job-watcher] handle event error: %v\n", err)
			}
		}
	}
}

func handleJobEvent(ctx context.Context, clientset *kubernetes.Clientset, redisClient *redis.Client, namespace string, event watch.Event) error {
	job, ok := event.Object.(*batchv1.Job)
	if !ok {
		return nil
	}

	// K8s Job 名から BuildJob ID を復元: "railpack-bj-xxx" → "bj_xxx"
	buildJobID := jobNameToBuildJobID(job.Name)
	if buildJobID == "" {
		return nil
	}

	redisChannel := fmt.Sprintf("stream:job:%s:%s", namespace, job.Name)

	switch event.Type {
	case watch.Added, watch.Modified:
		status := determineJobStatus(job)
		updates := map[string]interface{}{"status": status}

		if status == "Running" {
			now := time.Now()
			updates["started_at"] = now
			// Pod ログを非同期でストリーム
			go streamJobLogs(ctx, clientset, redisClient, namespace, job.Name, buildJobID, redisChannel)
		}

		if status == "Success" || status == "Failed" {
			now := time.Now()
			updates["finished_at"] = now
			model.UpdateBuildJobStatus(buildJobID, updates)
			return nil
		}

		model.UpdateBuildJobStatus(buildJobID, updates)

	case watch.Deleted:
		// Job 削除は TTL で自動削除されるため無視
	}

	return nil
}

func streamJobLogs(ctx context.Context, clientset *kubernetes.Clientset, redisClient *redis.Client, namespace, jobName, buildJobID, redisChannel string) {
	// Pod が起動するまで待機
	pod, err := waitForJobPod(ctx, clientset, namespace, jobName)
	if err != nil {
		fmt.Printf("[job-watcher] pod wait error for %s: %v\n", jobName, err)
		return
	}

	containers := []string{}
	for _, c := range pod.Spec.InitContainers {
		containers = append(containers, c.Name)
	}
	for _, c := range pod.Spec.Containers {
		containers = append(containers, c.Name)
	}

	for _, containerName := range containers {
		if err := waitForContainerRunning(ctx, clientset, namespace, pod.Name, containerName); err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		streamContainerLogs(ctx, clientset, redisClient, namespace, pod.Name, containerName, buildJobID, redisChannel)
	}
}

func streamContainerLogs(ctx context.Context, clientset *kubernetes.Clientset, redisClient *redis.Client, namespace, podName, containerName, buildJobID, redisChannel string) {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     true,
		Timestamps: true,
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
		_, message := parseTimestampedLine(line)
		logLine := fmt.Sprintf("[%s] %s\n", containerName, message)

		// DB に追記
		model.AppendBuildLog(buildJobID, []byte(logLine))

		// Redis に Publish
		msg := JobLogMessage{
			Container: containerName,
			Message:   message,
			Timestamp: time.Now(),
		}
		payload, _ := json.Marshal(msg)
		redisClient.Publish(ctx, redisChannel, payload)
	}
}

func waitForJobPod(ctx context.Context, clientset *kubernetes.Clientset, namespace, jobName string) (*corev1.Pod, error) {
	deadline := time.Now().Add(10 * time.Minute)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for pod")
		}
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
		all := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
		for _, cs := range all {
			if cs.Name == containerName && (cs.State.Running != nil || cs.State.Terminated != nil) {
				return nil
			}
		}
	}
}

func determineJobStatus(job *batchv1.Job) string {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return "Success"
		}
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return "Failed"
		}
	}
	if job.Status.Active > 0 {
		return "Running"
	}
	if job.Status.Succeeded > 0 {
		return "Success"
	}
	if job.Status.Failed > 0 {
		return "Failed"
	}
	return "Queued"
}

// jobNameToBuildJobID は "railpack-bj-xxx-yyy" → "bj_xxx-yyy" に変換します
func jobNameToBuildJobID(jobName string) string {
	if !strings.HasPrefix(jobName, "railpack-") {
		return ""
	}
	// "railpack-" を除いた部分: "bj-xxx-yyy"
	withoutPrefix := strings.TrimPrefix(jobName, "railpack-")
	// 最初の "-" を "_" に置換: "bj_xxx-yyy"
	return strings.Replace(withoutPrefix, "-", "_", 1)
}

func parseTimestampedLine(line string) (time.Time, string) {
	for i, ch := range line {
		if ch == ' ' {
			ts, err := time.Parse(time.RFC3339Nano, line[:i])
			if err == nil {
				return ts, line[i+1:]
			}
			break
		}
	}
	return time.Now(), line
}
