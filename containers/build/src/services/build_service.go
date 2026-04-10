package services

// BuildService ビルドとイメージ管理に関するビジネスロジックを定義するインターフェース
type BuildService interface {
	TriggerBuild(projectID string, deploymentID string) (string, error)
	GetBuildStatus(containerID string) (string, error)
	GetBuildLogs(containerID string) ([]byte, error)
}

// buildService BuildService の実装
type buildService struct {
}

// NewBuildService BuildService の新しいインスタンスを作成する
func NewBuildService() BuildService {
	return &buildService{}
}

func (service *buildService) TriggerBuild(projectID string, deploymentID string) (string, error) {
	// 実装はユーザーが行う
	return "container-uuid", nil
}

func (service *buildService) GetBuildStatus(containerID string) (string, error) {
	// 実装はユーザーが行う
	return "building", nil
}

func (service *buildService) GetBuildLogs(containerID string) ([]byte, error) {
	// 実装はユーザーが行う
	return nil, nil
}
