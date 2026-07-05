package models

import "time"

// CliToken は CLI から API を利用するための長期トークンのメタ情報を表す
// 失効はレコードの物理削除によって行う（DBに存在しないjtiは無効なトークンとして扱われる）
type CliToken struct {
	ID        string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`      // jti（JWTのjtiクレームと一致させる）
	UserID    string     `gorm:"type:varchar(255);not null;index"               json:"user_id"` // トークンを発行したユーザーID
	Name      string     `gorm:"type:varchar(255);not null"                     json:"name"`    // トークンの用途を識別するためのラベル
	ExpiresAt *time.Time `json:"expires_at"`                                                    // 有効期限（nullの場合は無期限）
	CreatedAt time.Time  `json:"created_at"`
}

func (CliToken) TableName() string { return "cli_tokens" }
