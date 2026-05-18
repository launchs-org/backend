package service

import (
	"launchs/shared/model"
	"strings"
)

type ContainerLogsResponse struct {
	Logs []ContainerLogEntry `json:"logs"`
}

type ContainerLogEntry struct {
	Line string `json:"line"`
}

// GetContainerLogs は DB に蓄積されたコンテナの実行ログを返します。
func GetContainerLogs(containerID, ownerID string) (*ContainerLogsResponse, error) {
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		return nil, ErrContainerNotFound
	}

	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	raw, err := model.GetContainerLog(containerID)
	if err != nil || len(raw) == 0 {
		return &ContainerLogsResponse{Logs: []ContainerLogEntry{}}, nil
	}

	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	entries := make([]ContainerLogEntry, 0, len(lines))
	for _, l := range lines {
		if l != "" {
			entries = append(entries, ContainerLogEntry{Line: l})
		}
	}

	return &ContainerLogsResponse{Logs: entries}, nil
}
