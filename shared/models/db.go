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

// DatabaseConfig データベース接続設定を保持する構造体
type DatabaseConfig struct {
	Host     string
	User     string
	Password string
	DBName   string
	Port     string
	SSLMode  string
	TimeZone string
}

// DefaultConfig は環境変数からデフォルトのデータベース設定を取得します
func DefaultConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Host:     getEnv("DB_HOST", "db"),
		User:     getEnv("DB_USER", "acp"),
		Password: getEnv("DB_PASSWORD", "example"),
		DBName:   getEnv("DB_NAME", "acp"),
		Port:     getEnv("DB_PORT", "5432"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
		TimeZone: getEnv("DB_TIMEZONE", "Asia/Tokyo"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// Instance はデータベース接続のインスタンスを保持するグローバル変数です。
var Instance *gorm.DB

// NewDatabase デフォルト設定でデータベース接続を作成します
func NewDatabase() (*Database, error) {
	return NewDatabaseWithConfig(DefaultConfig())
}

// NewDatabaseWithConfig 指定された設定でデータベース接続を作成します
func NewDatabaseWithConfig(config *DatabaseConfig) (*Database, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		config.Host, config.User, config.Password, config.DBName, config.Port, config.SSLMode, config.TimeZone)

	return Open(postgres.Open(dsn))
}

// Open 指定された gorm.Dialector でデータベース接続を開きます
func Open(dialector gorm.Dialector) (*Database, error) {
	db, err := gorm.Open(dialector, &gorm.Config{})
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
