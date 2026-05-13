package jobs

import "github.com/riverqueue/river"

type DeleteContainerJobArgs struct {
	ContainerID string `json:"container_id"`
	Namespace   string `json:"namespace"`
	ImageName   string `json:"image_name"`
}

func (DeleteContainerJobArgs) Kind() string { return "delete_container" }

func (DeleteContainerJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3, Queue: "controller"}
}
