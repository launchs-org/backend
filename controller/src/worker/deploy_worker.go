package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	"k8s.io/client-go/kubernetes"
)

var _ river.Worker[jobs.DeployJobArgs] = (*DeployWorker)(nil)

// DeployWorker は deploy ジョブを処理し、K8s Deployment を作成・更新します。
type DeployWorker struct {
	river.WorkerDefaults[jobs.DeployJobArgs]
}

func (w *DeployWorker) Timeout(*river.Job[jobs.DeployJobArgs]) time.Duration {
	return 10 * time.Minute
}

func (w *DeployWorker) Work(ctx context.Context, job *river.Job[jobs.DeployJobArgs]) error {
	payload := job.Args
	fmt.Printf("[deploy-worker] processing job %d (container_id: %s)\n", job.ID, payload.ContainerID)

	if err := processDeployTask(ctx, payload); err != nil {
		fmt.Printf("[deploy-worker] job %d failed: %v\n", job.ID, err)
		return err
	}
	return nil
}

// processDeployTask は Deployment の作成または更新を行います。
func processDeployTask(ctx context.Context, payload jobs.DeployJobArgs) error {
	// コンテナ情報を取得（Volumes, Service, Ingress を含む）
	container, err := model.GetContainerByID(payload.ContainerID)
	if err != nil {
		return fmt.Errorf("container not found: %w", err)
	}

	// プロジェクト情報から Namespace を取得
	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}

	clientset := database.K8sClientset.(*kubernetes.Clientset)
	namespace := project.Namespace

	// デプロイ開始状態を DB に反映
	model.UpdateContainerStatus(payload.ContainerID, "Deploying")

	// Deployment リソース定義を組み立て
	deployment, err := buildDeployment(container, project.Namespace, payload.ImageRef)
	if err != nil {
		model.UpdateContainerStatus(payload.ContainerID, "Failed")
		return fmt.Errorf("failed to build deployment spec: %w", err)
	}

	// 既存 Deployment があれば更新、なければ新規作成
	existing, err := clientset.AppsV1().Deployments(namespace).Get(ctx, container.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			model.UpdateContainerStatus(payload.ContainerID, "Failed")
			return fmt.Errorf("failed to get deployment: %w", err)
		}
		// 新規作成
		if _, err := clientset.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
			model.UpdateContainerStatus(payload.ContainerID, "Failed")
			return fmt.Errorf("failed to create deployment: %w", err)
		}
		fmt.Printf("[deploy-worker] created deployment %s in %s\n", container.Name, namespace)
	} else {
		// 既存の ResourceVersion を引き継いで更新
		deployment.ResourceVersion = existing.ResourceVersion
		if _, err := clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{}); err != nil {
			model.UpdateContainerStatus(payload.ContainerID, "Failed")
			return fmt.Errorf("failed to update deployment: %w", err)
		}
		fmt.Printf("[deploy-worker] updated deployment %s in %s\n", container.Name, namespace)
	}

	return nil
}

// buildDeployment は Container モデルから K8s Deployment を組み立てます。
// launchs-managed=true と container-id ラベルを付与して watcher の監視対象にします。
func buildDeployment(container *model.Container, namespace, imageRef string) (*appsv1.Deployment, error) {
	replicas := int32(container.Replicas)
	if replicas <= 0 {
		replicas = 1
	}

	// 環境変数を JSON から K8s EnvVar スライスに変換
	envVars, err := parseEnvVars(container.EnvVars)
	if err != nil {
		return nil, fmt.Errorf("invalid env_vars: %w", err)
	}

	// リソース制限を JSON から変換
	resourceReqs, err := parseResources(container.Resources)
	if err != nil {
		return nil, fmt.Errorf("invalid resources: %w", err)
	}

	// Volume マウント設定を組み立て
	volumeMounts, volumes := buildVolumeConfig(container.Volumes)

	// watcher が container-id ラベルで DB レコードと紐付ける。
	// launchs-managed=true が付いた Deployment のみ監視対象になる。
	labels := map[string]string{
		"app":             container.Name,
		"container-id":    container.ID,
		"launchs-managed": "true",
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
				ObjectMeta: metav1.ObjectMeta{
					// Pod にも launchs-managed を付与して Pod Watch でも識別できるようにする
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            container.Name,
							Image:           imageRef,
							Env:             envVars,
							Resources:       resourceReqs,
							VolumeMounts:    volumeMounts,
							ImagePullPolicy: corev1.PullAlways,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}, nil
}

// parseEnvVars は JSON 形式の環境変数文字列を K8s EnvVar スライスに変換します。
// 入力形式: {"KEY": "VALUE", ...} または [] （空の場合は空スライスを返す）
func parseEnvVars(envVarsJSON string) ([]corev1.EnvVar, error) {
	if envVarsJSON == "" || envVarsJSON == "{}" || envVarsJSON == "[]" {
		return nil, nil
	}

	// map 形式 {"KEY": "VALUE"} をパース
	var envMap map[string]string
	if err := json.Unmarshal([]byte(envVarsJSON), &envMap); err != nil {
		return nil, err
	}

	envVars := make([]corev1.EnvVar, 0, len(envMap))
	for k, v := range envMap {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}
	return envVars, nil
}

// ResourceConfig はリソース制限の JSON 形式を表します。
type ResourceConfig struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// parseResources は JSON 形式のリソース設定を K8s ResourceRequirements に変換します。
// 未設定の場合はデフォルト値（CPU: 100m, Memory: 128Mi）を使用します。
func parseResources(resourcesJSON string) (corev1.ResourceRequirements, error) {
	defaultCPU := os.Getenv("DEFAULT_CPU_LIMIT")
	if defaultCPU == "" {
		defaultCPU = "100m"
	}
	defaultMem := os.Getenv("DEFAULT_MEMORY_LIMIT")
	if defaultMem == "" {
		defaultMem = "128Mi"
	}

	cfg := ResourceConfig{CPU: defaultCPU, Memory: defaultMem}
	if resourcesJSON != "" && resourcesJSON != "{}" {
		if err := json.Unmarshal([]byte(resourcesJSON), &cfg); err != nil {
			return corev1.ResourceRequirements{}, err
		}
	}

	if cfg.CPU == "" {
		cfg.CPU = defaultCPU
	}
	if cfg.Memory == "" {
		cfg.Memory = defaultMem
	}

	return corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cfg.CPU),
			corev1.ResourceMemory: resource.MustParse(cfg.Memory),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cfg.CPU),
			corev1.ResourceMemory: resource.MustParse(cfg.Memory),
		},
	}, nil
}

// buildVolumeConfig は Container に紐づく Volume から VolumeMount と Volume スライスを生成します。
// PVC 名は "pvc-{volumeID}" の形式です。
func buildVolumeConfig(volumes []model.Volume) ([]corev1.VolumeMount, []corev1.Volume) {
	mounts := make([]corev1.VolumeMount, 0, len(volumes))
	k8sVolumes := make([]corev1.Volume, 0, len(volumes))

	for _, v := range volumes {
		// PVC 名の "-" を "_" に変換してボリューム名として使用
		volName := strings.ReplaceAll("pvc-"+v.ID, "_", "-")
		pvcName := "pvc-" + v.ID

		mounts = append(mounts, corev1.VolumeMount{
			Name:      volName,
			MountPath: v.MountPath,
		})
		k8sVolumes = append(k8sVolumes, corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
	}
	return mounts, k8sVolumes
}
