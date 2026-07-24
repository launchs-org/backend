package models

// ResolvedQuota はプランの上限値とユーザー個別上書きを統合した実効上限値を保持する
// サービス層はこの構造体を通じて上限値を参照し、プランや override の実装詳細を意識しない
type ResolvedQuota struct {
	MaxProjects              int            // プロジェクト上限数
	MaxDeployments           int            // デプロイメント上限数
	MaxReplicasPerDeployment int            // デプロイメントあたりのレプリカ上限
	MaxVolumes               int            // ボリューム数上限
	MaxVolumeSizeMB          int            // 1ボリュームあたりの最大サイズ（MB）
	MaxTotalVolumeMB         int            // ボリューム総容量上限（MB）
	InstanceLimits           map[string]int // インスタンスサイズ -> デプロイメント数上限（例: {"small": 10, "large": 2}）
}
