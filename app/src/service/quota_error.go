package service

import "fmt"

// QuotaExceededError は quota 超過時の構造化エラー
// errors.As でハンドラー側が Resource / Current / Limit を取り出してレスポンスを組み立てる
type QuotaExceededError struct {
	Resource string // 超過したリソース名（"projects" / "deployments" / "replicas" / "volumes" / "volume_size_mb" / "total_volume_mb" / "instance:small" など）
	Current  int    // 現在の使用数または要求値
	Limit    int    // 上限値
}

func (quotaErr *QuotaExceededError) Error() string {
	return fmt.Sprintf("%s quota exceeded: %d / %d", quotaErr.Resource, quotaErr.Current, quotaErr.Limit) // エラー文字列を返す
}
