package jobs

import "github.com/riverqueue/river"

type DeployJobArgs struct {
	ContainerID string `json:"container_id"`
	ImageRef    string `json:"image_ref"`
	Namespace   string `json:"namespace"`
	BuildJobID  string `json:"build_job_id"`
}

func (DeployJobArgs) Kind() string { return "deploy" }

func (DeployJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 1, Queue: "controller"}
}
