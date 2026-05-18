package watcher

import (
	"context"
	"fmt"
	"os"
	"time"

	"launchs/shared/job_queue"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// handleJobEvent は K8s Job の Watch イベントを受け取り、DB を更新します。
func handleJobEvent(ctx context.Context, clientset *kubernetes.Clientset, namespace string, event watch.Event, streamedJobs map[string]bool) error {
	job, ok := event.Object.(*batchv1.Job)
	if !ok {
		return nil
	}

	buildJobID := job.Labels["build-job-id"]
	if buildJobID == "" {
		fmt.Printf("[job-watcher] event=%s job=%s build_job_id=%s\n", event.Type, job.Name, buildJobID)
		return nil
	}

	switch event.Type {
	case watch.Added, watch.Modified:
		fmt.Printf("[job-watcher] event=%s job=%s build_job_id=%s\n", event.Type, job.Name, buildJobID)
		if err := onJobAddedOrModified(ctx, clientset, namespace, job, buildJobID, event.Type, streamedJobs); err != nil {
			return err
		}
	case watch.Deleted:
		fmt.Printf("[job-watcher] event=DELETED job=%s build_job_id=%s\n", job.Name, buildJobID)
	}

	return nil
}

// onJobAddedOrModified は Job の追加・変更イベントを処理します。
func onJobAddedOrModified(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	namespace string,
	job *batchv1.Job,
	buildJobID string,
	eventType watch.EventType,
	streamedJobs map[string]bool,
) error {
	status := determineJobStatus(job)
	updates := map[string]interface{}{"status": status}

	logJobEvent(eventType, job, buildJobID, status)

	switch status {
	case "Running":
		fmt.Printf("[job-watcher] event=%s job=%s build_job_id=%s status=%s\n", eventType, job.Name, buildJobID, status)
		updates["started_at"] = time.Now()
		updates["status"] = "Building"
		model.UpdateBuildJobStatus(buildJobID, updates)
		if !streamedJobs[job.Name] {
			streamedJobs[job.Name] = true
			go streamJobLogs(ctx, clientset, namespace, job.Name, buildJobID)
		}

	case "Success", "Failed":
		fmt.Printf("[job-watcher] event=%s job=%s build_job_id=%s status=%s\n", eventType, job.Name, buildJobID, status)
		if !streamedJobs[job.Name] {
			streamedJobs[job.Name] = true
			go streamJobLogs(ctx, clientset, namespace, job.Name, buildJobID)
		}
		updates["finished_at"] = time.Now()
		updates["status"] = status
		model.UpdateBuildJobStatus(buildJobID, updates)
		syncContainerStatus(buildJobID, status)
	default:
		model.UpdateBuildJobStatus(buildJobID, updates)
	}

	return nil
}

// syncContainerStatus は BuildJob の結果に応じて Container のステータスを更新します。
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

	container, err := model.GetContainerByID(buildJob.ContainerID)
	if err != nil {
		fmt.Printf("[job-watcher] failed to get container %s: %v\n", buildJob.ContainerID, err)
		return
	}

	imageRef := fmt.Sprintf("%s/%s/%s:%s",
		registryHost, container.ProjectID, buildJob.ContainerID, buildJob.ImageID)

	ctx := context.Background()
	if err := job_queue.EnqueueTo(ctx, "controller", jobs.DeployJobArgs{
		ContainerID: buildJob.ContainerID,
		ImageRef:    imageRef,
		BuildJobID:  buildJobID,
	}, nil); err != nil {
		fmt.Printf("[job-watcher] failed to enqueue deploy job for container %s: %v\n", buildJob.ContainerID, err)
	} else {
		fmt.Printf("[job-watcher] enqueued deploy job for container %s\n", buildJob.ContainerID)
	}
}

// determineJobStatus は Job の Conditions と Active/Succeeded/Failed カウントからステータスを返します。
func determineJobStatus(job *batchv1.Job) string {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return "Success"
		}
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return "Failed"
		}
	}

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
