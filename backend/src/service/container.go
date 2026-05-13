package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"launchs/shared/database"
	"launchs/shared/job_queue"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrContainerAlreadyExists = errors.New("container already exists")
	ErrContainerNotFound      = errors.New("container not found")
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

// CreateContainer はコンテナを作成し、build タスクをキューに投入します
func CreateContainer(ctx context.Context, input CreateContainerInput) (map[string]interface{}, error) {
	project, err := model.GetProjectByID(input.ProjectID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}

	if project.OwnerID != input.OwnerID {
		return nil, ErrForbidden
	}

	existing, err := model.GetContainerCountByProjectIDAndName(input.ProjectID, input.Name)
	if err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, ErrContainerAlreadyExists
	}

	containerID := "cont-" + uuid.New().String()
	imageID := "img-" + uuid.New().String()
	serviceID := "svc-" + uuid.New().String()
	buildJobID := "bj-" + uuid.New().String()

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

	image := model.Image{
		ID:          imageID,
		ContainerID: containerID,
		Type:        "user",
		Name:        fmt.Sprintf("%s-%s", project.Name, input.Name),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
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
		Status:        "Queued",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	k8sService := model.Service{
		ID:          serviceID,
		ContainerID: containerID,
		Type:        "LoadBalancer",
		Ports:       "[]",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
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

	if err := model.CreateContainerWithRelatedRecords(&image, &container, &k8sService, &buildJob); err != nil {
		return nil, err
	}

	// build タスクをキューに投入
	if err := enqueueBuildTask(ctx, project, &container, buildJob); err != nil {
		fmt.Printf("[service] failed to enqueue build task: %v\n", err)
	}

	return map[string]interface{}{
		"data": map[string]interface{}{
			"container": container,
			"build_job": buildJob,
		},
	}, nil
}

// DeleteContainer はコンテナ削除タスクをキューに投入します
func DeleteContainer(ctx context.Context, containerID string, ownerID string) error {
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrContainerNotFound
		}
		return err
	}

	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return err
	}
	if project.OwnerID != ownerID {
		return ErrForbidden
	}

	// delete_container ジョブをキューに追加
	return job_queue.EnqueueTo(ctx, "controller", jobs.DeleteContainerJobArgs{
		ContainerID: containerID,
		Namespace:   project.Namespace,
		ImageName:   container.ID,
	}, nil)
}

type UpdateContainerInput struct {
	ContainerID   string
	OwnerID       string
	RepositoryURL *string
	Branch        *string
	Directory     *string
	EnvVars       *string
	Replicas      *int
	Resources     *string
}

// UpdateContainer はコンテナの設定を更新し、build タスクを投入します
func UpdateContainer(ctx context.Context, input UpdateContainerInput) (map[string]interface{}, error) {
	container, err := model.GetContainerByID(input.ContainerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContainerNotFound
		}
		return nil, err
	}

	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.OwnerID != input.OwnerID {
		return nil, ErrForbidden
	}

	updates := make(map[string]interface{})
	if input.RepositoryURL != nil {
		container.RepositoryURL = *input.RepositoryURL
		updates["repository_url"] = *input.RepositoryURL
	}
	if input.Branch != nil {
		container.Branch = *input.Branch
		updates["branch"] = *input.Branch
	}
	if input.Directory != nil {
		container.Directory = *input.Directory
		updates["directory"] = *input.Directory
	}
	if input.EnvVars != nil {
		container.EnvVars = *input.EnvVars
		updates["env_vars"] = *input.EnvVars
	}
	if input.Replicas != nil {
		container.Replicas = *input.Replicas
		updates["replicas"] = *input.Replicas
	}
	if input.Resources != nil {
		container.Resources = *input.Resources
		updates["resources"] = *input.Resources
	}

	newImageID := "img_" + uuid.New().String()
	container.ImageID = newImageID
	updates["image_id"] = newImageID
	updates["updated_at"] = time.Now()

	if err := database.DB.Model(container).Updates(updates).Error; err != nil {
		return nil, err
	}

	newImage := model.Image{
		ID:          newImageID,
		ContainerID: container.ID,
		Type:        "user",
		Name:        fmt.Sprintf("%s-%s", project.Name, container.Name),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := database.DB.Create(&newImage).Error; err != nil {
		return nil, err
	}

	buildJob := model.BuildJob{
		ID:            "bj-" + uuid.New().String(),
		ProjectID:     project.ID,
		ContainerID:   container.ID,
		RepositoryURL: container.RepositoryURL,
		Branch:        container.Branch,
		Directory:     container.Directory,
		Status:        "Queued",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := database.DB.Create(&buildJob).Error; err != nil {
		return nil, err
	}

	if err := enqueueBuildTask(ctx, project, container, buildJob); err != nil {
		fmt.Printf("[service] failed to enqueue build task: %v\n", err)
	}

	return map[string]interface{}{
		"data": map[string]interface{}{
			"container": container,
			"build_job": buildJob,
		},
	}, nil
}

// RebuildContainer はコンテナを再ビルドします
func RebuildContainer(ctx context.Context, containerID, ownerID string) (map[string]interface{}, error) {
	return UpdateContainer(ctx, UpdateContainerInput{
		ContainerID: containerID,
		OwnerID:     ownerID,
	})
}

// RedeployContainer はコンテナを再デプロイします（ビルドなし）
func RedeployContainer(ctx context.Context, containerID, ownerID string) (map[string]interface{}, error) {
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContainerNotFound
		}
		return nil, err
	}

	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	registryHost := os.Getenv("REGISTRY_HOST")
	if registryHost == "" {
		registryHost = "172.33.0.1"
	}
	
	registryProject := container.ProjectID

	imageRef := fmt.Sprintf("%s/%s/%s:%s", registryHost, registryProject, container.ID, container.ImageID)

	// deploy ジョブをキューに追加
	if err := job_queue.EnqueueTo(ctx, "controller", jobs.DeployJobArgs{
		ContainerID: container.ID,
		ImageRef:    imageRef,
		Namespace:   project.Namespace,
	}, nil); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"data": container,
	}, nil
}

type ListBuildJobsInput struct {
	ContainerID string
	OwnerID     string
}

func ListBuildJobs(ctx context.Context, input ListBuildJobsInput) (map[string]interface{}, error) {
	container, err := model.GetContainerByID(input.ContainerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContainerNotFound
		}
		return nil, err
	}

	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.OwnerID != input.OwnerID {
		return nil, ErrForbidden
	}

	jobs, err := model.GetBuildJobsByContainerID(input.ContainerID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"data": map[string]interface{}{
			"items": jobs,
			"total": len(jobs),
		},
	}, nil
}

func GetContainer(ctx context.Context, containerID string, ownerID string) (map[string]interface{}, error) {
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContainerNotFound
		}
		return nil, err
	}

	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	return map[string]interface{}{
		"data": container,
	}, nil
}

// enqueueBuildTask は build ジョブをキューに追加します
func enqueueBuildTask(ctx context.Context, project *model.Project, container *model.Container, buildJob model.BuildJob) error {
	return job_queue.EnqueueTo(ctx, "builder", jobs.BuildJobArgs{
		BuildJobID:    buildJob.ID,
		ContainerID:   container.ID,
		ImageID:       container.ImageID,
		ProjectID:     project.ID,
		ProjectName:   project.Name,
		ContainerName: container.Name,
		Namespace:     project.Namespace,
		RepositoryURL: container.RepositoryURL,
		Branch:        container.Branch,
		Directory:     container.Directory,
		BuildType:     "railpack",
	}, nil)
}
