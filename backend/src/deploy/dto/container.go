package dto

// ContainerCreateRequest はコンテナ作成リクエストの構造体です
type ContainerCreateRequest struct {
	Name          string `json:"name"`
	RepositoryURL string `json:"repository_url"`
	Branch        string `json:"branch"`
	Directory     string `json:"directory"`
	Replicas      int    `json:"replicas"`
	EnvVars       string `json:"env_vars"`
	Resources     string `json:"resources"`
}

// ContainerResponse はコンテナ情報のレスポンス構造体です
type ContainerResponse struct {
	ID            string `json:"id"`
	ProjectID     string `json:"project_id"`
	Name          string `json:"name"`
	RepositoryURL string `json:"repository_url"`
	Branch        string `json:"branch"`
	Status        string `json:"status"`
	Version       string `json:"version"`
}
