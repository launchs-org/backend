package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	tmpl "backend/template"
	"launchs/shared/job_queue"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrTemplateNotFound      = errors.New("template not found")
	ErrMissingRequiredEnvVar = errors.New("missing required environment variable")
)

type DeployFromTemplateInput struct {
	ProjectID     string
	OwnerID       string
	Name          string
	TemplateName  string
	EnvVars       map[string]string
	CreateService bool
	VolumeSizeMB  *int
	EnableIngress bool
	Command       string
	Args          string
}

func ListTemplates() []*tmpl.Template {
	return tmpl.List()
}

func DeployFromTemplate(ctx context.Context, input DeployFromTemplateInput) (map[string]interface{}, error) {
	t, ok := tmpl.Get(input.TemplateName)
	if !ok {
		return nil, ErrTemplateNotFound
	}

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

	// Validate required env vars and build merged env map
	envMap := make(map[string]string)
	for _, def := range t.EnvVars {
		if val, provided := input.EnvVars[def.Key]; provided && val != "" {
			envMap[def.Key] = val
		} else if def.Default != "" {
			envMap[def.Key] = def.Default
		} else if def.Required {
			return nil, fmt.Errorf("%w: %s", ErrMissingRequiredEnvVar, def.Key)
		}
	}
	envJSON, _ := json.Marshal(envMap)

	containerID := "cont-" + uuid.New().String()
	serviceID := "svc-" + uuid.New().String()
	imageRef := fmt.Sprintf("%s:%s", t.Container.Image, t.Container.Tag)

	// Build resources JSON
	resources := fmt.Sprintf(
		`{"requests":{"cpu":"%s","memory":"%s"},"limits":{"cpu":"%s","memory":"%s"}}`,
		t.Container.Resources.CPURequest,
		t.Container.Resources.MemoryRequest,
		t.Container.Resources.CPULimit,
		t.Container.Resources.MemoryLimit,
	)

	command := t.Container.Command
	if input.Command != "" {
		command = input.Command
	}
	args := t.Container.Args
	if input.Args != "" {
		args = input.Args
	}

	container := model.Container{
		ID:            containerID,
		ProjectID:     project.ID,
		Name:          input.Name,
		ImageID:       imageRef,
		RepositoryURL: "",
		Branch:        "",
		Directory:     "/",
		Replicas:      t.Container.Replicas,
		EnvVars:       string(envJSON),
		Resources:     resources,
		Status:        "Queued",
		ContainerType: "database",
		TemplateName:  t.Name,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Build ports JSON for Service
	type portEntry struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
	}
	portsList := make([]portEntry, 0, len(t.Container.Ports))
	for _, p := range t.Container.Ports {
		portsList = append(portsList, portEntry{Port: p.Port, Protocol: p.Protocol})
	}
	portsJSON, _ := json.Marshal(portsList)

	var svc *model.Service
	if input.CreateService {
		svc = &model.Service{
			ID:          serviceID,
			ContainerID: containerID,
			Type:        "ClusterIP",
			Ports:       string(portsJSON),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
	}

	var ingress *model.Ingress
	if t.Ingress != nil && t.Ingress.Enabled && input.EnableIngress {
		subdomain := fmt.Sprintf("%s-%s", input.Name, containerID[:8])
		ingress = &model.Ingress{
			ID:          "ing-" + uuid.New().String(),
			ContainerID: containerID,
			Subdomain:   subdomain,
			HttpPort:    t.Ingress.HttpPort,
			Status:      "Pending",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
	}

	var vol *model.Volume
	if t.Volume != nil {
		shouldCreate := t.Volume.Required
		if !t.Volume.Required {
			// For optional volumes, check if VolumeSizeMB was provided (means user opted in)
			shouldCreate = input.VolumeSizeMB != nil
		}
		if shouldCreate {
			sizeMB := t.Volume.DefaultSizeMB
			if input.VolumeSizeMB != nil {
				sizeMB = *input.VolumeSizeMB
			}
			volumeID := "vol-" + uuid.New().String()
			vol = &model.Volume{
				ID:          volumeID,
				ProjectID:   project.ID,
				ContainerID: containerID,
				Name:        fmt.Sprintf("%s-data", input.Name),
				SizeMB:      sizeMB,
				MountPath:   t.Volume.MountPath,
				Status:      "Pending",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
		}
	}

	if err := model.CreateTemplateContainer(&container, svc, vol, ingress); err != nil {
		return nil, err
	}

	volumeID := ""
	volumeMountPath := ""
	if vol != nil {
		volumeID = vol.ID
		volumeMountPath = vol.MountPath

		// PVC を先に作成してから Deployment を作る
		if err := job_queue.EnqueueTo(ctx, "controller", jobs.CreateVolumeJobArgs{
			VolumeID:  vol.ID,
			Namespace: project.Namespace,
			SizeMB:    vol.SizeMB,
		}, nil); err != nil {
			fmt.Printf("[service] failed to enqueue create_volume job: %v\n", err)
		}
	}

	if err := job_queue.EnqueueTo(ctx, "controller", jobs.DeployTemplateJobArgs{
		ContainerID:     containerID,
		Namespace:       project.Namespace,
		ImageRef:        imageRef,
		EnvVars:         string(envJSON),
		CPURequest:      t.Container.Resources.CPURequest,
		CPULimit:        t.Container.Resources.CPULimit,
		MemoryRequest:   t.Container.Resources.MemoryRequest,
		MemoryLimit:     t.Container.Resources.MemoryLimit,
		Ports:           string(portsJSON),
		VolumeID:        volumeID,
		VolumeMountPath: volumeMountPath,
		Command:         command,
		Args:            args,
	}, nil); err != nil {
		fmt.Printf("[service] failed to enqueue deploy_template job: %v\n", err)
	}

	if ingress != nil {
		if err := job_queue.EnqueueTo(ctx, "controller", jobs.CreateIngressJobArgs{
			ContainerID:   containerID,
			ContainerName: input.Name,
			Namespace:     project.Namespace,
			Subdomain:     ingress.Subdomain,
			HttpPort:      ingress.HttpPort,
		}, nil); err != nil {
			fmt.Printf("[service] failed to enqueue create_ingress job: %v\n", err)
		}
	}

	return map[string]interface{}{
		"data": map[string]interface{}{
			"container": container,
		},
	}, nil
}
