package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"launchs/shared/job_queue"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/google/uuid"
)

var (
	ErrVolumeNotFound     = errors.New("volume not found")
	ErrVolumeSizeExceeded = errors.New("volume size exceeded (max 5GB)")
)

type CreateVolumeInput struct {
	ProjectID   string
	OwnerID     string
	ContainerID string
	Name        string
	SizeMB      int
	MountPath   string
}

func CreateVolume(ctx context.Context, input CreateVolumeInput) (*model.Volume, error) {
	project, err := model.GetProjectByID(input.ProjectID)
	if err != nil {
		return nil, ErrProjectNotFound
	}

	if project.OwnerID != input.OwnerID {
		return nil, ErrForbidden
	}

	if input.SizeMB > 5120 {
		return nil, ErrVolumeSizeExceeded
	}

	volumeID := "vol-" + uuid.New().String()

	volume := &model.Volume{
		ID:          volumeID,
		ProjectID:   input.ProjectID,
		ContainerID: input.ContainerID,
		Name:        input.Name,
		SizeMB:      input.SizeMB,
		MountPath:   input.MountPath,
		Status:      "Pending",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := model.CreateVolume(volume); err != nil {
		return nil, fmt.Errorf("failed to save volume to DB: %w", err)
	}

	if err := job_queue.EnqueueTo(ctx, "controller", jobs.CreateVolumeJobArgs{
		VolumeID:  volumeID,
		Namespace: project.Namespace,
		SizeMB:    input.SizeMB,
	}, nil); err != nil {
		fmt.Printf("[service] failed to enqueue create_volume job: %v\n", err)
	}

	return volume, nil
}

func ListVolumes(ctx context.Context, containerID string, ownerID string) ([]model.Volume, error) {
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		return nil, ErrContainerNotFound
	}

	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return nil, ErrProjectNotFound
	}

	if project.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	return model.GetVolumesByContainerID(containerID)
}

func DeleteVolume(ctx context.Context, volumeID string, ownerID string) error {
	volume, err := model.GetVolumeByID(volumeID)
	if err != nil {
		return ErrVolumeNotFound
	}

	project, err := model.GetProjectByID(volume.ProjectID)
	if err != nil {
		return ErrProjectNotFound
	}

	if project.OwnerID != ownerID {
		return ErrForbidden
	}

	volume.Status = "Deleting"
	if err := model.UpdateVolume(volume); err != nil {
		return fmt.Errorf("failed to update volume status: %w", err)
	}

	if err := job_queue.EnqueueTo(ctx, "controller", jobs.DeleteVolumeJobArgs{
		VolumeID:  volumeID,
		Namespace: project.Namespace,
	}, nil); err != nil {
		fmt.Printf("[service] failed to enqueue delete_volume job: %v\n", err)
	}

	return nil
}
