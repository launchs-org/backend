package railpack

import (
	"context"
	"fmt"
	"time"

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
	StatusInit     BuildStatus = "Init"     // Job作成済み、Pod起動待ち
	StatusRunning  BuildStatus = "Running"  // ビルド実行中
	StatusComplete BuildStatus = "Complete" // ビルド成功
	StatusFailed   BuildStatus = "Failed"   // ビルド失敗
)

func newResources(cpu, memory, disk string) corev1.ResourceRequirements {
	rl := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(cpu),
		corev1.ResourceMemory: resource.MustParse(memory),
	}
	if disk != "" {
		rl[corev1.ResourceEphemeralStorage] = resource.MustParse(disk)
	}
	return corev1.ResourceRequirements{Limits: rl}
}

// createJob は BuildConfig を元に Kubernetes Job を作成し jobID を返す。
func createJob(ctx context.Context, cs *kubernetes.Clientset, ns string, cfg BuildConfig) (string, error) {
	r := cfg.Resources

	buildRes := newResources(r.BuildCPU, r.BuildMemory, r.BuildDisk)
	initRes := newResources(r.InitCPU, r.InitMemory, "")

	// emptyDir は BuildDisk の 90%
	diskQty := resource.MustParse(r.BuildDisk)
	emptyDirSize := resource.NewMilliQuantity(
		int64(float64(diskQty.Value())*0.9)*1000,
		resource.BinarySI,
	)

	jobName := "railpack-" + cfg.JobID
	deadline := int64(cfg.Timeout.Seconds())

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ns,
			Labels:    map[string]string{"app": "railpack", "build-job-id": cfg.JobID,"managed-by": "launchs","launchs-managed": "true"},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: pointer.Int32(600),
			ActiveDeadlineSeconds:   &deadline,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"build-job-id": cfg.JobID, "managed-by": "launchs"},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes:       buildVolumes(emptyDirSize),
					InitContainers: []corev1.Container{
						// 1. Harbor TLS 証明書をシステム CA バンドルと結合して配置
						//    + レジストリ認証用 config.json を生成
						setupEnvContainer(cfg, initRes),
						// 2. Git リポジトリをクローン
						gitCloneContainer(cfg, initRes),
						// 3. railpack prepare でビルドプランを生成
						railpackPrepareContainer(cfg, initRes),
					},
					Containers: []corev1.Container{
						// buildctl でビルドしてそのままレジストリへプッシュ
						buildctlContainer(cfg, buildRes),
					},
				},
			},
		},
	}

	_, err := cs.BatchV1().Jobs(ns).Create(ctx, job, metav1.CreateOptions{})
	return cfg.JobID, err
}

// ── ボリューム ──────────────────────────────────────────────

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

// ── InitContainer ───────────────────────────────────────────

// setupEnvContainer は以下を一括で行う InitContainer:
//  1. Harbor の TLS 証明書チェーンを取得
//  2. moby/buildkit イメージのシステム CA バンドルと結合して /workspace/certs/ca-bundle.crt に保存
//     (buildctl-daemonless rootless では --opt registry.X.ca が効かないため
//     SSL_CERT_FILE による指定が唯一の有効な回避策)
//  3. レジストリ認証情報を /workspace/.docker/config.json に書き出す
//     (buildctl は DOCKER_CONFIG でこのディレクトリを参照する)
//
// 参照: https://github.com/moby/buildkit/issues/6068
func setupEnvContainer(cfg BuildConfig, res corev1.ResourceRequirements) corev1.Container {
	// システム CA バンドルのパスは moby/buildkit イメージと同じものを使う
	const systemCABundle = "/etc/ssl/certs/ca-certificates.crt"

	script := fmt.Sprintf(`
set -e

# ── 証明書 ──────────────────────────────────────────────────
mkdir -p /workspace/certs
# システム CA バンドルをベースにコピーし Harbor の証明書チェーンを追記する
cp %s /workspace/certs/ca-bundle.crt
openssl s_client \
  -connect "%s:443" \
  -showcerts \
  </dev/null 2>/dev/null \
| awk '/-----BEGIN CERTIFICATE-----/{p=1} p{print} /-----END CERTIFICATE-----/{p=0}' \
>> /workspace/certs/ca-bundle.crt
echo "証明書を取得しました: $(grep -c 'BEGIN CERTIFICATE' /workspace/certs/ca-bundle.crt) 件"

# ── 認証情報 ─────────────────────────────────────────────────
mkdir -p /workspace/.docker
AUTH_B64=$(printf '%%s:%%s' "${REGISTRY_USERNAME}" "${REGISTRY_PASSWORD}" | base64 | tr -d '\n')
printf '{"auths":{"%s":{"auth":"%%s"}}}\n' "${AUTH_B64}" > /workspace/.docker/config.json
echo "config.json を生成しました"
`,
		systemCABundle,
		cfg.RegistryHost,
		cfg.RegistryHost,
	)

	return corev1.Container{
		Name:      "setup-env",
		Image:     "moby/buildkit:v0.27.0-rootless", // CA バンドルのパスを buildctl と合わせる
		Resources: res,
		Env: []corev1.EnvVar{
			{Name: "REGISTRY_USERNAME", Value: cfg.RegistryUsername},
			{Name: "REGISTRY_PASSWORD", Value: cfg.RegistryPassword},
		},
		Command: []string{"sh", "-c"},
		Args:    []string{script},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: "/workspace"},
		},
	}
}

func gitCloneContainer(cfg BuildConfig, res corev1.ResourceRequirements) corev1.Container {
	return corev1.Container{
		Name:      "git-clone",
		Image:     "alpine/git:latest",
		Resources: res,
		Env: []corev1.EnvVar{
			{Name: "GIT_REPO", Value: cfg.GitRepo},
			{Name: "GIT_BRANCH", Value: cfg.GitBranch},
		},
		Command: []string{"sh", "-c"},
		Args:    []string{"mkdir -p /workspace/repo && git clone --depth=1 --branch=${GIT_BRANCH} ${GIT_REPO} /workspace/repo && chmod -R 777 /workspace"},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: "/workspace"},
		},
	}
}

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

// ── メインコンテナ ──────────────────────────────────────────

// buildctlContainer はビルドしてそのままレジストリへ直接プッシュする。
// キャッシュも同じレジストリへ export/import する。
//
// TLS: setupEnvContainer が生成した ca-bundle.crt を SSL_CERT_FILE で指定する。
//
//	buildctl-daemonless rootless では --opt registry.X.ca が効かないため
//	SSL_CERT_FILE が唯一の有効な回避策。(github.com/moby/buildkit/issues/6068)
//
// 認証: setupEnvContainer が生成した config.json を DOCKER_CONFIG で参照する。
func buildctlContainer(cfg BuildConfig, res corev1.ResourceRequirements) corev1.Container {
	imageRef := fmt.Sprintf("%s/%s/%s:%s",
		cfg.RegistryHost, cfg.RegistryProject, cfg.ImageName, cfg.ImageTag)

	insecureFlag := ""
	if cfg.RegistryInsecure {
		insecureFlag = ",registry.insecure=true"
	}

	buildArgs := fmt.Sprintf(
		`buildctl-daemonless.sh build \
  --local context=/workspace/repo/${BUILD_CONTEXT_SUBDIR} \
  --local dockerfile=/workspace \
  --frontend=gateway.v0 \
  --opt source=ghcr.io/railwayapp/railpack-frontend \
  --opt compression=zstd \
  --opt push-parallelism=1 \
  --opt compression-level=22 \
  --output "type=image,name=%s,push=true%s" \
  --export-cache type=inline \
  --import-cache type=registry,ref=%s%s`,
		imageRef, insecureFlag,
		imageRef, insecureFlag,
	)

	envVars := []corev1.EnvVar{
		{Name: "BUILD_CONTEXT_SUBDIR", Value: cfg.Subdir},
		{Name: "BUILDKITD_FLAGS", Value: "--oci-worker-no-process-sandbox"},
		// setupEnvContainer が生成した config.json のディレクトリを指定
		{Name: "DOCKER_CONFIG", Value: "/workspace/.docker"},
	}
	// Insecure=false のときのみ SSL_CERT_FILE を設定する
	if !cfg.RegistryInsecure {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "SSL_CERT_FILE",
			Value: "/workspace/certs/ca-bundle.crt",
		})
	}

	return corev1.Container{
		Name:       "buildctl",
		Image:      "moby/buildkit:v0.27.0-rootless",
		Resources:  res,
		WorkingDir: "/workspace",
		Env:        envVars,
		Command:    []string{"sh", "-c"},
		Args:       []string{buildArgs},
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

// ── Job 管理ヘルパー ────────────────────────────────────────

func deleteJob(ctx context.Context, cs *kubernetes.Clientset, ns, jobID string) error {
	prop := metav1.DeletePropagationForeground
	return cs.BatchV1().Jobs(ns).Delete(ctx,
		"railpack-"+jobID,
		metav1.DeleteOptions{PropagationPolicy: &prop},
	)
}

// getJobStatus は指定した jobID の現在の BuildStatus を返す。
func getJobStatus(ctx context.Context, cs *kubernetes.Clientset, ns, jobID string) (BuildStatus, error) {
	job, err := cs.BatchV1().Jobs(ns).Get(ctx, "railpack-"+jobID, metav1.GetOptions{})
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

func waitForPod(ctx context.Context, cs *kubernetes.Clientset, ns, jobID string) (*corev1.Pod, error) {
	for range make([]struct{}, 30) {
		pods, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
			LabelSelector: "build-job-id=" + jobID,
		})
		if err == nil && len(pods.Items) > 0 {
			return &pods.Items[0], nil
		}
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("pod not found for job %s", jobID)
}
