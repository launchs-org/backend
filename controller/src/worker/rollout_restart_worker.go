package worker

import (
	"context"
	"fmt"
	"time"

	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/riverqueue/river"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"launchs/shared/database"
)

var _ river.Worker[jobs.RolloutRestartJobArgs] = (*RolloutRestartWorker)(nil)

type RolloutRestartWorker struct {
	river.WorkerDefaults[jobs.RolloutRestartJobArgs]
}

func (w *RolloutRestartWorker) Timeout(*river.Job[jobs.RolloutRestartJobArgs]) time.Duration {
	return 5 * time.Minute
}

func (w *RolloutRestartWorker) Work(ctx context.Context, job *river.Job[jobs.RolloutRestartJobArgs]) error {
	payload := job.Args
	fmt.Printf("[rollout-restart-worker] processing job %d (container_id: %s)\n", job.ID, payload.ContainerID)

	clientset := database.K8sClientset.(*kubernetes.Clientset)

	patch := fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`,
		time.Now().UTC().Format(time.RFC3339),
	)

	if _, err := clientset.AppsV1().Deployments(payload.Namespace).Patch(
		ctx, payload.Deployment, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{},
	); err != nil {
		model.UpdateContainerStatus(payload.ContainerID, "Failed")
		return fmt.Errorf("failed to patch deployment: %w", err)
	}

	model.UpdateContainerStatus(payload.ContainerID, "Deploying")
	fmt.Printf("[rollout-restart-worker] restarted deployment %s in %s\n", payload.Deployment, payload.Namespace)
	return nil
}
