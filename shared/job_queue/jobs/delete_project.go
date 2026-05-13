package jobs

import "github.com/riverqueue/river"

type DeleteProjectJobArgs struct {
	ProjectID string `json:"project_id"`
	Namespace string `json:"namespace"`
}

func (DeleteProjectJobArgs) Kind() string { return "delete_project" }

func (DeleteProjectJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3, Queue: "controller"}
}
