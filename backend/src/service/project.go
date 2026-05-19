package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"launchs/shared/job_queue"
	"launchs/shared/job_queue/jobs"
	"gorm.io/gorm"
	"launchs/shared/model"

	"github.com/google/uuid"
)

var projectNameRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

var (
	ErrInvalidProjectName   = errors.New("INVALID_PROJECT_NAME")
	ErrProjectAlreadyExists = errors.New("PROJECT_ALREADY_EXISTS")
	ErrProjectNotFound      = errors.New("NOT_FOUND")
	ErrForbidden            = errors.New("FORBIDDEN")
)

type CreateProjectInput struct {
	Name    string
	OwnerID string
}

func CreateProject(ctx context.Context, input CreateProjectInput) (*model.Project, error) {
	if input.Name == "" || !projectNameRegex.MatchString(input.Name) {
		return nil, ErrInvalidProjectName
	}

	existing, _ := model.GetProjectByName(input.Name)
	if existing != nil {
		return nil, ErrProjectAlreadyExists
	}

	projectID := uuid.New().String()
	namespace := fmt.Sprintf("ns-%s", projectID)

	project := &model.Project{
		ID:              projectID,
		Name:            input.Name,
		K8sResourceName: input.Name,
		Namespace:       namespace,
		OwnerID:         input.OwnerID,
	}

	if err := model.CreateProject(project); err != nil {
		return nil, fmt.Errorf("プロジェクトの保存に失敗しました: %w", err)
	}

	if err := job_queue.EnqueueTo(ctx, "controller", jobs.CreateProjectJobArgs{
		ProjectID:   projectID,
		ProjectName: input.Name,
		Namespace:   namespace,
		OwnerID:     input.OwnerID,
	}, nil); err != nil {
		fmt.Printf("[service] failed to enqueue create_project job: %v\n", err)
	}

	return project, nil
}

func GetProjectByID(ctx context.Context, id string, userID string) (*model.Project, error) {
	project, err := model.GetProjectByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}
	if project.OwnerID != userID {
		return nil, ErrForbidden
	}
	return project, nil
}

func ListProjects(ctx context.Context, userID string) ([]model.Project, error) {
	return model.GetProjectsByOwnerID(userID)
}

func DeleteProject(ctx context.Context, id string, userID string) error {
	project, err := model.GetProjectByID(id)
	if err != nil {
		return ErrProjectNotFound
	}
	if project.OwnerID != userID {
		return ErrForbidden
	}

	if err := model.UpdateProjectStatus(id, "Deleting"); err != nil {
		return fmt.Errorf("failed to update project status: %w", err)
	}

	return job_queue.EnqueueTo(ctx, "controller", jobs.DeleteProjectJobArgs{
		ProjectID: id,
		Namespace: project.Namespace,
	}, nil)
}
