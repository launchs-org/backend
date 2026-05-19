package worker

import (
	"context"
	"fmt"
	"time"

	"launchs/shared/database"
	"launchs/shared/job_queue"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"
	"launchs/shared/templates"

	"github.com/riverqueue/river"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var _ river.Worker[jobs.DeployStatefulSetJobArgs] = (*DeployStatefulSetWorker)(nil)

type DeployStatefulSetWorker struct {
	river.WorkerDefaults[jobs.DeployStatefulSetJobArgs]
}

func (w *DeployStatefulSetWorker) Timeout(*river.Job[jobs.DeployStatefulSetJobArgs]) time.Duration {
	return 10 * time.Minute
}

func (w *DeployStatefulSetWorker) Work(ctx context.Context, job *river.Job[jobs.DeployStatefulSetJobArgs]) error {
	payload := job.Args
	fmt.Printf("[deploy-template-worker] processing job %d (container_id: %s, type: %s)\n", job.ID, payload.ContainerID, payload.ContainerType)

	if err := processDeployTemplate(ctx, payload); err != nil {
		fmt.Printf("[deploy-template-worker] job %d failed: %v\n", job.ID, err)
		return err
	}
	return nil
}

func processDeployTemplate(ctx context.Context, payload jobs.DeployStatefulSetJobArgs) error {
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

	model.UpdateContainerStatus(payload.ContainerID, "Deploying")

	deployment, err := buildTemplateDeployment(container, namespace, payload.ImageRef)
	if err != nil {
		model.UpdateContainerStatus(payload.ContainerID, "Failed")
		return fmt.Errorf("failed to build deployment spec: %w", err)
	}

	existing, err := clientset.AppsV1().Deployments(namespace).Get(ctx, container.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			model.UpdateContainerStatus(payload.ContainerID, "Failed")
			return fmt.Errorf("failed to get deployment: %w", err)
		}
		if _, err := clientset.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
			model.UpdateContainerStatus(payload.ContainerID, "Failed")
			return fmt.Errorf("failed to create deployment: %w", err)
		}
		fmt.Printf("[deploy-template-worker] created deployment %s in %s\n", container.Name, namespace)
	} else {
		deployment.ResourceVersion = existing.ResourceVersion
		if _, err := clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{}); err != nil {
			model.UpdateContainerStatus(payload.ContainerID, "Failed")
			return fmt.Errorf("failed to update deployment: %w", err)
		}
		fmt.Printf("[deploy-template-worker] updated deployment %s in %s\n", container.Name, namespace)
	}

	// テンプレートのポートに対応するK8s Serviceを作成・更新する
	if err := enqueueTemplateService(ctx, container, project.Namespace, payload.ContainerType); err != nil {
		fmt.Printf("[deploy-template-worker] failed to enqueue service job: %v\n", err)
	}

	return nil
}

func enqueueTemplateService(ctx context.Context, container *model.Container, namespace, containerType string) error {
	tmpl, ok := templates.GetByID(containerType)
	if !ok || tmpl.Port == 0 {
		return nil
	}

	return job_queue.EnqueueTo(ctx, "controller", jobs.UpdateServiceJobArgs{
		ContainerID: container.ID,
		Namespace:   namespace,
		ServiceName: container.Name,
		ServiceType: "LoadBalancer",
		Ports: []jobs.ServicePortArgs{
			{Name: containerType, Port: tmpl.Port, Target: tmpl.Port},
		},
		IsActive: true,
	}, nil)
}

func buildTemplateDeployment(container *model.Container, namespace, imageRef string) (*appsv1.Deployment, error) {
	replicas := int32(1)

	envVars, err := parseEnvVars(container.EnvVars)
	if err != nil {
		return nil, fmt.Errorf("invalid env_vars: %w", err)
	}

	labels := map[string]string{
		"app":             container.Name,
		"container-id":    container.ID,
		"launchs-managed": "true",
	}

	var containerPorts []corev1.ContainerPort
	if tmpl, ok := templates.GetByID(container.ContainerType); ok && tmpl.Port > 0 {
		containerPorts = []corev1.ContainerPort{{ContainerPort: int32(tmpl.Port), Protocol: corev1.ProtocolTCP}}
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      container.Name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": container.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            container.Name,
							Image:           imageRef,
							Env:             envVars,
							Ports:           containerPorts,
							ImagePullPolicy: corev1.PullAlways,
						},
					},
				},
			},
		},
	}, nil
}
