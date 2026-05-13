package jobs

import "github.com/riverqueue/river"

type RolloutRestartJobArgs struct {
	ContainerID string `json:"container_id"`
	Namespace   string `json:"namespace"`
	Deployment  string `json:"deployment"`
}

func (RolloutRestartJobArgs) Kind() string { return "rollout_restart" }

func (RolloutRestartJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3, Queue: "controller"}
}
