package database

import (
	"database/sql"
	"os"

	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// TaskDB は River ジョブキュー専用の DB 接続（taskdb）
var TaskDB *sql.DB

func Init() {
	dsn := os.Getenv("DATABASE_DSN")

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database: " + err.Error())
	}
}

func InitTaskDB() {
	dsn := os.Getenv("TASK_DATABASE_DSN")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic("failed to connect task database: " + err.Error())
	}
	if err := db.Ping(); err != nil {
		panic("failed to ping task database: " + err.Error())
	}
	TaskDB = db
}
