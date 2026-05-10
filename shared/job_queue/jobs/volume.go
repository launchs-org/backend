package jobs

import "github.com/riverqueue/river"

type CreateVolumeJobArgs struct {
	VolumeID  string `json:"volume_id"`
	Namespace string `json:"namespace"`
	SizeMB    int    `json:"size_mb"`
}

func (CreateVolumeJobArgs) Kind() string { return "create_volume" }

func (CreateVolumeJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3}
}

type DeleteVolumeJobArgs struct {
	VolumeID  string `json:"volume_id"`
	Namespace string `json:"namespace"`
}

func (DeleteVolumeJobArgs) Kind() string { return "delete_volume" }

func (DeleteVolumeJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3}
}
