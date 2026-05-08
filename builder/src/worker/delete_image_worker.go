package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"builder/service"
	"launchs/shared/model"
)

// DeleteImagePayload は delete_image タスクのペイロードです
type DeleteImagePayload struct {
	ImageName string   `json:"image_name"`
	ImageTags []string `json:"image_tags"`
}

// RunDeleteImageWorker は delete_image タスクを処理するワーカーです
func RunDeleteImageWorker(ctx context.Context) {
	fmt.Println("[delete-image-worker] starting")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		task, err := model.ClaimTask(ctx, "delete_image")
		if err != nil {
			fmt.Printf("[delete-image-worker] claim error: %v\n", err)
			sleep(ctx, 5*time.Second)
			continue
		}
		if task == nil {
			sleep(ctx, 5*time.Second)
			continue
		}

		fmt.Printf("[delete-image-worker] processing task %s\n", task.ID)
		if err := processDeleteImageTask(ctx, task); err != nil {
			fmt.Printf("[delete-image-worker] task %s failed: %v\n", task.ID, err)
			model.FailTask(task.ID, err.Error())
		} else {
			model.CompleteTask(task.ID)
		}
	}
}

func processDeleteImageTask(ctx context.Context, task *model.Task) error {
	var payload DeleteImagePayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if payload.ImageName == "" {
		return fmt.Errorf("image_name is required")
	}

	tags := payload.ImageTags
	if len(tags) == 0 {
		tags = []string{"latest"}
	}

	return service.DeleteFromRegistry(payload.ImageName, tags)
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
