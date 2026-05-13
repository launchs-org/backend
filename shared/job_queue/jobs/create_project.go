package jobs

import "github.com/riverqueue/river"

type CreateProjectJobArgs struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	Namespace   string `json:"namespace"`
	OwnerID     string `json:"owner_id"`
}

func (CreateProjectJobArgs) Kind() string { return "create_project" }

func (CreateProjectJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3, Queue: "controller"}
}
