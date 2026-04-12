package models

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Database データベース接続とマイグレーションを管理する構造体
type Database struct {
	Conn *gorm.DB
}

// Instance はデータベース接続のインスタンスを保持するグローバル変数です。
var Instance *gorm.DB

// NewDatabase 新しいデータベース接続を作成します
func NewDatabase() (*Database, error) {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "db"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "acp"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "example"
	}
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "acp"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Tokyo",
		host, user, password, dbname, port)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// グローバルインスタンスを保持
	Instance = db

	return &Database{Conn: db}, nil
}

// AutoMigrate マイクロサービス全体で共有されるテーブルのマイグレーションを実行します
func (db *Database) AutoMigrate() error {
	return db.Conn.AutoMigrate(
		&User{},
		&Project{},
		&Deployment{},
		&Container{},
		&EnvVar{},
		&Port{},
	)
}
