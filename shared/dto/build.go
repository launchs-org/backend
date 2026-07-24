package dto

// BuildWorkflowInput は BuildWorkflow に渡す入力パラメータを定義する
type BuildWorkflowInput struct {
	BuildID       string `json:"build_id"`       // ビルドID
	DeploymentID  string `json:"deployment_id"`  // 対象デプロイメントのID
	ProjectID     string `json:"project_id"`     // 対象プロジェクトのID
	HarborHost    string `json:"harbor_host"`    // Harbor のホスト名（クラスタ内DNS）
}

// CancelBuildWorkflowInput は CancelBuildWorkflow に渡す入力パラメータを定義する
type CancelBuildWorkflowInput struct {
	BuildID string `json:"build_id"` // キャンセル対象のビルドID
}
