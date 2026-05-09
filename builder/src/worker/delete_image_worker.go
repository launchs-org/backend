package worker

import (
	"context"
	"fmt"

	"builder/service"
	"launchs/shared/job_queue/jobs"

	"github.com/riverqueue/river"
)

type DeleteImageWorker struct {
	river.WorkerDefaults[jobs.DeleteImageJobArgs]
}

func (w *DeleteImageWorker) Work(ctx context.Context, job *river.Job[jobs.DeleteImageJobArgs]) error {
	payload := job.Args

	fmt.Printf("[delete-image-worker] processing job %d (image: %s)\n", job.ID, payload.ImageName)

	if payload.ImageName == "" {
		return fmt.Errorf("image_name is required")
	}

	tags := payload.Tags
	if len(tags) == 0 {
		tags = []string{"latest"}
	}

	return service.DeleteFromRegistry(payload.ImageName, tags)
}
