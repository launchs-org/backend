package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"backend/database"
	"backend/model"
	"backend/railpack"
	"backend/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes"
)

var (
	ErrContainerAlreadyExists = errors.New("container already exists")
)

type CreateContainerInput struct {
	ProjectID     string
	OwnerID       string
	Name          string
	RepositoryURL string
	Branch        string
	Directory     string
	EnvVars       string
	Replicas      int
	Resources     string
}

// CreateContainer はコンテナを作成し、ビルドジョブを発行します
func CreateContainer(ctx context.Context, input CreateContainerInput) (map[string]interface{}, error) {
	// プロジェクトを取得
	project, err := model.GetProjectByID(input.ProjectID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}

	// 権限チェック
	if project.OwnerID != input.OwnerID {
		return nil, ErrForbidden
	}

	// 重複チェック
	existing, err := model.GetContainerCountByProjectIDAndName(input.ProjectID, input.Name)
	if err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, ErrContainerAlreadyExists
	}

	// 各種IDを生成
	containerID := "cont_" + uuid.New().String()
	imageID := "img_" + uuid.New().String()
	serviceID := "svc_" + uuid.New().String()
	buildJobID := "bj_" + uuid.New().String()

	// デフォルト値の設定
	branch := input.Branch
	if branch == "" {
		branch = "main"
	}
	directory := input.Directory
	if directory == "" {
		directory = "/"
	}
	replicas := input.Replicas
	if replicas == 0 {
		replicas = 1
	}

	envVarsStr := input.EnvVars
	if envVarsStr == "" {
		envVarsStr = "{}"
	}

	resourcesStr := input.Resources
	if resourcesStr == "" {
		resourcesStr = "{}"
	}

	// Image作成
	image := model.Image{
		ID:          imageID,
		ContainerID: containerID,
		Type:        "user",
		Name:        fmt.Sprintf("%s-%s", project.Name, input.Name),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Container作成
	container := model.Container{
		ID:            containerID,
		ProjectID:     project.ID,
		Name:          input.Name,
		ImageID:       imageID,
		RepositoryURL: input.RepositoryURL,
		Branch:        branch,
		Directory:     directory,
		Replicas:      replicas,
		EnvVars:       envVarsStr,
		Resources:     resourcesStr,
		Status:        "Stopped",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Service作成
	k8sService := model.Service{
		ID:          serviceID,
		ContainerID: containerID,
		Type:        "LoadBalancer",
		Ports:       "[]",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// BuildJob作成
	buildJob := model.BuildJob{
		ID:            buildJobID,
		ProjectID:     project.ID,
		ContainerID:   containerID,
		RepositoryURL: input.RepositoryURL,
		Branch:        branch,
		Directory:     directory,
		Status:        "Queued",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// トランザクションで保存
	err = model.CreateContainerWithRelatedRecords(&image, &container, &k8sService, &buildJob)

	if err != nil {
		return nil, err
	}

	// railpackのビルドを発行する (非同期処理)
	go startRailpackBuild(*project, container, buildJob)

	// レスポンスを作成
	return map[string]interface{}{
		"data": map[string]interface{}{
			"container": container,
			"build_job": buildJob,
		},
	}, nil
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
		ImageName: container.Name,
		ImageTag:  buildJob.ID,
	})
	if err != nil {
		handleBuildError(buildJob.ID, container.ID, fmt.Errorf("failed to generate upload token: %w", err))
		return
	}

	client, err := railpack.New(database.K8sClientset.(*kubernetes.Clientset), railpack.BuildConfig{
		GitRepo:        container.RepositoryURL,
		GitBranch:      container.Branch,
		Subdir:         container.Directory,
		ImageName:      container.Name,
		ImageTag:       buildJob.ID,
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

	status, err := client.Wait(ctx, railpackJobID)
	if err != nil {
		handleBuildError(buildJob.ID, container.ID, fmt.Errorf("build failed while waiting: %w", err))
		return
	}

	if status == railpack.StatusComplete {
		now := time.Now()
		model.UpdateBuildJobStatus(buildJob.ID, map[string]interface{}{
			"status":      "Success",
			"finished_at": now,
		})
		// 本来はここでデプロイに進む
		// database.DB.Model(&model.Container{}).Where("id = ?", container.ID).Update("status", "Running")
	} else {
		handleBuildError(buildJob.ID, container.ID, fmt.Errorf("build failed with status: %s", status))
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

// HandleUploadTar は受け取ったtarを保存します
func HandleUploadTar(body io.Reader, jobID, imageName, imageTag string) error {
	// saveDir := os.Getenv("TAR_SAVE_DIR")
	// if saveDir == "" {
	// 	saveDir = "/tmp/launchs-tar"
	// }
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
	// TODO: crane.Pushなどでイメージをレジストリにプッシュする処理をここに追加する

	return nil
}
