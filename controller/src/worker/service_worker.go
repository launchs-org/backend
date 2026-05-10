package worker

import (
	"context"
	"fmt"
	"time"

	"launchs/shared/database"
	"launchs/shared/job_queue/jobs"

	"github.com/riverqueue/river"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

var _ river.Worker[jobs.UpdateServiceJobArgs] = (*UpdateServiceWorker)(nil)

type UpdateServiceWorker struct {
	river.WorkerDefaults[jobs.UpdateServiceJobArgs]
}

func (w *UpdateServiceWorker) Timeout(*river.Job[jobs.UpdateServiceJobArgs]) time.Duration {
	return 5 * time.Minute
}

func (w *UpdateServiceWorker) Work(ctx context.Context, job *river.Job[jobs.UpdateServiceJobArgs]) error {
	payload := job.Args
	fmt.Printf("[update-service-worker] processing job %d (container_id: %s)\n", job.ID, payload.ContainerID)

	clientset := database.K8sClientset.(*kubernetes.Clientset)
	namespace := payload.Namespace
	name := payload.ServiceName

	if !payload.IsActive || len(payload.Ports) == 0 {
		err := clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("[update-service-worker] failed to delete service (may not exist): %v\n", err)
		}
		return nil
	}

	var k8sPorts []corev1.ServicePort
	for _, p := range payload.Ports {
		k8sPorts = append(k8sPorts, corev1.ServicePort{
			Name:       p.Name,
			Port:       int32(p.Port),
			TargetPort: intstr.FromInt(p.Target),
		})
	}

	k8sSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"app": name},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceType(payload.ServiceType),
			Selector: map[string]string{"app": name},
			Ports:    k8sPorts,
		},
	}

	svcClient := clientset.CoreV1().Services(namespace)
	existing, err := svcClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if _, err := svcClient.Create(ctx, k8sSvc, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create k8s service: %w", err)
		}
	} else {
		existing.Spec.Type = k8sSvc.Spec.Type
		existing.Spec.Ports = k8sSvc.Spec.Ports
		if _, err := svcClient.Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update k8s service: %w", err)
		}
	}

	fmt.Printf("[update-service-worker] synced service %s in %s\n", name, namespace)
	return nil
}
