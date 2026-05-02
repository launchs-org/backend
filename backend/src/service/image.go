package service

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"backend/model"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
)

// HandleUploadTar は受け取ったtarを保存します
func HandleUploadTar(body io.Reader, jobID, imageName, imageTag string) error {
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

	fmt.Printf("Tar saved successfully: %s\n", savePath)

	fmt.Printf("Starting to push image %s:%s to registry...\n", imageName, imageTag)
	if err := PushToRegistry(savePath, imageName, imageTag); err != nil {
		return fmt.Errorf("failed to push image to registry: %w", err)
	}
	fmt.Printf("Image %s:%s pushed successfully to registry.\n", imageName, imageTag)

	now := time.Now()
	model.UpdateBuildJobStatus(jobID, map[string]interface{}{
		"status":      "Success",
		"finished_at": now,
	})

	job, err := model.GetBuildJobByID(jobID)
	if err != nil {
		return fmt.Errorf("failed to get build job: %w", err)
	}

	model.UpdateContainerStatus(job.ContainerID, "Deploying")

	registryHost := os.Getenv("REGISTRY_HOST")
	if registryHost == "" {
		registryHost = "172.33.0.1"
	}
	registryProject := os.Getenv("REGISTRY_PROJECT")
	if registryProject == "" {
		registryProject = "launchs"
	}
	imageRef := fmt.Sprintf("%s/%s/%s:%s", registryHost, registryProject, imageName, imageTag)

	go deployToKubernetes(job.ContainerID, imageRef)

	return nil
}

// pushToRegistry は crane を使用してイメージをプッシュします。最大 5 回リトライします。
func PushToRegistry(tarPath, imageName, imageTag string) error {
	registryHost := os.Getenv("REGISTRY_HOST")
	if registryHost == "" {
		registryHost = "172.33.0.1"
	}
	registryProject := os.Getenv("REGISTRY_PROJECT")
	if registryProject == "" {
		registryProject = "buildkit"
	}
	username := "robot$buildkit+buildkit"
	password := "6vgO9jcQsfjy9XEfHBLEFyDmriaHXUQD"
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
			fmt.Printf("Attempt %d: failed to load image from tar: %v\n", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}

		if err := crane.Push(img, target, opts...); err != nil {
			lastErr = err
			fmt.Printf("Attempt %d: failed to push image: %v\n", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}

		// 成功
		_ = os.Remove(tarPath)
		return nil
	}

	return fmt.Errorf("failed to push image after 5 attempts: %v", lastErr)
}
