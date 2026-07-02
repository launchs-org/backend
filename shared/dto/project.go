package dto

// CreateProjectWorkflowInput は CreateProjectWorkflow に渡す入力パラメータを定義する
type CreateProjectWorkflowInput struct {
	ProjectID          string `json:"project_id"`           // 対象プロジェクトのID
	HarborStorageLimit int64  `json:"harbor_storage_limit"` // Harbor プロジェクトのストレージ上限（バイト）
}

// DeleteProjectWorkflowInput は DeleteProjectWorkflow に渡す入力パラメータを定義する
type DeleteProjectWorkflowInput struct {
	ProjectID string `json:"project_id"` // 削除対象プロジェクトのID
}

// CreateVolumeWorkflowInput は CreateVolumeWorkflow に渡す入力パラメータを定義する
type CreateVolumeWorkflowInput struct {
	VolumeID         string `json:"volume_id"`          // 対象ボリュームのID
	StorageClassName string `json:"storage_class_name"` // 使用するStorageClass名
}

// DeleteVolumeWorkflowInput は DeleteVolumeWorkflow に渡す入力パラメータを定義する
type DeleteVolumeWorkflowInput struct {
	VolumeID string `json:"volume_id"` // 削除対象ボリュームのID
}
