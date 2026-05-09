package jobs

import "github.com/riverqueue/river"

type DeleteImageJobArgs struct {
	ImageName string   `json:"image_name"`
	Tags      []string `json:"tags"`
}

func (DeleteImageJobArgs) Kind() string { return "delete_image" }

func (DeleteImageJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3}
}
