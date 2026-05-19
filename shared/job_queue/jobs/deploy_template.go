package jobs

import "github.com/riverqueue/river"

type DeployTemplateJobArgs struct {
	ContainerID     string `json:"container_id"`
	Namespace       string `json:"namespace"`
	ImageRef        string `json:"image_ref"`
	EnvVars         string `json:"env_vars"`
	CPURequest      string `json:"cpu_request"`
	CPULimit        string `json:"cpu_limit"`
	MemoryRequest   string `json:"memory_request"`
	MemoryLimit     string `json:"memory_limit"`
	Ports           string `json:"ports"`
	VolumeID        string `json:"volume_id"`
	VolumeMountPath string `json:"volume_mount_path"`
	Command         string `json:"command"`
	Args            string `json:"args"`
}

func (DeployTemplateJobArgs) Kind() string { return "deploy_template" }

func (DeployTemplateJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3, Queue: "controller"}
}
