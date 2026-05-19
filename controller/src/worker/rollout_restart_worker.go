package worker

import (
	"context"
	"fmt"
	"time"

	"launchs/shared/database"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/riverqueue/river"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var _ river.Worker[jobs.RolloutRestartJobArgs] = (*RolloutRestartWorker)(nil)

type RolloutRestartWorker struct {
	river.WorkerDefaults[jobs.RolloutRestartJobArgs]
}

func (w *RolloutRestartWorker) Timeout(*river.Job[jobs.RolloutRestartJobArgs]) time.Duration {
	return 10 * time.Minute
}

func (w *RolloutRestartWorker) Work(ctx context.Context, job *river.Job[jobs.RolloutRestartJobArgs]) error {
	payload := job.Args
	fmt.Printf("[redeploy-worker] processing job %d (container_id: %s)\n", job.ID, payload.ContainerID)

	container, err := model.GetContainerByID(payload.ContainerID)
	if err != nil {
		return fmt.Errorf("container not found: %w", err)
	}

	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}

	clientset := database.K8sClientset.(*kubernetes.Clientset)
	namespace := project.Namespace

	err = clientset.AppsV1().Deployments(namespace).Delete(ctx, container.Name, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		model.UpdateContainerStatus(payload.ContainerID, "Failed")
		return fmt.Errorf("failed to delete deployment: %w", err)
	}
	fmt.Printf("[redeploy-worker] deleted deployment %s in %s\n", container.Name, namespace)

	var deployment *appsv1.Deployment
	// テンプレートコンテナはイメージ名が固定（mysql:8.0 など）なので container.ImageID をそのまま使う
	if container.ContainerType != "" && container.ContainerType != "user" {
		deployment, err = buildTemplateDeployment(container, namespace, container.ImageID)
	} else {
		deployment, err = buildDeployment(container, namespace, payload.ImageRef)
	}
	if err != nil {
		model.UpdateContainerStatus(payload.ContainerID, "Failed")
		return fmt.Errorf("failed to build deployment spec: %w", err)
	}

	if _, err := clientset.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
		model.UpdateContainerStatus(payload.ContainerID, "Failed")
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	model.UpdateContainerStatus(payload.ContainerID, "Deploying")
	fmt.Printf("[redeploy-worker] recreated deployment %s in %s\n", container.Name, namespace)

	// テンプレートコンテナは再デプロイ後にも K8s Service を再作成する
	if container.ContainerType != "" && container.ContainerType != "user" {
		if err := enqueueTemplateService(ctx, container, namespace, container.ContainerType); err != nil {
			fmt.Printf("[redeploy-worker] failed to enqueue service job: %v\n", err)
		}
	}

	return nil
}
