package worker

import (
	"context"
	"fmt"
	"time"

	"launchs/shared/database"
	"launchs/shared/job_queue/jobs"

	"github.com/riverqueue/river"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ river.Worker[jobs.CreateProjectJobArgs] = (*CreateProjectWorker)(nil)

type CreateProjectWorker struct {
	river.WorkerDefaults[jobs.CreateProjectJobArgs]
}

func (w *CreateProjectWorker) Timeout(*river.Job[jobs.CreateProjectJobArgs]) time.Duration {
	return 5 * time.Minute
}

func (w *CreateProjectWorker) Work(ctx context.Context, job *river.Job[jobs.CreateProjectJobArgs]) error {
	payload := job.Args
	fmt.Printf("[create-project-worker] processing job %d (project_id: %s)\n", job.ID, payload.ProjectID)

	namespace := payload.Namespace

	nsSpec := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"managed-by": "launchs",
				"project-id": payload.ProjectID,
			},
		},
	}
	if _, err := database.K8sClientset.CoreV1().Namespaces().Create(ctx, nsSpec, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", namespace, err)
	}

	netPol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-traefik-cloudflared-local",
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "traefik",
								},
							},
						},
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "cloudflared",
								},
							},
						},
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": namespace,
								},
							},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{},
			},
		},
	}
	_, _ = database.K8sClientset.NetworkingV1().NetworkPolicies(namespace).Create(ctx, netPol, metav1.CreateOptions{})

	fmt.Printf("[create-project-worker] created namespace and network policy for project %s\n", payload.ProjectID)
	return nil
}
