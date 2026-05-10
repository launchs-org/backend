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

var _ river.Worker[jobs.DeleteContainerJobArgs] = (*DeleteContainerWorker)(nil)

type DeleteContainerWorker struct {
	river.WorkerDefaults[jobs.DeleteContainerJobArgs]
}

func (w *DeleteContainerWorker) Timeout(*river.Job[jobs.DeleteContainerJobArgs]) time.Duration {
	return 10 * time.Minute
}

func (w *DeleteContainerWorker) Work(ctx context.Context, job *river.Job[jobs.DeleteContainerJobArgs]) error {
	payload := job.Args
	fmt.Printf("[delete-container-worker] processing job %d (container_id: %s)\n", job.ID, payload.ContainerID)

	container, err := model.GetContainerByID(payload.ContainerID)
	if err != nil {
		fmt.Printf("[delete-container-worker] container not found in DB, skipping k8s cleanup: %v\n", err)
		return nil
	}

	clientset := database.K8sClientset.(*kubernetes.Clientset)
	namespace := payload.Namespace
	deploymentName := container.Name

	// Deployment 削除
	err = clientset.AppsV1().Deployments(namespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		fmt.Printf("[delete-container-worker] failed to delete deployment: %v\n", err)
	}

	// Service 削除 (存在すれば)
	_ = clientset.CoreV1().Services(namespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})

	// IngressRoute 削除はdelete_ingress workerで行うので省略

	// DB レコード削除
	if err := model.DeleteContainer(payload.ContainerID); err != nil {
		return fmt.Errorf("failed to delete container from DB: %w", err)
	}

	fmt.Printf("[delete-container-worker] deleted container %s\n", payload.ContainerID)
	return nil
}
