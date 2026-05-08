package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"launchs/shared/database"
	"launchs/shared/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DeleteContainerPayload は delete_container タスクのペイロードです
type DeleteContainerPayload struct {
	ContainerID string `json:"container_id"`
	Namespace   string `json:"namespace"`
	ImageName   string `json:"image_name"`
}

// DeleteProjectPayload は delete_project タスクのペイロードです
type DeleteProjectPayload struct {
	ProjectID string `json:"project_id"`
	Namespace string `json:"namespace"`
}

// RunDeleteWorker は delete_container / delete_project タスクを処理するワーカーです
func RunDeleteWorker(ctx context.Context) {
	fmt.Println("[delete-worker] starting")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// delete_container を優先して処理
		task, err := model.ClaimTask(ctx, "delete_container")
		if err != nil {
			fmt.Printf("[delete-worker] claim error: %v\n", err)
			workerSleep(ctx, 5*time.Second)
			continue
		}
		if task == nil {
			// delete_project を試みる
			task, err = model.ClaimTask(ctx, "delete_project")
			if err != nil || task == nil {
				workerSleep(ctx, 5*time.Second)
				continue
			}
		}

		fmt.Printf("[delete-worker] processing task %s (type: %s)\n", task.ID, task.TaskType)
		var processErr error
		switch task.TaskType {
		case "delete_container":
			processErr = processDeleteContainerTask(ctx, task)
		case "delete_project":
			processErr = processDeleteProjectTask(ctx, task)
		}

		if processErr != nil {
			fmt.Printf("[delete-worker] task %s failed: %v\n", task.ID, processErr)
			model.FailTask(task.ID, processErr.Error())
		} else {
			model.CompleteTask(task.ID)
		}
	}
}

func processDeleteContainerTask(ctx context.Context, task *model.Task) error {
	var payload DeleteContainerPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	clientset := database.K8sClientset.(*kubernetes.Clientset)
	ns := payload.Namespace

	// K8s リソース削除
	clientset.AppsV1().Deployments(ns).Delete(ctx, payload.ContainerID, metav1.DeleteOptions{})
	clientset.CoreV1().Services(ns).Delete(ctx, payload.ContainerID, metav1.DeleteOptions{})
	clientset.NetworkingV1().Ingresses(ns).Delete(ctx, payload.ContainerID, metav1.DeleteOptions{})

	// ボリューム（PVC）削除
	volumes, _ := model.GetVolumesByContainerID(payload.ContainerID)
	for _, vol := range volumes {
		pvcName := fmt.Sprintf("pvc-%s", vol.ID)
		clientset.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, pvcName, metav1.DeleteOptions{})
	}

	// DB レコード削除
	model.DeleteImagesByContainerID(payload.ContainerID)
	model.DeleteServiceByContainerID(payload.ContainerID)
	model.DeleteIngress(payload.ContainerID)
	model.DeleteBuildJobsByContainerID(payload.ContainerID)
	model.DeleteContainer(payload.ContainerID)

	// Harbor イメージ削除タスクを作成（builder が処理）
	if payload.ImageName != "" {
		images, err := model.GetImagesByContainerID(payload.ContainerID)
		tags := []string{}
		if err == nil {
			for _, img := range images {
				tags = append(tags, img.ID)
			}
		}
		if len(tags) == 0 {
			tags = []string{"latest"}
		}

		tagsJSON, _ := json.Marshal(tags)
		deleteImageTask := &model.Task{
			ID:       "task_delimg_" + payload.ContainerID[:8] + "_" + fmt.Sprintf("%d", time.Now().UnixNano()),
			TaskType: "delete_image",
			Status:   "pending",
			Payload:  fmt.Sprintf(`{"image_name":%q,"image_tags":%s}`, payload.ImageName, tagsJSON),
			TimeoutAt: time.Now().Add(5 * time.Minute),
		}
		model.CreateTask(deleteImageTask)
	}

	fmt.Printf("[delete-worker] container %s deleted\n", payload.ContainerID)
	return nil
}

func processDeleteProjectTask(ctx context.Context, task *model.Task) error {
	var payload DeleteProjectPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	clientset := database.K8sClientset.(*kubernetes.Clientset)

	// Namespace 削除（配下リソースは連鎖削除される）
	if err := clientset.CoreV1().Namespaces().Delete(ctx, payload.Namespace, metav1.DeleteOptions{}); err != nil {
		fmt.Printf("[delete-worker] namespace delete error (may be already gone): %v\n", err)
	}

	// DB からプロジェクト削除
	model.DeleteProject(payload.ProjectID)

	fmt.Printf("[delete-worker] project %s deleted\n", payload.ProjectID)
	return nil
}
