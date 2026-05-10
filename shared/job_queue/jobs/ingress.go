package jobs

import "github.com/riverqueue/river"

type CreateIngressJobArgs struct {
	ContainerID         string `json:"container_id"`
	ContainerName       string `json:"container_name"`
	Namespace           string `json:"namespace"`
	Subdomain           string `json:"subdomain"`
	CustomDomain        string `json:"custom_domain"`
	CustomDomainEnabled bool   `json:"custom_domain_enabled"`
	HttpPort            int    `json:"http_port"`
}

func (CreateIngressJobArgs) Kind() string { return "create_ingress" }

func (CreateIngressJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3}
}

type UpdateIngressJobArgs struct {
	ContainerID         string `json:"container_id"`
	ContainerName       string `json:"container_name"`
	Namespace           string `json:"namespace"`
	Subdomain           string `json:"subdomain"`
	CustomDomain        string `json:"custom_domain"`
	CustomDomainEnabled bool   `json:"custom_domain_enabled"`
	HttpPort            int    `json:"http_port"`
}

func (UpdateIngressJobArgs) Kind() string { return "update_ingress" }

func (UpdateIngressJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3}
}

type DeleteIngressJobArgs struct {
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	Namespace     string `json:"namespace"`
}

func (DeleteIngressJobArgs) Kind() string { return "delete_ingress" }

func (DeleteIngressJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3}
}
