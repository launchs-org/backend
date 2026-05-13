package jobs

import "github.com/riverqueue/river"

type RolloutRestartJobArgs struct {
	ContainerID string `json:"container_id"`
	Namespace   string `json:"namespace"`
	Deployment  string `json:"deployment"`
	ImageRef    string `json:"image_ref"`
}

func (RolloutRestartJobArgs) Kind() string { return "rollout_restart" }

func (RolloutRestartJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3, Queue: "controller"}
}
