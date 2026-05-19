package jobs

import "github.com/riverqueue/river"

type DeployStatefulSetJobArgs struct {
	ContainerID   string `json:"container_id"`
	ImageRef      string `json:"image_ref"`
	Namespace     string `json:"namespace"`
	ContainerType string `json:"container_type"`
}

func (DeployStatefulSetJobArgs) Kind() string { return "deploy_statefulset" }

func (DeployStatefulSetJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3, Queue: "controller"}
}
