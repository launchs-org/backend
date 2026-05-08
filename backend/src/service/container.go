package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"launchs/shared/database"
	"backend/k8slogwatcher"
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

	containerID := "cont_" + uuid.New().String()
	imageID := "img_" + uuid.New().String()
	serviceID := "svc_" + uuid.New().String()
	buildJobID := "bj_" + uuid.New().String()

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
	if err := enqueueBuildTask(project, &container, buildJob); err != nil {
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

	// delete_container タスクを投入
	payload := map[string]string{
		"container_id": containerID,
		"namespace":    project.Namespace,
		"image_name":   container.ID,
	}
	payloadJSON, _ := json.Marshal(payload)

	deleteTask := &model.Task{
		ID:        "task_del_" + containerID[:8] + "_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		TaskType:  "delete_container",
		Status:    "pending",
		Payload:   string(payloadJSON),
		TimeoutAt: time.Now().Add(5 * time.Minute),
	}
	return model.CreateTask(deleteTask)
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
		ID:            "bj_" + uuid.New().String(),
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

	if err := enqueueBuildTask(project, container, buildJob); err != nil {
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
	registryProject := os.Getenv("REGISTRY_PROJECT")
	if registryProject == "" {
		registryProject = "launchs"
	}
	imageRef := fmt.Sprintf("%s/%s/%s:%s", registryHost, registryProject, container.ID, container.ImageID)

	// deploy タスクをキューに投入
	payload := map[string]string{
		"container_id": container.ID,
		"image_ref":    imageRef,
		"namespace":    project.Namespace,
		"build_job_id": "",
	}
	payloadJSON, _ := json.Marshal(payload)

	deployTask := &model.Task{
		ID:        "task_deploy_" + container.ID[:8] + "_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		TaskType:  "deploy",
		Status:    "pending",
		Payload:   string(payloadJSON),
		TimeoutAt: time.Now().Add(10 * time.Minute),
	}
	if err := model.CreateTask(deployTask); err != nil {
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

func StreamContainerLogs(ctx context.Context, containerID string, ownerID string, baselogCallback func(k8slogwatcher.LogEntry)) error {
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

	sinceTime := time.Now().Add(-1 * time.Hour)
	logCallback := func(entry k8slogwatcher.LogEntry) {
		baselogCallback(entry)
	}

	sub, err := k8slogwatcher.GlobalWatcher.Subscribe(ctx, project.Namespace, container.Name, sinceTime, logCallback)
	if err != nil {
		return fmt.Errorf("failed to subscribe container logs: %w", err)
	}

	<-ctx.Done()

	k8slogwatcher.GlobalWatcher.Unsubscribe(project.Namespace, container.Name)
	_ = sub

	return nil
}

// enqueueBuildTask は build タスクを tasks テーブルに INSERT します
func enqueueBuildTask(project *model.Project, container *model.Container, buildJob model.BuildJob) error {
	payload := map[string]string{
		"build_job_id":   buildJob.ID,
		"container_id":   container.ID,
		"image_id":       container.ImageID,
		"project_id":     project.ID,
		"project_name":   project.Name,
		"container_name": container.Name,
		"namespace":      project.Namespace,
		"repository_url": container.RepositoryURL,
		"branch":         container.Branch,
		"directory":      container.Directory,
		"build_type":     "railpack",
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	task := &model.Task{
		ID:        "task_build_" + buildJob.ID,
		TaskType:  "build",
		Status:    "pending",
		Payload:   string(payloadJSON),
		TimeoutAt: time.Now().Add(30 * time.Minute),
	}
	return model.CreateTask(task)
}
