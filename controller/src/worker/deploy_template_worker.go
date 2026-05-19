package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"launchs/shared/database"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/riverqueue/river"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

var _ river.Worker[jobs.DeployTemplateJobArgs] = (*DeployTemplateWorker)(nil)

type DeployTemplateWorker struct {
	river.WorkerDefaults[jobs.DeployTemplateJobArgs]
}

func (w *DeployTemplateWorker) Timeout(*river.Job[jobs.DeployTemplateJobArgs]) time.Duration {
	return 10 * time.Minute
}

func (w *DeployTemplateWorker) Work(ctx context.Context, job *river.Job[jobs.DeployTemplateJobArgs]) error {
	p := job.Args
	fmt.Printf("[deploy-template-worker] processing job %d (container_id: %s)\n", job.ID, p.ContainerID)

	clientset := database.K8sClientset.(*kubernetes.Clientset)

	model.UpdateContainerStatus(p.ContainerID, "Deploying")

	container, err := model.GetContainerByID(p.ContainerID)
	if err != nil {
		model.UpdateContainerStatus(p.ContainerID, "Failed")
		return fmt.Errorf("container not found: %w", err)
	}

	// ボリュームがある場合は PVC が作成されるまで待機（最大 2 分）
	if p.VolumeID != "" {
		pvcName := fmt.Sprintf("pvc-%s", p.VolumeID)
		deadline := time.Now().Add(2 * time.Minute)
		for time.Now().Before(deadline) {
			_, pvcErr := clientset.CoreV1().PersistentVolumeClaims(p.Namespace).Get(ctx, pvcName, metav1.GetOptions{})
			if pvcErr == nil {
				break
			}
			if !errors.IsNotFound(pvcErr) {
				model.UpdateContainerStatus(p.ContainerID, "Failed")
				return fmt.Errorf("failed to check PVC %s: %w", pvcName, pvcErr)
			}
			fmt.Printf("[deploy-template-worker] waiting for PVC %s to be created...\n", pvcName)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
		// タイムアウト後に再確認
		if _, pvcErr := clientset.CoreV1().PersistentVolumeClaims(p.Namespace).Get(ctx, pvcName, metav1.GetOptions{}); pvcErr != nil {
			model.UpdateContainerStatus(p.ContainerID, "Failed")
			return fmt.Errorf("PVC %s not ready after 2 minutes: %w", pvcName, pvcErr)
		}
	}

	// リソース要件を組み立て
	resourceReqs := buildTemplateResources(p)

	// 環境変数をパース（deploy_worker の parseEnvVars を再利用）
	envVars, err := parseEnvVars(p.EnvVars)
	if err != nil {
		model.UpdateContainerStatus(p.ContainerID, "Failed")
		return fmt.Errorf("invalid env_vars: %w", err)
	}

	// ポート定義
	ports := buildContainerPorts(p.Ports)

	// ボリュームマウント
	volumeMounts, k8sVolumes := buildVolumeConfig(container.Volumes)

	// command / args（空の場合は省略してイメージデフォルトを使用）
	var command []string
	if p.Command != "" {
		command = []string{p.Command}
	}
	var args []string
	if p.Args != "" {
		args = strings.Fields(p.Args)
	}

	labels := map[string]string{
		"app":             container.Name,
		"container-id":    container.ID,
		"launchs-managed": "true",
	}

	replicas := int32(container.Replicas)
	if replicas <= 0 {
		replicas = 1
	}

	k8sContainer := corev1.Container{
		Name:            container.Name,
		Image:           p.ImageRef,
		Env:             envVars,
		Resources:       resourceReqs,
		VolumeMounts:    volumeMounts,
		Ports:           ports,
		ImagePullPolicy: corev1.PullAlways,
	}
	if len(command) > 0 {
		k8sContainer.Command = command
	}
	if len(args) > 0 {
		k8sContainer.Args = args
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      container.Name,
			Namespace: p.Namespace,
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
					Containers: []corev1.Container{k8sContainer},
					Volumes:    k8sVolumes,
				},
			},
		},
	}

	existing, err := clientset.AppsV1().Deployments(p.Namespace).Get(ctx, container.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			model.UpdateContainerStatus(p.ContainerID, "Failed")
			return fmt.Errorf("failed to get deployment: %w", err)
		}
		if _, err := clientset.AppsV1().Deployments(p.Namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
			model.UpdateContainerStatus(p.ContainerID, "Failed")
			return fmt.Errorf("failed to create deployment: %w", err)
		}
		fmt.Printf("[deploy-template-worker] created deployment %s in %s\n", container.Name, p.Namespace)
	} else {
		deployment.ResourceVersion = existing.ResourceVersion
		if _, err := clientset.AppsV1().Deployments(p.Namespace).Update(ctx, deployment, metav1.UpdateOptions{}); err != nil {
			model.UpdateContainerStatus(p.ContainerID, "Failed")
			return fmt.Errorf("failed to update deployment: %w", err)
		}
		fmt.Printf("[deploy-template-worker] updated deployment %s in %s\n", container.Name, p.Namespace)
	}

	// K8s Service が DB にある場合は ClusterIP Service を作成
	if container.Service != nil {
		if err := ensureK8sService(ctx, clientset, container, p.Namespace); err != nil {
			fmt.Printf("[deploy-template-worker] warning: failed to create service: %v\n", err)
		}
	}

	return nil
}

func buildTemplateResources(p jobs.DeployTemplateJobArgs) corev1.ResourceRequirements {
	cpuReq := p.CPURequest
	if cpuReq == "" {
		cpuReq = "100m"
	}
	cpuLim := p.CPULimit
	if cpuLim == "" {
		cpuLim = "500m"
	}
	memReq := p.MemoryRequest
	if memReq == "" {
		memReq = "128Mi"
	}
	memLim := p.MemoryLimit
	if memLim == "" {
		memLim = "512Mi"
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuReq),
			corev1.ResourceMemory: resource.MustParse(memReq),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuLim),
			corev1.ResourceMemory: resource.MustParse(memLim),
		},
	}
}

type portEntry struct {
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
}

func buildContainerPorts(portsJSON string) []corev1.ContainerPort {
	if portsJSON == "" || portsJSON == "[]" {
		return nil
	}
	var entries []portEntry
	if err := json.Unmarshal([]byte(portsJSON), &entries); err != nil {
		return nil
	}
	ports := make([]corev1.ContainerPort, 0, len(entries))
	for _, e := range entries {
		proto := corev1.ProtocolTCP
		if strings.ToUpper(e.Protocol) == "UDP" {
			proto = corev1.ProtocolUDP
		}
		ports = append(ports, corev1.ContainerPort{
			ContainerPort: e.Port,
			Protocol:      proto,
		})
	}
	return ports
}

type svcPortEntry struct {
	Name   string `json:"name"`
	Port   int32  `json:"port"`
	Target int    `json:"target"`
}

func ensureK8sService(ctx context.Context, clientset *kubernetes.Clientset, container *model.Container, namespace string) error {
	svc := container.Service

	var portEntries []svcPortEntry
	if svc.Ports != "" {
		if err := json.Unmarshal([]byte(svc.Ports), &portEntries); err != nil {
			return fmt.Errorf("failed to parse service ports: %w", err)
		}
	}
	var k8sPorts []corev1.ServicePort
	for _, p := range portEntries {
		k8sPorts = append(k8sPorts, corev1.ServicePort{
			Name:       p.Name,
			Port:       p.Port,
			TargetPort: intstr.FromInt(int(p.Port)),
		})
	}

	k8sSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      container.Name,
			Namespace: namespace,
			Labels:    map[string]string{"app": container.Name},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": container.Name},
			Type:     corev1.ServiceTypeClusterIP,
			Ports:    k8sPorts,
		},
	}

	_, getErr := clientset.CoreV1().Services(namespace).Get(ctx, container.Name, metav1.GetOptions{})
	if getErr != nil {
		if !errors.IsNotFound(getErr) {
			return getErr
		}
		if _, createErr := clientset.CoreV1().Services(namespace).Create(ctx, k8sSvc, metav1.CreateOptions{}); createErr != nil {
			return fmt.Errorf("failed to create k8s service: %w", createErr)
		}
		model.SetServiceStatus(container.ID, "active")
		fmt.Printf("[deploy-template-worker] created service %s in %s\n", container.Name, namespace)
	}
	return nil
}
