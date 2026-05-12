package watcher

import (
	"strings"
	"time"
)

// jobNameToBuildJobID は K8s Job 名を BuildJob ID 形式に変換します。
// 例: "railpack-bj-abc-123" → "bj_abc-123"
// "railpack-" プレフィックスを除去し、最初の "-" を "_" に置き換えます。
func jobNameToBuildJobID(jobName string) string {
	if !strings.HasPrefix(jobName, "railpack-") {
		return ""
	}
	// "railpack-" を除去: "bj-abc-123"
	withoutPrefix := strings.TrimPrefix(jobName, "railpack-")
	// 先頭の "-" だけ "_" に置換: "bj_abc-123"
	return withoutPrefix
}

// parseTimestampedLine は K8s ログ行の先頭にある RFC3339Nano タイムスタンプを
// パースして (timestamp, message) に分解します。
// パース失敗時は現在時刻と元の行全体を返します。
func parseTimestampedLine(line string) (time.Time, string) {
	for i, ch := range line {
		if ch == ' ' {
			ts, err := time.Parse(time.RFC3339Nano, line[:i])
			if err == nil {
				return ts, line[i+1:]
			}
			break
		}
	}
	// タイムスタンプが見つからない場合はそのまま返す
	return time.Now(), line
}
