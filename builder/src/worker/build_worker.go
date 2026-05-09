package worker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"builder/railpack"
	"launchs/shared/database"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/riverqueue/river"
	"k8s.io/client-go/kubernetes"
)

var _ river.Worker[jobs.BuildJobArgs] = (*BuildWorker)(nil)

type BuildWorker struct {
	river.WorkerDefaults[jobs.BuildJobArgs]
}

func (w *BuildWorker) Timeout(*river.Job[jobs.BuildJobArgs]) time.Duration {
	return 40 * time.Minute
}

func (w *BuildWorker) Work(ctx context.Context, job *river.Job[jobs.BuildJobArgs]) error {
	payload := job.Args

	fmt.Printf("[build-worker] processing job %d (build_job_id: %s)\n", job.ID, payload.BuildJobID)

	if err := processBuildTask(ctx, payload); err != nil {
		fmt.Printf("[build-worker] job %d failed: %v\n", job.ID, err)
		return err
	}
	return nil
}

func processBuildTask(ctx context.Context, payload jobs.BuildJobArgs) error {
	model.UpdateContainerStatus(payload.ContainerID, "Building")
	model.UpdateBuildJobStatus(payload.BuildJobID, map[string]interface{}{
		"status":     "Running",
		"started_at": time.Now(),
	})

	uploadEndpoint := os.Getenv("UPLOAD_ENDPOINT")
	if uploadEndpoint == "" {
		uploadEndpoint = "http://builder:8091/internal/upload"
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
		Namespace:      buildNamespace,
		JobID:          strings.ReplaceAll(payload.BuildJobID, "_", "-"),
		Timeout:        35 * time.Minute,
	})
	if err != nil {
		model.UpdateBuildJobStatus(payload.BuildJobID, map[string]interface{}{
			"status":      "Failed",
			"finished_at": time.Now(),
		})
		model.UpdateContainerStatus(payload.ContainerID, "Failed")
		return fmt.Errorf("failed to create railpack client: %w", err)
	}

	jobID, err := client.Build(ctx)
	if err != nil {
		model.UpdateBuildJobStatus(payload.BuildJobID, map[string]interface{}{
			"status":      "Failed",
			"finished_at": time.Now(),
		})
		model.UpdateContainerStatus(payload.ContainerID, "Failed")
		return fmt.Errorf("failed to create build job: %w", err)
	}

	fmt.Printf("[build-worker] K8s Job created (jobID: %s), polling for completion...\n", jobID)
	return waitForBuildJobCompletion(ctx, payload.BuildJobID)
}

func waitForBuildJobCompletion(ctx context.Context, buildJobID string) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(35 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("build job timed out after 35 minutes")
		case <-ticker.C:
			job, err := model.GetBuildJobByID(buildJobID)
			if err != nil {
				fmt.Printf("[build-worker] polling error for %s: %v\n", buildJobID, err)
				continue
			}
			switch job.Status {
			case "Success":
				return nil
			case "Failed":
				return fmt.Errorf("build job failed")
			}
		}
	}
}
