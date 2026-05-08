package worker

import (
	"context"
	"fmt"
	"time"

	"launchs/shared/model"
)

// RunTimeoutWorker は delete_image タスクのタイムアウトを監視します
func RunTimeoutWorker(ctx context.Context) {
	fmt.Println("[timeout-worker] starting")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkTimeouts(ctx)
		}
	}
}

func checkTimeouts(ctx context.Context) {
	tasks, err := model.GetTimedOutRunningTasks(ctx, "delete_image")
	if err != nil {
		fmt.Printf("[timeout-worker] error querying timed out tasks: %v\n", err)
		return
	}

	for _, task := range tasks {
		fmt.Printf("[timeout-worker] cancelling timed out task %s\n", task.ID)
		model.CancelTask(task.ID, "task timed out")
	}
}
