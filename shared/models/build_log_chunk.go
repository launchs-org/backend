package models

import "time"

// BuildLogChunk はビルドのログをchunk単位で保存するモデル
type BuildLogChunk struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"` // UUID主キー
	BuildID   string    `gorm:"type:uuid;not null;index"                       json:"build_id"` // ビルドID
	Content   string    `gorm:"type:text;not null"                             json:"content"` // ログの内容
	CreatedAt time.Time `                                                      json:"created_at"` // 作成日時（since差分取得・期限削除に使用）
}

func (BuildLogChunk) TableName() string { return "build_log_chunks" } // テーブル名を明示する
