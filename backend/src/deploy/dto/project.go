package dto

// ProjectCreateRequest はプロジェクト作成リクエストの構造体です
type ProjectCreateRequest struct {
	Name            string `json:"name"`
	K8sResourceName string `json:"k8s_resource_name"`
	Namespace       string `json:"namespace"`
}

// ProjectResponse はプロジェクト情報のレスポンス構造体です
type ProjectResponse struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	K8sResourceName string              `json:"k8s_resource_name"`
	Namespace       string              `json:"namespace"`
	CreatedAt       string              `json:"created_at"`
	Containers      []ContainerResponse `json:"containers,omitempty"`
}
