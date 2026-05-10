package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"launchs/shared/job_queue"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/btcsuite/btcutil/base58"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ServicePort struct {
	Name   string `json:"name"`
	Port   int    `json:"port"`
	Target int    `json:"target"`
}

func UpdateService(ctx context.Context, containerID, ownerID, svcType string, ports []ServicePort, isActive bool) (map[string]interface{}, error) {
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

	svc, err := model.GetServiceByContainerID(containerID)
	if err != nil {
		return nil, err
	}

	portsJSON, err := json.Marshal(ports)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ports: %w", err)
	}

	svc.Type = svcType
	svc.Ports = string(portsJSON)
	svc.IsActive = isActive
	if err := model.UpdateService(svc); err != nil {
		return nil, err
	}

	portArgs := make([]jobs.ServicePortArgs, len(ports))
	for i, p := range ports {
		portArgs[i] = jobs.ServicePortArgs{Name: p.Name, Port: p.Port, Target: p.Target}
	}

	if err := job_queue.Enqueue(ctx, jobs.UpdateServiceJobArgs{
		ContainerID: containerID,
		Namespace:   project.Namespace,
		ServiceName: container.Name,
		ServiceType: svcType,
		Ports:       portArgs,
		IsActive:    isActive,
	}, nil); err != nil {
		fmt.Printf("[service] failed to enqueue update_service job: %v\n", err)
	}

	return map[string]interface{}{
		"data": svc,
	}, nil
}

func CreateIngress(ctx context.Context, containerID, ownerID, customDomain string, customDomainEnabled bool, httpPort int) (map[string]interface{}, error) {
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		return nil, err
	}

	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	existingIng, _ := model.GetIngressByContainerID(containerID)
	if existingIng != nil {
		return nil, errors.New("ingress already exists for this container")
	}

	hasher := sha256.Sum256([]byte(fmt.Sprintf("%s-%s", project.ID, container.ID)))
	encoded := base58.Encode(hasher[:])
	subdomain := fmt.Sprintf("%s.launchs.org", encoded)

	ing := &model.Ingress{
		ID:                  "ing_" + uuid.New().String(),
		ContainerID:         containerID,
		Subdomain:           subdomain,
		CustomDomain:        customDomain,
		CustomDomainEnabled: customDomainEnabled,
		HttpPort:            httpPort,
		TlsEnabled:          false,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	if err := model.CreateIngress(ing); err != nil {
		return nil, err
	}

	if err := job_queue.Enqueue(ctx, jobs.CreateIngressJobArgs{
		ContainerID:         containerID,
		ContainerName:       container.Name,
		Namespace:           project.Namespace,
		Subdomain:           subdomain,
		CustomDomain:        customDomain,
		CustomDomainEnabled: customDomainEnabled,
		HttpPort:            httpPort,
	}, nil); err != nil {
		fmt.Printf("[service] failed to enqueue create_ingress job: %v\n", err)
	}

	return map[string]interface{}{
		"data": ing,
	}, nil
}

func UpdateIngress(ctx context.Context, containerID, ownerID, customDomain string, customDomainEnabled bool, httpPort int) (map[string]interface{}, error) {
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		return nil, err
	}

	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	ing, err := model.GetIngressByContainerID(containerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("ingress not found")
		}
		return nil, err
	}

	ing.CustomDomain = customDomain
	ing.CustomDomainEnabled = customDomainEnabled
	ing.HttpPort = httpPort
	ing.UpdatedAt = time.Now()

	if err := model.UpdateIngress(ing); err != nil {
		return nil, err
	}

	if err := job_queue.Enqueue(ctx, jobs.UpdateIngressJobArgs{
		ContainerID:         containerID,
		ContainerName:       container.Name,
		Namespace:           project.Namespace,
		Subdomain:           ing.Subdomain,
		CustomDomain:        customDomain,
		CustomDomainEnabled: customDomainEnabled,
		HttpPort:            httpPort,
	}, nil); err != nil {
		fmt.Printf("[service] failed to enqueue update_ingress job: %v\n", err)
	}

	return map[string]interface{}{
		"data": ing,
	}, nil
}

func DeleteIngressRoute(ctx context.Context, containerID, ownerID string) (map[string]interface{}, error) {
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		return nil, err
	}

	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	ing, err := model.GetIngressByContainerID(containerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("ingressroute record not found in database")
		}
		return nil, err
	}

	if err := model.DeleteIngress(containerID); err != nil {
		return nil, err
	}

	if err := job_queue.Enqueue(ctx, jobs.DeleteIngressJobArgs{
		ContainerID:   containerID,
		ContainerName: container.Name,
		Namespace:     project.Namespace,
	}, nil); err != nil {
		fmt.Printf("[service] failed to enqueue delete_ingress job: %v\n", err)
	}

	return map[string]interface{}{
		"data": map[string]string{"id": ing.ID},
	}, nil
}
