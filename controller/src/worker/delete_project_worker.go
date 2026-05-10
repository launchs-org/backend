package worker

import (
	"context"
	"fmt"
	"time"

	"launchs/shared/database"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/riverqueue/river"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var _ river.Worker[jobs.DeleteProjectJobArgs] = (*DeleteProjectWorker)(nil)

type DeleteProjectWorker struct {
	river.WorkerDefaults[jobs.DeleteProjectJobArgs]
}

func (w *DeleteProjectWorker) Timeout(*river.Job[jobs.DeleteProjectJobArgs]) time.Duration {
	return 10 * time.Minute
}

func (w *DeleteProjectWorker) Work(ctx context.Context, job *river.Job[jobs.DeleteProjectJobArgs]) error {
	payload := job.Args
	fmt.Printf("[delete-project-worker] processing job %d (project_id: %s)\n", job.ID, payload.ProjectID)

	clientset := database.K8sClientset.(*kubernetes.Clientset)

	// Namespace 削除（配下のリソースも全て削除される）
	err := clientset.CoreV1().Namespaces().Delete(ctx, payload.Namespace, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete namespace %s: %w", payload.Namespace, err)
	}

	// DB レコード削除
	if err := model.DeleteProject(payload.ProjectID); err != nil {
		return fmt.Errorf("failed to delete project from DB: %w", err)
	}

	fmt.Printf("[delete-project-worker] deleted project %s (namespace: %s)\n", payload.ProjectID, payload.Namespace)
	return nil
}
