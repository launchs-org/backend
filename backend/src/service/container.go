package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"backend/database"
	"backend/k8slogwatcher"
	"backend/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// DeleteContainer はコンテナと関連するリソースを削除します
func DeleteContainer(ctx context.Context, containerID string, ownerID string) error {
	// 1. コンテナを取得
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrContainerNotFound
		}
		return err
	}

	// 2. 権限チェック
	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return err
	}
	if project.OwnerID != ownerID {
		return ErrForbidden
	}

	// 3. 削除処理 (トランザクション内)
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// イメージの削除
		if err := model.DeleteImagesByContainerID(containerID); err != nil {
			return err
		}
		// サービス設定の削除
		if err := model.DeleteServiceByContainerID(containerID); err != nil {
			return err
		}
		// Ingress設定の削除
		if err := model.DeleteIngress(containerID); err != nil {
			return err
		}
		// ビルドジョブの削除
		if err := model.DeleteBuildJobsByContainerID(containerID); err != nil {
			return err
		}
		// コンテナ自身の削除
		if err := model.DeleteContainer(containerID); err != nil {
			return err
		}

		// Kubernetes リソースの削除
		// Deployment の削除
		_ = database.K8sClientset.AppsV1().Deployments(project.Namespace).Delete(ctx, container.Name, metav1.DeleteOptions{})

		// Service の削除
		_ = database.K8sClientset.CoreV1().Services(project.Namespace).Delete(ctx, container.Name, metav1.DeleteOptions{})

		// Ingress の削除 (Ingress名がコンテナ名と一致していると仮定)
		_ = database.K8sClientset.NetworkingV1().Ingresses(project.Namespace).Delete(ctx, container.Name, metav1.DeleteOptions{})

		return nil
	})

	return err
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

// UpdateContainer はコンテナの設定を更新し、再ビルドを開始します
func UpdateContainer(ctx context.Context, input UpdateContainerInput) (map[string]interface{}, error) {
	// 1. コンテナを取得
	container, err := model.GetContainerByID(input.ContainerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContainerNotFound
		}
		return nil, err
	}

	// 2. プロジェクトを取得して権限チェック
	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.OwnerID != input.OwnerID {
		return nil, ErrForbidden
	}

	// 3. フィールドの更新
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

	// 再ビルドのために新しいイメージIDを生成
	newImageID := "img_" + uuid.New().String()
	container.ImageID = newImageID
	updates["image_id"] = newImageID
	updates["updated_at"] = time.Now()

	// 4. 更新を保存
	if err := database.DB.Model(container).Updates(updates).Error; err != nil {
		return nil, err
	}

	// 5. 新しい Image レコードを作成
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

	// 6. BuildJob を作成
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

	// 7. ビルドを開始
	go startRailpackBuild(*project, *container, buildJob)

	return map[string]interface{}{
		"data": map[string]interface{}{
			"container": container,
			"build_job": buildJob,
		},
	}, nil
}

// RedeployContainer はコンテナを再デプロイします
func RedeployContainer(ctx context.Context, containerID, ownerID string) (map[string]interface{}, error) {
	return UpdateContainer(ctx, UpdateContainerInput{
		ContainerID: containerID,
		OwnerID:     ownerID,
	})
}

type ListBuildJobsInput struct {
	ContainerID string
	OwnerID     string
}

// ListBuildJobs はコンテナのビルド履歴一覧を取得します
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

// GetContainer はコンテナの詳細を取得します
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

// StreamContainerLogs はコンテナの実行ログをストリーミングします
func StreamContainerLogs(ctx context.Context, containerID string, ownerID string, baselogCallback func(k8slogwatcher.LogEntry)) error {
	// 1. コンテナを取得
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrContainerNotFound
		}
		return err
	}

	// 2. プロジェクトを取得して権限チェック
	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return err
	}
	if project.OwnerID != ownerID {
		return ErrForbidden
	}

	// 3. 購読を開始 (1時間前からのログを取得)
	sinceTime := time.Now().Add(-1 * time.Hour)

	// callback を生成
	logCallback := func(entry k8slogwatcher.LogEntry) {
		// データベースに保存する

		baselogCallback(entry)
	}
	
	// GlobalWatcher を使用して Deployment (コンテナ名と一致) を監視
	sub, err := k8slogwatcher.GlobalWatcher.Subscribe(ctx, project.Namespace, container.Name, sinceTime, logCallback)
	if err != nil {
		return fmt.Errorf("failed to subscribe container logs: %w", err)
	}

	// コンテキストがキャンセルされるまで待機 (WebSocketが切れるまで)
	<-ctx.Done()

	// 購読を停止
	k8slogwatcher.GlobalWatcher.Unsubscribe(project.Namespace, container.Name)

	_ = sub // Subscription を使用しない場合の警告回避

	return nil
}
