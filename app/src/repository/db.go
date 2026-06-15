package repository

import (
	"app/logger"
	"app/models"
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	// データベース
	Database *gorm.DB = nil
)

func Init() error {
	// ログを出す
	logger.Println("データベースを初期化します")

	// パスワードなどを埋め込む
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Tokyo",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	// データベースに接続を行う
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	// エラー処理
	if err != nil {
		return err
	}

	// データベースを格納
	Database = db

	// ログを出す
	logger.Println("データベースに接続しました")
	logger.Println("マイグレーションを実行します")

	// 自動マイグレーションを実行する
	err = AutoMigrate()

	// エラー処理
	if err != nil {
		return err
	}

	// ログを出す
	logger.Println("マイグレーションを実行しました")

	return nil
}

func AutoMigrate() error {
	return Database.AutoMigrate(
		&models.InstanceSize{},
		&models.UserQuota{},
		&models.Project{},
		&models.HarborCredential{},
		&models.Deployment{},
		&models.DeploymentBuild{},
		&models.BuildLogChunk{}, // ビルドログをchunk単位で保存するテーブル
		&models.PodLogChunk{},   // 稼働中Podのログをchunk単位で保存するテーブル
		&models.ApplyHistory{},
		&models.DeploymentWebhook{},
		&models.Service{},
		&models.IngressRoute{},
		&models.EnvVar{},
		&models.EnvVarMount{},
		&models.Volume{},
		&models.VolumeMount{},
	)
}
