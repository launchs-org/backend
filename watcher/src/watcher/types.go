package watcher

import "time"

// JobLogMessage は Redis に Publish するビルドジョブのログメッセージ形式です。
// フロントエンドはこの JSON をデシリアライズしてリアルタイムログを表示します。
type JobLogMessage struct {
	Container string    `json:"container"` // ログを出力したコンテナ名
	Message   string    `json:"message"`   // ログ本文（タイムスタンプ除去済み）
	Timestamp time.Time `json:"timestamp"` // ログを受信した時刻
}
