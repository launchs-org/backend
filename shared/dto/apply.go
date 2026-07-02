package dto

// ApplyWorkflowInput は ApplyWorkflow に渡す入力パラメータを定義する
type ApplyWorkflowInput struct {
	DeploymentID string `json:"deployment_id"` // 対象デプロイメントのID
	BaseDomain   string `json:"base_domain"`   // ベースドメイン
}

// DeleteDeploymentWorkflowInput は DeleteDeploymentWorkflow に渡す入力パラメータを定義する
type DeleteDeploymentWorkflowInput struct {
	DeploymentID string `json:"deployment_id"` // 削除対象デプロイメントのID
}
