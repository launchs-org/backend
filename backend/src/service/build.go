package service

import (
	"context"
	"fmt"
	"os"
	"time"

	"backend/database"
	"backend/model"
	"backend/railpack"
	"backend/utils"

	"k8s.io/client-go/kubernetes"
)

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

	// ログ取得 (ここでは簡単にコンソールに出すだけかDB保存するか)
	logCh, errCh := client.StreamLogs(ctx, railpackJobID)
	go func() {
		for line := range logCh {
			// 将来的にはここで BuildJob.BuildLog に追記するなどの処理を入れる
			fmt.Println("[build]", buildJob.ID, line)
		}
		if err := <-errCh; err != nil {
			fmt.Println("[build error]", buildJob.ID, err)
		}
	}()
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
