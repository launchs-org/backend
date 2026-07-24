package models

import "time"

// HarborCredential は Project ごとの Harbor robot account 情報を管理する
// Project 作成時に Harbor API で生成した robot account の認証情報を保持する
type HarborCredential struct {
	ID              string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ProjectID       string    `gorm:"type:uuid;not null;uniqueIndex"                 json:"project_id"` // 1プロジェクトに1つの認証情報
	RobotName       string    `gorm:"type:text;not null"                             json:"robot_name"`       // base64 エンコード済み robot アカウント名
	RobotSecret     string    `gorm:"type:text;not null"                             json:"-"`                // robot アカウントのシークレット（レスポンスに含めない）
	HarborEndpoint  string    `gorm:"type:text;not null"                             json:"harbor_endpoint"`  // Harbor のエンドポイント URL
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (HarborCredential) TableName() string { return "harbor_credentials" } // テーブル名を明示する
