package service

import (
	"context"
	"fmt"
	"launchs/shared/database"
	"backend/k8slogwatcher"
	"launchs/shared/model"
	"backend/railpack"
	"launchs/shared/utils"
	"os"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
)

// StreamBuildJobLogs はビルドジョブのログをストリーミングします
func StreamBuildJobLogs(ctx context.Context, buildJobID string, ownerID string, logCallback func(string), statusCallback func(string)) error {
	// ビルドジョブを取得
	job, err := model.GetBuildJobByID(buildJobID)
	if err != nil {
		return ErrBuildJobNotFound
	}

	// プロジェクトを取得して所有者チェック
	project, err := model.GetProjectByID(job.ProjectID)
	if err != nil {
		return err
	}
	if project.OwnerID != ownerID {
		return ErrForbidden
	}

	// 1. 履歴ログを送信 (DBから取得)
	history, err := model.GetBuildJobLog(buildJobID)
	if err == nil && len(history) > 0 {
		logCallback(string(history))
	}

	// すでに終了している場合はここで終了
	if job.Status != "Queued" && job.Status != "Running" {
		statusCallback(job.Status)
		return nil
	}

	// 2. リアルタイムログとステータスを監視
	// JobWatcher を使用して Kubernetes Job を監視
	// Namespace は環境変数から取得 (railpack と合わせる)
	namespace := os.Getenv("BUILD_NAMESPACE")
	if namespace == "" {
		namespace = "buildkit"
	}

	// JobWatcher に登録
	// Job名は railpack- + BuildJob ID (アンダースコアをハイフンに変換)
	k8sJobName := "railpack-" + strings.ReplaceAll(buildJobID, "_", "-")
	_, err = k8slogwatcher.GlobalJobWatcher.Watch(
		ctx,
		namespace,
		k8sJobName,
		func(entry k8slogwatcher.JobLogEntry) {
			// ログを送信
			logCallback(entry.Message + "\n")
		},
		func(entry k8slogwatcher.JobStatusEntry) {
			// ステータスを送信
			statusCallback(string(entry.Status))
		},
	)

	if err != nil {
		return fmt.Errorf("failed to start watching job: %w", err)
	}

	// コンテキストがキャンセルされるまで待機
	<-ctx.Done()

	return nil
}

// startRailpackBuild は非同期でビルドを実行します
func startRailpackBuild(project model.Project, container model.Container, buildJob model.BuildJob) {
	ctx := context.Background()

	// 状態を Building に更新
	model.UpdateContainerStatus(container.ID, "Building")
	model.UpdateBuildJobStatus(buildJob.ID, map[string]interface{}{
		"status":     "Running",
		"started_at": time.Now(),
	})

	uploadEndpoint := os.Getenv("UPLOAD_ENDPOINT")
	if uploadEndpoint == "" {
		uploadEndpoint = "https://10.10.11.8:8090/app/internal/upload"
	}
	uploadToken, err := utils.GenerateJobToken(utils.JobTokenClaim{
		JobID:     buildJob.ID,
		ImageName: container.ID,
		ImageTag:  container.ImageID,
	})
	if err != nil {
		handleBuildError(buildJob.ID, container.ID, fmt.Errorf("failed to generate upload token: %w", err))
		return
	}

	client, err := railpack.New(database.K8sClientset.(*kubernetes.Clientset), railpack.BuildConfig{
		GitRepo:        container.RepositoryURL,
		GitBranch:      container.Branch,
		Subdir:         container.Directory,
		ImageName:      container.ID,
		ImageTag:       container.ImageID,
		UploadEndpoint: uploadEndpoint,
		UploadToken:    uploadToken,
		Namespace:      os.Getenv("BUILD_NAMESPACE"), // fallback is default in railpack
		Timeout:        10 * time.Minute,
		JobID:          strings.ReplaceAll(buildJob.ID, "_", "-"),
	})

	if err != nil {
		handleBuildError(buildJob.ID, container.ID, fmt.Errorf("failed to init railpack: %w", err))
		return
	}

	railpackJobID, err := client.Build(ctx)
	if err != nil {
		handleBuildError(buildJob.ID, container.ID, fmt.Errorf("failed to start railpack build: %w", err))
		return
	}

	// Kubernetes Job 名を構築 (railpack 内での命名規則に合わせる)
	k8sJobName := "railpack-" + railpackJobID
	namespace := os.Getenv("BUILD_NAMESPACE")
	if namespace == "" {
		namespace = "buildkit"
	}

	// JobWatcher を使用してログを監視し、データベースに保存
	_, err = k8slogwatcher.GlobalJobWatcher.Watch(
		ctx,
		namespace,
		k8sJobName,
		func(entry k8slogwatcher.JobLogEntry) {
			// コンテナ名を含めてログを保存 (railpack の形式を模倣)
			line := fmt.Sprintf("[%s] %s\n", entry.Container, entry.Message)
			err := model.AppendBuildLog(buildJob.ID, []byte(line))
			if err != nil {
				fmt.Println("[build log save error]", buildJob.ID, err)
			}
		},
		func(entry k8slogwatcher.JobStatusEntry) {
			// ステータスが完了 (Succeeded/Failed) した場合の処理
			if entry.Status == k8slogwatcher.JobStatusSucceeded {
				model.UpdateBuildJobStatus(buildJob.ID, map[string]interface{}{
					"status":      "Success",
					"finished_at": time.Now(),
				})
				// model.UpdateContainerStatus(container.ID, "Success")
			} else if entry.Status == k8slogwatcher.JobStatusFailed {
				handleBuildError(buildJob.ID, container.ID, fmt.Errorf("job failed: %s", entry.Message))
			}
		},
	)

	if err != nil {
		handleBuildError(buildJob.ID, container.ID, fmt.Errorf("failed to watch build job: %w", err))
	}
}

func handleBuildError(buildJobID, containerID string, err error) {
	fmt.Println("Build Error:", err)
	now := time.Now()
	model.UpdateBuildJobStatus(buildJobID, map[string]interface{}{
		"status":      "Failed",
		"finished_at": now,
	})
	model.UpdateContainerStatus(containerID, "Failed")
}

var (
	// ErrBuildJobNotFound はビルドジョブが見つからない場合のエラーです
	ErrBuildJobNotFound = fmt.Errorf("build job not found")
)

// GetBuildJobLogs はビルドジョブのログを取得します
func GetBuildJobLogs(ctx context.Context, buildJobID string, ownerID string) (string, error) {
	// ビルドジョブを取得
	job, err := model.GetBuildJobByID(buildJobID)
	if err != nil {
		return "", ErrBuildJobNotFound
	}

	// プロジェクトを取得して所有者チェック
	project, err := model.GetProjectByID(job.ProjectID)
	if err != nil {
		return "", err
	}
	if project.OwnerID != ownerID {
		return "", ErrForbidden
	}

	// ログを取得
	logBytes, err := model.GetBuildJobLog(buildJobID)
	if err != nil {
		return "", err
	}

	return string(logBytes), nil
}
