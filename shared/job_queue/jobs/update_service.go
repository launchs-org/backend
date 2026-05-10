package jobs

import "github.com/riverqueue/river"

type ServicePortArgs struct {
	Name   string `json:"name"`
	Port   int    `json:"port"`
	Target int    `json:"target"`
}

type UpdateServiceJobArgs struct {
	ContainerID string            `json:"container_id"`
	Namespace   string            `json:"namespace"`
	ServiceName string            `json:"service_name"`
	ServiceType string            `json:"service_type"`
	Ports       []ServicePortArgs `json:"ports"`
	IsActive    bool              `json:"is_active"`
}

func (UpdateServiceJobArgs) Kind() string { return "update_service" }

func (UpdateServiceJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3}
}
