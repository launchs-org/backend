package jobs

import "github.com/riverqueue/river"

type BuildJobArgs struct {
	BuildJobID    string `json:"build_job_id"`
	ContainerID   string `json:"container_id"`
	ImageID       string `json:"image_id"`
	ProjectID     string `json:"project_id"`
	ProjectName   string `json:"project_name"`
	ContainerName string `json:"container_name"`
	Namespace     string `json:"namespace"`
	RepositoryURL string `json:"repository_url"`
	Branch        string `json:"branch"`
	Directory     string `json:"directory"`
	BuildType     string `json:"build_type"`
}

func (BuildJobArgs) Kind() string { return "build" }

func (BuildJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 1, Queue: "builder"}
}
