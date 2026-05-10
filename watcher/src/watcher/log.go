package watcher

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// logJobEvent は K8s Job のイベント発生時に構造化ログを出力します。
// ステータス・JobID・BuildJobID・イベント種別を 1 行にまとめて出力します。
func logJobEvent(eventType watch.EventType, job *batchv1.Job, buildJobID, status string) {
	jobID := job.Labels["job-uuid"]
	fmt.Printf("[job-watcher] event=%s job=%s build_job_id=%s status=%s active=%d succeeded=%d failed=%d ts=%s\n",
		eventType,
		job.Name,
		buildJobID,
		status,
		job.Status.Active,
		job.Status.Succeeded,
		job.Status.Failed,
		time.Now().Format(time.RFC3339),
	)
	_ = jobID // ラベルとして保持しているが、ログには job.Name で代替
}

// logDeploymentEvent は K8s Deployment のイベント発生時に構造化ログを出力します。
// Namespace・Deployment 名・ContainerID・ステータス・レプリカ数を 1 行にまとめて出力します。
func logDeploymentEvent(eventType watch.EventType, deploy *appsv1.Deployment, containerID, status string) {
	desired := int32(0)
	if deploy.Spec.Replicas != nil {
		desired = *deploy.Spec.Replicas
	}
	fmt.Printf("[deploy-watcher] event=%s namespace=%s deployment=%s container_id=%s status=%s desired=%d ready=%d unavailable=%d ts=%s\n",
		eventType,
		deploy.Namespace,
		deploy.Name,
		containerID,
		status,
		desired,
		deploy.Status.ReadyReplicas,
		deploy.Status.UnavailableReplicas,
		time.Now().Format(time.RFC3339),
	)
}
