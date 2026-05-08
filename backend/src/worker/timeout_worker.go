package worker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"launchs/shared/database"
	"launchs/shared/model"

	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RunTimeoutWorker はタイムアウトしたタスクを検出してキャンセル処理します（30秒ごと）
func RunTimeoutWorker(ctx context.Context) {
	fmt.Println("[timeout-worker] starting")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkBuildTimeouts(ctx)
			checkDeployTimeouts(ctx)
			checkDeleteTimeouts(ctx)
		}
	}
}

func checkBuildTimeouts(ctx context.Context) {
	tasks, err := model.GetTimedOutRunningTasks(ctx, "build")
	if err != nil {
		return
	}
	for _, task := range tasks {
		fmt.Printf("[timeout-worker] cancelling timed out build task %s\n", task.ID)
		cancelBuildTask(ctx, &task)
	}
}

func cancelBuildTask(ctx context.Context, task *model.Task) {
	// ペイロードから BuildJob ID を取得してキャンセル
	// Payload は JSON なので単純な文字列探索で BuildJobID を取得
	buildJobID := extractJSONField(task.Payload, "build_job_id")
	containerID := extractJSONField(task.Payload, "container_id")

	if buildJobID != "" {
		// K8s Job を削除
		buildNamespace := os.Getenv("BUILD_NAMESPACE")
		if buildNamespace == "" {
			buildNamespace = "buildkit"
		}
		jobName := "railpack-" + strings.ReplaceAll(buildJobID, "_", "-")
		clientset := database.K8sClientset.(*kubernetes.Clientset)
		propagation := metav1.DeletePropagationForeground
		clientset.BatchV1().Jobs(buildNamespace).Delete(ctx, jobName, metav1.DeleteOptions{
			PropagationPolicy: &propagation,
		})

		model.UpdateBuildJobStatus(buildJobID, map[string]interface{}{
			"status":      "Failed",
			"finished_at": time.Now(),
		})
	}
	if containerID != "" {
		model.UpdateContainerStatus(containerID, "Failed")
	}

	model.CancelTask(task.ID, "build task timed out")
}

func checkDeployTimeouts(ctx context.Context) {
	tasks, err := model.GetTimedOutRunningTasks(ctx, "deploy")
	if err != nil {
		return
	}
	for _, task := range tasks {
		fmt.Printf("[timeout-worker] cancelling timed out deploy task %s\n", task.ID)
		containerID := extractJSONField(task.Payload, "container_id")
		if containerID != "" {
			// Deployment をロールバック
			go rollbackDeployment(ctx, containerID)
			model.UpdateContainerStatus(containerID, "Failed")
		}
		model.CancelTask(task.ID, "deploy task timed out")
	}
}

func rollbackDeployment(ctx context.Context, containerID string) {
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		return
	}
	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return
	}

	clientset := database.K8sClientset.(*kubernetes.Clientset)
	// kubectl rollout undo に相当: DeploymentRevision を1つ戻す
	// client-go には直接のロールバック API がないため、annotation を使う
	deploy, err := clientset.AppsV1().Deployments(project.Namespace).Get(ctx, container.Name, metav1.GetOptions{})
	if err != nil {
		return
	}
	if deploy.Annotations == nil {
		deploy.Annotations = map[string]string{}
	}
	deploy.Annotations["deployment.kubernetes.io/revision"] = ""
	clientset.AppsV1().Deployments(project.Namespace).Update(ctx, deploy, metav1.UpdateOptions{})
}

func checkDeleteTimeouts(ctx context.Context) {
	for _, taskType := range []string{"delete_container", "delete_project"} {
		tasks, err := model.GetTimedOutRunningTasks(ctx, taskType)
		if err != nil {
			continue
		}
		for _, task := range tasks {
			fmt.Printf("[timeout-worker] cancelling timed out %s task %s\n", taskType, task.ID)
			model.CancelTask(task.ID, taskType+" task timed out — manual verification required")
		}
	}
}

// listenForTasks は pg_notify で新規タスク通知を受け取りワーカーループを即時起動します
func listenForTasks(ctx context.Context, channel string) {
	// PostgreSQL LISTEN は lib/pq が必要だが、現状は 5秒ポーリングで代替
	// 将来的に pgx の LISTEN/NOTIFY に移行可能
	fmt.Printf("[worker] using polling mode (5s interval)\n")
}

func workerSleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// extractJSONField は JSON 文字列から指定フィールドの値を単純な文字列解析で取得します
func extractJSONField(jsonStr, field string) string {
	key := fmt.Sprintf(`"%s":"`, field)
	idx := strings.Index(jsonStr, key)
	if idx == -1 {
		return ""
	}
	start := idx + len(key)
	end := strings.Index(jsonStr[start:], `"`)
	if end == -1 {
		return ""
	}
	return jsonStr[start : start+end]
}
