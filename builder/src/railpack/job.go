package railpack

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
)

// BuildStatus はビルドジョブの現在の状態を表します。
type BuildStatus string

const (
	StatusInit      BuildStatus = "Init"      // Job作成済み、Pod起動待ち
	StatusRunning   BuildStatus = "Running"   // ビルド実行中
	StatusComplete  BuildStatus = "Complete"  // ビルド成功
	StatusFailed    BuildStatus = "Failed"    // ビルド失敗
)

// createJob は BuildConfig を元に Kubernetes Job を作成し、jobID を返します。
func createJob(ctx context.Context, clientset *kubernetes.Clientset, cfg BuildConfig) (string, error) {
	resources := cfg.Resources

	buildResources := newResourceRequirements(resources.BuildCPU, resources.BuildMemory, resources.BuildDisk)
	initResources  := newResourceRequirements(resources.InitCPU, resources.InitMemory, "")
	pushResources  := newResourceRequirements(resources.PushCPU, resources.PushMemory, "")

	// emptyDir のサイズ制限は BuildDisk の 90% に設定
	diskQuantity := resource.MustParse(resources.BuildDisk)
	emptyDirSize := resource.NewMilliQuantity(
		int64(float64(diskQuantity.Value())*0.9)*1000,
		resource.BinarySI,
	)

	jobID := cfg.JobID
	if jobID == "" {
		jobID = uuid.New().String()
	}
	jobName  := "railpack-" + jobID
	deadline := int64(cfg.Timeout.Seconds())

	// Job と Pod の両方に launchs-managed=true を付与することで
	// watcher が LabelSelector で効率的に絞り込めるようにする。
	// build-job-id は watcher 側で DB の BuildJob と紐付けるために使用する。
	managedLabels := map[string]string{
		"app":             "railpack",
		"job-uuid":        jobID,
		"launchs-managed": "true",
		"build-job-id":    cfg.JobID, // "bj-xxx-yyy" 形式（"-" 区切り）
	}
	podLabels := map[string]string{
		"job-uuid":        jobID,
		"launchs-managed": "true",
		"build-job-id":    cfg.JobID,
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: cfg.Namespace,
			Labels:    managedLabels,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: pointer.Int32(600),
			ActiveDeadlineSeconds:   &deadline,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes:       buildVolumes(emptyDirSize),
					InitContainers: []corev1.Container{
						gitCloneContainer(cfg, initResources),
						railpackPrepareContainer(cfg, initResources),
					},
					Containers: []corev1.Container{
						buildctlContainer(cfg, buildResources),
						tarPushContainer(cfg, pushResources, jobID),
					},
				},
			},
		},
	}

	_, err := clientset.BatchV1().Jobs(cfg.Namespace).Create(ctx, job, metav1.CreateOptions{})
	return jobID, err
}

// deleteJob は指定した jobID の Kubernetes Job を強制削除します。
func deleteJob(ctx context.Context, clientset *kubernetes.Clientset, namespace, jobID string) error {
	propagation := metav1.DeletePropagationForeground
	return clientset.BatchV1().Jobs(namespace).Delete(
		ctx,
		"railpack-"+jobID,
		metav1.DeleteOptions{PropagationPolicy: &propagation},
	)
}

// getJobStatus は指定した jobID の現在の BuildStatus を返します。
func getJobStatus(ctx context.Context, clientset *kubernetes.Clientset, namespace, jobID string) (BuildStatus, error) {
	job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, "railpack-"+jobID, metav1.GetOptions{})
	if err != nil {
		return StatusFailed, err
	}
	switch {
	case job.Status.Succeeded > 0:
		return StatusComplete, nil
	case job.Status.Failed > 0:
		return StatusFailed, nil
	case job.Status.Active > 0:
		return StatusRunning, nil
	default:
		return StatusInit, nil
	}
}

// waitForPod は jobID に対応する Pod が現れるまで最大 30 秒待機します。
func waitForPod(ctx context.Context, clientset *kubernetes.Clientset, namespace, jobID string) (*corev1.Pod, error) {
	for range make([]struct{}, 30) {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "job-uuid=" + jobID,
		})
		if err == nil && len(pods.Items) > 0 {
			return &pods.Items[0], nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("job %s の Pod が見つかりません", jobID)
}


// ── リソース設定ヘルパー ────────────────────────────────────

// newResourceRequirements は CPU・メモリ・ディスクから ResourceRequirements を作成します。
func newResourceRequirements(cpu, memory, disk string) corev1.ResourceRequirements {
	resourceList := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(cpu),
		corev1.ResourceMemory: resource.MustParse(memory),
	}
	if disk != "" {
		resourceList[corev1.ResourceEphemeralStorage] = resource.MustParse(disk)
	}
	return corev1.ResourceRequirements{Limits: resourceList}
}

// ── ボリューム定義 ──────────────────────────────────────────

func buildVolumes(emptyDirSize *resource.Quantity) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: emptyDirSize},
			},
		},
		{
			Name:         "socket",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
	}
}

// ── InitContainer 定義 ──────────────────────────────────────

// gitCloneContainer は Git リポジトリをクローンする InitContainer を返します。
// cfg.GitSubmodules が true の場合はサブモジュールも再帰的にクローンします。
func gitCloneContainer(cfg BuildConfig, res corev1.ResourceRequirements) corev1.Container {
	// サブモジュールの有無でコマンドを切り替える
	cloneCommand := "git clone --verbose --depth=1 --branch=${GIT_BRANCH} ${GIT_REPO} /workspace/repo"
	if cfg.GitSubmodules {
		// --recurse-submodules でサブモジュールも同時にクローン
		// --shallow-submodules でサブモジュールも depth=1 に抑える
		cloneCommand = "git clone --verbose --depth=1 --branch=${GIT_BRANCH} --recurse-submodules --shallow-submodules ${GIT_REPO} /workspace/repo"
	}

	return corev1.Container{
		Name:      "git-clone",
		Image:     "alpine/git:latest",
		Resources: res,
		Env: []corev1.EnvVar{
			{Name: "GIT_REPO", Value: cfg.GitRepo},
			{Name: "GIT_BRANCH", Value: cfg.GitBranch},
		},
		Command: []string{"sh", "-c"},
		Args:    []string{"mkdir -p /workspace/repo && " + cloneCommand + " && chmod -R 777 /workspace"},
		// SecurityContext: &corev1.SecurityContext{
		// 	RunAsNonRoot:             pointer.Bool(true),
		// 	RunAsUser:                pointer.Int64(1000),
		// 	RunAsGroup:               pointer.Int64(1000),
		// 	ReadOnlyRootFilesystem:   pointer.Bool(true),
		// 	AllowPrivilegeEscalation: pointer.Bool(false),
		// 	Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		// 	SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		// },
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: "/workspace"},
		},
	}
}

// railpackPrepareContainer は railpack の prepare コマンドを実行する InitContainer を返します。
// railpack バイナリの実行に root 権限が必要なため、SecurityContext は設定しません。
func railpackPrepareContainer(cfg BuildConfig, res corev1.ResourceRequirements) corev1.Container {
	return corev1.Container{
		Name:       "railpack",
		Image:      "ghcr.io/launchs-org/railpack-container:latest",
		Resources:  res,
		WorkingDir: "/workspace/repo",
		Env: []corev1.EnvVar{
			{Name: "BUILD_CONTEXT_SUBDIR", Value: cfg.Subdir},
		},
		Command: []string{"sh", "-c"},
		Args:    []string{"cd ${BUILD_CONTEXT_SUBDIR} && railpack prepare . --plan-out /workspace/railpack-plan.json"},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: "/workspace"},
		},
	}
}

// ── メインコンテナ定義 ──────────────────────────────────────

// buildctlContainer は BuildKit によるイメージビルドを行うメインコンテナを返します。
// レジストリキャッシュ・認証は使用しません。出力は /workspace/output.tar に保存します。
func buildctlContainer(cfg BuildConfig, res corev1.ResourceRequirements) corev1.Container {
	return corev1.Container{
		Name:       "buildctl",
		Image:      "moby/buildkit:master-rootless",
		Resources:  res,
		WorkingDir: "/workspace",
		Env: []corev1.EnvVar{
			{Name: "BUILD_CONTEXT_SUBDIR", Value: cfg.Subdir},
			{Name: "BUILDKITD_FLAGS", Value: "--oci-worker-no-process-sandbox"},
		},
		Command: []string{"sh", "-c"},
		Args: []string{
			`buildctl-daemonless.sh build \
  --local context=/workspace/repo/${BUILD_CONTEXT_SUBDIR} \
  --local dockerfile=/workspace \
  --frontend=gateway.v0 \
  --opt source=ghcr.io/railwayapp/railpack-frontend \
  --output "type=docker,dest=/workspace/output.tar" && \
touch /workspace/build.done`,
		},
		SecurityContext: &corev1.SecurityContext{
			SeccompProfile:  &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined},
			AppArmorProfile: &corev1.AppArmorProfile{Type: corev1.AppArmorProfileTypeUnconfined},
			RunAsUser:       pointer.Int64(1000),
			RunAsGroup:      pointer.Int64(1000),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "socket", MountPath: "/run/buildkit"},
			{Name: "workspace", MountPath: "/workspace"},
		},
	}
}

// tarPushContainer はビルド成果物の tar を受け取り側サーバーへ送信するコンテナを返します。
func tarPushContainer(cfg BuildConfig, res corev1.ResourceRequirements, jobID string) corev1.Container {
	return corev1.Container{
		Name:      "tar-push",
		Image:     "alpine:latest",
		Resources: res,
		Env: []corev1.EnvVar{
			{Name: "UPLOAD_URL", Value: cfg.UploadEndpoint},
			{Name: "UPLOAD_TOKEN", Value: cfg.UploadToken},
			{Name: "JOB_ID", Value: jobID},
			{Name: "IMAGE_NAME", Value: cfg.ImageName},
			{Name: "IMAGE_TAG", Value: cfg.ImageTag},
		},
		Command: []string{"sh", "-c"},
		Args: []string{
			`apk add --no-cache curl
until [ -f /workspace/build.done ]; do sleep 3; done
curl -k --fail \
  -X POST "${UPLOAD_URL}" \
  -H "Authorization: Bearer ${UPLOAD_TOKEN}" \
  -H "X-Job-Id: ${JOB_ID}" \
  -H "X-Image-Name: ${IMAGE_NAME}" \
  -H "X-Image-Tag: ${IMAGE_TAG}" \
  -H "Content-Type: application/octet-stream" \
  -H "Transfer-Encoding: chunked" \
  --data-binary @/workspace/output.tar`,
		},
		// SecurityContext: &corev1.SecurityContext{
		// 	RunAsNonRoot:             pointer.Bool(true),
		// 	RunAsUser:                pointer.Int64(1000),
		// 	RunAsGroup:               pointer.Int64(1000),
		// 	ReadOnlyRootFilesystem:   pointer.Bool(false), // apk install のため書き込みが必要
		// 	AllowPrivilegeEscalation: pointer.Bool(false),
		// 	Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		// 	SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		// },
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: "/workspace"},
		},
	}
}
