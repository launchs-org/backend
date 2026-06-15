package k8s

import (
	"app/models"
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// buildJobTTLSeconds はビルド Job 完了後に k8s が自動削除するまでの秒数
const buildJobTTLSeconds = int32(600) // 10 分

// CreateBuildJob はビルドタイプに応じた k8s Job を作成する
func CreateBuildJob(
	ctx context.Context,
	client kubernetes.Interface,
	buildData *models.DeploymentBuild,
	deploymentData *models.Deployment,
	namespace string,
	harborEndpoint string,
	harborRobotName string,
	harborRobotSecret string,
	pushedImageURL string,
) (string, error) {
	jobName := fmt.Sprintf("build-%s", buildData.ID) // ジョブ名をビルド ID から生成する

	jobSpec, err := buildJobSpec(buildData, deploymentData, harborEndpoint, harborRobotName, harborRobotSecret, pushedImageURL) // ビルドタイプに応じた Job Spec を生成する
	if err != nil {
		return "", err // Job Spec 生成エラーを返す
	}

	ttlSeconds := buildJobTTLSeconds // 完了後 TTL を定数から取得する
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,   // ジョブ名を設定する
			Namespace: namespace, // namespace を設定する
			Labels: map[string]string{
				"launchs.org/build-id":      buildData.ID,      // ビルド ID ラベルを設定する
				"launchs.org/deployment-id": deploymentData.ID, // デプロイメント ID ラベルを設定する
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttlSeconds, // 完了後 TTL を設定する
			Template: corev1.PodTemplateSpec{
				Spec: jobSpec, // Pod Spec を設定する
			},
		},
	}

	_, err = client.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{}) // k8s に Job を作成する
	if err != nil {
		return "", err // Job 作成エラーを返す
	}
	return jobName, nil // 作成したジョブ名を返す
}

// DeleteBuildJob は jobName に対応する k8s Job を削除する
func DeleteBuildJob(ctx context.Context, client kubernetes.Interface, namespace, jobName string) error {
	propagationPolicy := metav1.DeletePropagationForeground                                                                             // Pod も連動して削除する
	return client.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}) // Job を削除する
}

// buildJobSpec はビルドタイプに応じた PodSpec を生成する
func buildJobSpec(
	buildData *models.DeploymentBuild,
	deploymentData *models.Deployment,
	harborEndpoint string,
	harborRobotName string,
	harborRobotSecret string,
	pushedImageURL string,
) (corev1.PodSpec, error) {
	commonEnv := []corev1.EnvVar{
		{Name: "GITHUB_REPO_URL", Value: buildData.CommitSHA},      // GitHub リポジトリ URL を環境変数として渡す
		{Name: "GIT_CLONE_URL", Value: deploymentData.GithubRepoURL}, // clone URL を環境変数として渡す
		{Name: "GIT_BRANCH", Value: buildData.Branch},               // ブランチ名を環境変数として渡す
		{Name: "GIT_COMMIT_SHA", Value: buildData.CommitSHA},        // コミット SHA を環境変数として渡す
		{Name: "BUILD_DIRECTORY", Value: buildData.Directory},       // ビルドディレクトリを環境変数として渡す
		{Name: "HARBOR_ENDPOINT", Value: harborEndpoint},            // Harbor エンドポイントを環境変数として渡す
		{Name: "HARBOR_ROBOT_NAME", Value: harborRobotName},         // Harbor robot アカウント名を環境変数として渡す
		{Name: "HARBOR_ROBOT_SECRET", Value: harborRobotSecret},     // Harbor robot シークレットを環境変数として渡す
		{Name: "PUSH_IMAGE_URL", Value: pushedImageURL},             // push 先のイメージ URL を環境変数として渡す
	}

	dockerfileEnv := append(commonEnv, corev1.EnvVar{ // Dockerfile パスを追加する
		Name:  "DOCKERFILE_PATH",
		Value: buildData.DockerfilePath,
	})
	return corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever, // Job は失敗してもリスタートしない
		Containers: []corev1.Container{
			{
				Name:  "builder",                         // コンテナ名を設定する
				Image: "launchs/dockerfile-builder:latest", // Dockerfile ビルダーイメージを使用する
				Env:   dockerfileEnv,                     // 環境変数を設定する
			},
		},
	}, nil
}
