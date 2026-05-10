package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"launchs/shared/database"
	"launchs/shared/job_queue/jobs"

	"github.com/riverqueue/river"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var ingressRouteGVR = schema.GroupVersionResource{
	Group:    "traefik.io",
	Version:  "v1alpha1",
	Resource: "ingressroutes",
}

var _ river.Worker[jobs.CreateIngressJobArgs] = (*CreateIngressWorker)(nil)
var _ river.Worker[jobs.UpdateIngressJobArgs] = (*UpdateIngressWorker)(nil)
var _ river.Worker[jobs.DeleteIngressJobArgs] = (*DeleteIngressWorker)(nil)

type CreateIngressWorker struct {
	river.WorkerDefaults[jobs.CreateIngressJobArgs]
}

func (w *CreateIngressWorker) Timeout(*river.Job[jobs.CreateIngressJobArgs]) time.Duration {
	return 5 * time.Minute
}

func (w *CreateIngressWorker) Work(ctx context.Context, job *river.Job[jobs.CreateIngressJobArgs]) error {
	payload := job.Args
	fmt.Printf("[create-ingress-worker] processing job %d (container_id: %s)\n", job.ID, payload.ContainerID)

	hosts := []string{payload.Subdomain}
	if payload.CustomDomain != "" && payload.CustomDomainEnabled {
		hosts = append(hosts, payload.CustomDomain)
	}

	return syncIngressRoute(ctx, payload.Namespace, payload.ContainerName, hosts, "/", payload.HttpPort)
}

type UpdateIngressWorker struct {
	river.WorkerDefaults[jobs.UpdateIngressJobArgs]
}

func (w *UpdateIngressWorker) Timeout(*river.Job[jobs.UpdateIngressJobArgs]) time.Duration {
	return 5 * time.Minute
}

func (w *UpdateIngressWorker) Work(ctx context.Context, job *river.Job[jobs.UpdateIngressJobArgs]) error {
	payload := job.Args
	fmt.Printf("[update-ingress-worker] processing job %d (container_id: %s)\n", job.ID, payload.ContainerID)

	hosts := []string{payload.Subdomain}
	if payload.CustomDomain != "" && payload.CustomDomainEnabled {
		hosts = append(hosts, payload.CustomDomain)
	}

	return syncIngressRoute(ctx, payload.Namespace, payload.ContainerName, hosts, "/", payload.HttpPort)
}

type DeleteIngressWorker struct {
	river.WorkerDefaults[jobs.DeleteIngressJobArgs]
}

func (w *DeleteIngressWorker) Timeout(*river.Job[jobs.DeleteIngressJobArgs]) time.Duration {
	return 5 * time.Minute
}

func (w *DeleteIngressWorker) Work(ctx context.Context, job *river.Job[jobs.DeleteIngressJobArgs]) error {
	payload := job.Args
	fmt.Printf("[delete-ingress-worker] processing job %d (container_id: %s)\n", job.ID, payload.ContainerID)

	err := database.K8sDynamicClient.Resource(ingressRouteGVR).Namespace(payload.Namespace).Delete(ctx, payload.ContainerName, metav1.DeleteOptions{})
	if err != nil {
		fmt.Printf("[delete-ingress-worker] warning: failed to delete ingressroute (may not exist): %v\n", err)
	}
	return nil
}

func syncIngressRoute(ctx context.Context, namespace, name string, hosts []string, path string, port int) error {
	var hostMatches []string
	for _, host := range hosts {
		hostMatches = append(hostMatches, fmt.Sprintf("Host(`%s`)", host))
	}

	matchRule := ""
	if len(hostMatches) > 1 {
		matchRule = fmt.Sprintf("(%s) && PathPrefix(`%s`)", strings.Join(hostMatches, " || "), path)
	} else if len(hostMatches) == 1 {
		matchRule = fmt.Sprintf("%s && PathPrefix(`%s`)", hostMatches[0], path)
	}

	ingressRoute := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "IngressRoute",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"entryPoints": []interface{}{"web", "websecure"},
				"routes": []interface{}{
					map[string]interface{}{
						"match": matchRule,
						"kind":  "Rule",
						"services": []interface{}{
							map[string]interface{}{
								"name": name,
								"port": port,
							},
						},
					},
				},
			},
		},
	}

	client := database.K8sDynamicClient
	res, err := client.Resource(ingressRouteGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		_, err = client.Resource(ingressRouteGVR).Namespace(namespace).Create(ctx, ingressRoute, metav1.CreateOptions{})
		return err
	}

	ingressRoute.SetResourceVersion(res.GetResourceVersion())
	_, err = client.Resource(ingressRouteGVR).Namespace(namespace).Update(ctx, ingressRoute, metav1.UpdateOptions{})
	return err
}
