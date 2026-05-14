package worker

import (
	"context"
	"fmt"
	"time"

	"launchs/shared/database"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/riverqueue/river"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var _ river.Worker[jobs.ScaleJobArgs] = (*ScaleWorker)(nil)

type ScaleWorker struct {
	river.WorkerDefaults[jobs.ScaleJobArgs]
}

func (w *ScaleWorker) Timeout(*river.Job[jobs.ScaleJobArgs]) time.Duration {
	return 2 * time.Minute
}

func (w *ScaleWorker) Work(ctx context.Context, job *river.Job[jobs.ScaleJobArgs]) error {
	p := job.Args
	fmt.Printf("[scale-worker] scaling %s in %s to %d replicas\n", p.Deployment, p.Namespace, p.Replicas)

	clientset := database.K8sClientset.(*kubernetes.Clientset)

	model.UpdateContainerStatus(p.ContainerID, "Scaling")

	scale, err := clientset.AppsV1().Deployments(p.Namespace).GetScale(ctx, p.Deployment, metav1.GetOptions{})
	if err != nil {
		model.UpdateContainerStatus(p.ContainerID, "Failed")
		return fmt.Errorf("failed to get scale: %w", err)
	}

	scale.Spec.Replicas = int32(p.Replicas)
	if _, err := clientset.AppsV1().Deployments(p.Namespace).UpdateScale(ctx, p.Deployment, scale, metav1.UpdateOptions{}); err != nil {
		model.UpdateContainerStatus(p.ContainerID, "Failed")
		return fmt.Errorf("failed to update scale: %w", err)
	}

	if err := database.DB.Model(&model.Container{}).Where("id = ?", p.ContainerID).
		Update("replicas", p.Replicas).Error; err != nil {
		return fmt.Errorf("failed to update replicas in db: %w", err)
	}

	// ステータスは watcher が Deployment の Ready 状態を確認して Running に戻す
	fmt.Printf("[scale-worker] scaled %s to %d replicas, waiting for watcher to confirm\n", p.Deployment, p.Replicas)
	return nil
}
