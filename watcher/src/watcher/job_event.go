package watcher

import (
	"context"
	"fmt"
	"os"
	"time"

	"launchs/shared/job_queue"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/redis/go-redis/v9"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// handleJobEvent は K8s Job の Watch イベントを受け取り、DB と Redis を更新します。
func handleJobEvent(ctx context.Context, clientset *kubernetes.Clientset, redisClient *redis.Client, namespace string, event watch.Event) error {
	job, ok := event.Object.(*batchv1.Job)
	if !ok {
		return nil
	}

	// ラベル "build-job-id" から BuildJob ID を取得する（優先）。
	// ラベルがない場合は Job 名から変換してフォールバック。
	buildJobID := job.Labels["build-job-id"]
	if buildJobID == "" {
		buildJobID = jobNameToBuildJobID(job.Name)
	}
	if buildJobID == "" {
		return nil
	}

	// Redis Pub/Sub チャンネル名（フロントエンドがこのチャンネルを購読する）
	redisChannel := fmt.Sprintf("stream:job:%s:%s", namespace, job.Name)

	switch event.Type {
	case watch.Added, watch.Modified:
		if err := onJobAddedOrModified(ctx, clientset, redisClient, namespace, job, buildJobID, redisChannel, event.Type); err != nil {
			return err
		}
	case watch.Deleted:
		// Job 削除は TTL により K8s が自動で行うため何もしない
		fmt.Printf("[job-watcher] event=DELETED job=%s build_job_id=%s\n", job.Name, buildJobID)
	}

	return nil
}

// onJobAddedOrModified は Job の追加・変更イベントを処理します。
// ステータスが Running になったときにログストリームを開始し、
// 完了または失敗したときに DB と Container ステータスを更新します。
func onJobAddedOrModified(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	redisClient *redis.Client,
	namespace string,
	job *batchv1.Job,
	buildJobID string,
	redisChannel string,
	eventType watch.EventType,
) error {
	status := determineJobStatus(job)
	updates := map[string]interface{}{"status": status}

	// Job 状態変化を構造化ログに出力
	logJobEvent(eventType, job, buildJobID, status)

	switch status {
	case "Running":
		// 実行開始時刻を記録し、ログストリームをバックグラウンドで開始
		updates["started_at"] = time.Now()
		model.UpdateBuildJobStatus(buildJobID, updates)
		go streamJobLogs(ctx, clientset, redisClient, namespace, job.Name, buildJobID, redisChannel)

	case "Success", "Failed":
		// 完了時刻を記録して DB を更新
		updates["finished_at"] = time.Now()
		model.UpdateBuildJobStatus(buildJobID, updates)

		// Container のステータスも連動して更新
		syncContainerStatus(buildJobID, status)

	default:
		// Queued などその他の状態はステータスのみ更新
		model.UpdateBuildJobStatus(buildJobID, updates)
	}

	return nil
}

// syncContainerStatus は BuildJob の結果に応じて Container のステータスを更新します。
// 成功なら "Deploying" に遷移して deploy ジョブをキューに追加します。
// 失敗なら "Failed" に遷移します。
func syncContainerStatus(buildJobID, jobStatus string) {
	buildJob, err := model.GetBuildJobByID(buildJobID)
	if err != nil {
		fmt.Printf("[job-watcher] failed to get build job %s: %v\n", buildJobID, err)
		return
	}

	if jobStatus != "Success" {
		model.UpdateContainerStatus(buildJob.ContainerID, "Failed")
		return
	}

	model.UpdateContainerStatus(buildJob.ContainerID, "Deploying")

	registryHost := os.Getenv("REGISTRY_HOST")
	if registryHost == "" {
		registryHost = "172.33.0.1"
	}
	registryProject := os.Getenv("REGISTRY_PROJECT")
	if registryProject == "" {
		registryProject = "launchs"
	}
	imageRef := fmt.Sprintf("%s/%s/%s:%s",
		registryHost, registryProject, buildJob.ContainerID, buildJob.ImageID)

	ctx := context.Background()
	if err := job_queue.Enqueue(ctx, jobs.DeployJobArgs{
		ContainerID: buildJob.ContainerID,
		ImageRef:    imageRef,
		BuildJobID:  buildJobID,
	}, nil); err != nil {
		fmt.Printf("[job-watcher] failed to enqueue deploy job for container %s: %v\n", buildJob.ContainerID, err)
	} else {
		fmt.Printf("[job-watcher] enqueued deploy job for container %s\n", buildJob.ContainerID)
	}
}

// determineJobStatus は Job の Conditions と Active/Succeeded/Failed カウントから
// アプリケーション用のステータス文字列を返します。
func determineJobStatus(job *batchv1.Job) string {
	// Conditions を優先チェック（K8s が明示的に Complete/Failed を設定した場合）
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return "Success"
		}
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return "Failed"
		}
	}

	// Conditions が付く前の過渡期はカウントで判断
	switch {
	case job.Status.Active > 0:
		return "Running"
	case job.Status.Succeeded > 0:
		return "Success"
	case job.Status.Failed > 0:
		return "Failed"
	default:
		return "Queued"
	}
}
