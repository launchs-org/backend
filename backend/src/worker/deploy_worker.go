package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"launchs/shared/model"

	"backend/service"
)

// DeployPayload は deploy タスクのペイロードです
type DeployPayload struct {
	ContainerID string `json:"container_id"`
	ImageRef    string `json:"image_ref"`
	Namespace   string `json:"namespace"`
	BuildJobID  string `json:"build_job_id"`
}

// RunDeployWorker は deploy タスクを処理するワーカーです
func RunDeployWorker(ctx context.Context) {
	fmt.Println("[deploy-worker] starting")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		task, err := model.ClaimTask(ctx, "deploy")
		if err != nil {
			fmt.Printf("[deploy-worker] claim error: %v\n", err)
			workerSleep(ctx, 5*time.Second)
			continue
		}
		if task == nil {
			workerSleep(ctx, 5*time.Second)
			continue
		}

		fmt.Printf("[deploy-worker] processing task %s\n", task.ID)
		if err := processDeployTask(ctx, task); err != nil {
			fmt.Printf("[deploy-worker] task %s failed: %v\n", task.ID, err)
			model.FailTask(task.ID, err.Error())
		} else {
			model.CompleteTask(task.ID)
		}
	}
}

func processDeployTask(ctx context.Context, task *model.Task) error {
	var payload DeployPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	model.UpdateContainerStatus(payload.ContainerID, "Deploying")

	// K8s Deployment を作成/更新 (既存の DeployToKubernetes を再利用)
	go service.DeployToKubernetes(payload.ContainerID, payload.ImageRef)

	return nil
}
