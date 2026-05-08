package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"launchs/shared/database"
	"launchs/shared/model"
	"launchs/shared/utils"

	"backend/railpack"

	"k8s.io/client-go/kubernetes"
)

// BuildPayload は build タスクのペイロードです
type BuildPayload struct {
	BuildJobID    string `json:"build_job_id"`
	ContainerID   string `json:"container_id"`
	ImageID       string `json:"image_id"`
	ProjectID     string `json:"project_id"`
	ProjectName   string `json:"project_name"`
	ContainerName string `json:"container_name"`
	Namespace     string `json:"namespace"`

	RepositoryURL string `json:"repository_url"`
	Branch        string `json:"branch"`
	Directory     string `json:"directory"`

	BuildType      string `json:"build_type"`
	DockerfilePath string `json:"dockerfile_path"`
}

// RunBuildWorker は build タスクを処理するワーカーです
func RunBuildWorker(ctx context.Context) {
	fmt.Println("[build-worker] starting")

	// pg_notify の LISTEN を設定
	go listenForTasks(ctx, "task_created")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		task, err := model.ClaimTask(ctx, "build")
		if err != nil {
			fmt.Printf("[build-worker] claim error: %v\n", err)
			workerSleep(ctx, 5*time.Second)
			continue
		}
		if task == nil {
			// タスクなし — ポーリング間隔
			workerSleep(ctx, 5*time.Second)
			continue
		}

		fmt.Printf("[build-worker] processing task %s\n", task.ID)
		if err := processBuildTask(ctx, task); err != nil {
			fmt.Printf("[build-worker] task %s failed: %v\n", task.ID, err)
			model.FailTask(task.ID, err.Error())
		} else {
			model.CompleteTask(task.ID)
		}
	}
}

func processBuildTask(ctx context.Context, task *model.Task) error {
	var payload BuildPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	// ステータスを Building に更新
	model.UpdateContainerStatus(payload.ContainerID, "Building")
	model.UpdateBuildJobStatus(payload.BuildJobID, map[string]interface{}{
		"status":     "Running",
		"started_at": time.Now(),
	})

	uploadEndpoint := os.Getenv("UPLOAD_ENDPOINT")
	if uploadEndpoint == "" {
		uploadEndpoint = "http://10.10.11.8:8091/internal/upload"
	}

	uploadToken, err := utils.GenerateJobToken(utils.JobTokenClaim{
		JobID:     payload.BuildJobID,
		ImageName: payload.ContainerID,
		ImageTag:  payload.ImageID,
	})
	if err != nil {
		return fmt.Errorf("failed to generate upload token: %w", err)
	}

	buildNamespace := os.Getenv("BUILD_NAMESPACE")
	if buildNamespace == "" {
		buildNamespace = "buildkit"
	}

	clientset := database.K8sClientset.(*kubernetes.Clientset)

	client, err := railpack.New(clientset, railpack.BuildConfig{
		GitRepo:        payload.RepositoryURL,
		GitBranch:      payload.Branch,
		Subdir:         payload.Directory,
		ImageName:      payload.ContainerID,
		ImageTag:       payload.ImageID,
		UploadEndpoint: uploadEndpoint,
		UploadToken:    uploadToken,
		Namespace:      buildNamespace,
		Timeout:        30 * time.Minute,
		JobID:          strings.ReplaceAll(payload.BuildJobID, "_", "-"),
	})
	if err != nil {
		return fmt.Errorf("failed to init railpack: %w", err)
	}

	_, err = client.Build(ctx)
	if err != nil {
		model.UpdateBuildJobStatus(payload.BuildJobID, map[string]interface{}{
			"status":      "Failed",
			"finished_at": time.Now(),
		})
		model.UpdateContainerStatus(payload.ContainerID, "Failed")
		return fmt.Errorf("failed to start build: %w", err)
	}

	fmt.Printf("[build-worker] K8s Job created for build_job %s\n", payload.BuildJobID)
	return nil
}
