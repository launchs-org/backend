package jobs

import "github.com/riverqueue/river"

type ScaleJobArgs struct {
	ContainerID string `json:"container_id"`
	Namespace   string `json:"namespace"`
	Deployment  string `json:"deployment"`
	Replicas    int    `json:"replicas"`
}

func (ScaleJobArgs) Kind() string { return "scale" }

func (ScaleJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3, Queue: "controller"}
}
