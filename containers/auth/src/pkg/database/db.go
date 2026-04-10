package database

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Instance はデータベース接続のインスタンスを保持します。
var Instance *gorm.DB

// Connect データベースに接続し、接続インスタンスを初期化します。
func Connect() error {
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

	databaseConnection, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("データベースへの接続に失敗しました: %w", err)
	}

	Instance = databaseConnection
	return nil
}
