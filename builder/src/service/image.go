package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"launchs/shared/job_queue"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
)

// HandleUploadTar は受け取った tar を Harbor にプッシュし、deploy ジョブを作成します
func HandleUploadTar(ctx context.Context, body io.Reader, jobID, imageName, imageTag string) error {
	saveDir := "./launchs-tar"
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		return fmt.Errorf("failed to create dir: %w", err)
	}

	savePath := filepath.Join(saveDir, fmt.Sprintf("%s.tar", jobID))
	file, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, body); err != nil {
		return fmt.Errorf("failed to write tar: %w", err)
	}

	fmt.Printf("[builder] pushing image %s:%s to registry\n", imageName, imageTag)
	if err := PushToRegistry(savePath, imageName, imageTag); err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}

	// BuildJob を Success に更新
	now := time.Now()
	model.UpdateBuildJobStatus(jobID, map[string]interface{}{
		"status":      "Success",
		"finished_at": now,
	})

	// deploy ジョブをキューに追加
	registryHost := registryHostEnv()
	registryProject := registryProjectEnv()
	imageRef := fmt.Sprintf("%s/%s/%s:%s", registryHost, registryProject, imageName, imageTag)

	buildJob, err := model.GetBuildJobByID(jobID)
	if err != nil {
		return fmt.Errorf("failed to get build job: %w", err)
	}

	if err := job_queue.Enqueue(ctx, jobs.DeployJobArgs{
		ContainerID: buildJob.ContainerID,
		ImageRef:    imageRef,
		BuildJobID:  jobID,
	}, nil); err != nil {
		return fmt.Errorf("failed to enqueue deploy job: %w", err)
	}

	fmt.Printf("[builder] enqueued deploy job for container %s\n", buildJob.ContainerID)
	return nil
}

// PushToRegistry は crane を使ってイメージを Harbor にプッシュします（最大5回リトライ）
func PushToRegistry(tarPath, imageName, imageTag string) error {
	registryHost := registryHostEnv()
	registryProject := os.Getenv("REGISTRY_PROJECT")
	if registryProject == "" {
		registryProject = "buildkit"
	}
	username := os.Getenv("REGISTRY_USERNAME")
	if username == "" {
		username = "robot$buildkit+buildkit"
	}
	password := os.Getenv("REGISTRY_PASSWORD")
	insecure := os.Getenv("REGISTRY_INSECURE") == "true"

	opts := []crane.Option{
		crane.WithAuth(&authn.Basic{Username: username, Password: password}),
		crane.WithTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		}),
	}

	target := fmt.Sprintf("%s/%s/%s:%s", registryHost, registryProject, imageName, imageTag)

	var lastErr error
	for i := 0; i < 5; i++ {
		img, err := crane.Load(tarPath)
		if err != nil {
			lastErr = err
			fmt.Printf("[builder] attempt %d: failed to load tar: %v\n", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		if err := crane.Push(img, target, opts...); err != nil {
			lastErr = err
			fmt.Printf("[builder] attempt %d: failed to push image: %v\n", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		os.Remove(tarPath)
		return nil
	}
	return fmt.Errorf("failed after 5 attempts: %w", lastErr)
}

// DeleteFromRegistry は Harbor からイメージ（全タグ）を削除します
func DeleteFromRegistry(imageName string, tags []string) error {
	registryHost := registryHostEnv()
	registryProject := registryProjectEnv()
	username := os.Getenv("REGISTRY_USERNAME")
	if username == "" {
		username = "robot$buildkit+buildkit"
	}
	password := os.Getenv("REGISTRY_PASSWORD")
	insecure := os.Getenv("REGISTRY_INSECURE") == "true"

	opts := []crane.Option{
		crane.WithAuth(&authn.Basic{Username: username, Password: password}),
		crane.WithTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		}),
	}

	var lastErr error
	for _, tag := range tags {
		ref := fmt.Sprintf("%s/%s/%s:%s", registryHost, registryProject, imageName, tag)
		if err := crane.Delete(ref, opts...); err != nil {
			fmt.Printf("[builder] failed to delete image %s: %v\n", ref, err)
			lastErr = err
		}
	}
	return lastErr
}

func registryHostEnv() string {
	if v := os.Getenv("REGISTRY_HOST"); v != "" {
		return v
	}
	return "172.33.0.1"
}

func registryProjectEnv() string {
	if v := os.Getenv("REGISTRY_PROJECT"); v != "" {
		return v
	}
	return "launchs"
}
