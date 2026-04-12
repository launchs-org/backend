package models

import (
	"os"
	"testing"
)

func TestNewDatabase(t *testing.T) {
	// DB_HOST が設定されている場合のみ実行する
	if os.Getenv("DB_HOST") == "" {
		t.Skip("DB_HOST なし：DB接続テストをスキップします。ローカルでテストする場合は DB_HOST=localhost 等を指定してください。")
	}

	db, err := NewDatabase()
	if err != nil {
		t.Fatalf("データベース接続に失敗しました: %v", err)
	}

	if db.Conn == nil {
		t.Error("gorm.DB インスタンスが nil です")
	}

	// 接続確認（Ping）
	sqlDB, err := db.Conn.DB()
	if err != nil {
		t.Fatalf("SQL DB 構造体の取得に失敗しました: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("データベースへの Ping に失敗しました: %v", err)
	}
}

func TestAutoMigrate(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("DB_HOST なし：マイグレーションテストをスキップします。")
	}

	db, err := NewDatabase()
	if err != nil {
		t.Fatalf("データベース接続に失敗しました: %v", err)
	}

	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("マイグレーションに失敗しました: %v", err)
	}
}
