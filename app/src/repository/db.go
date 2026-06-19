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

	// マスターデータを挿入する
	if err := seedMasterData(); err != nil {
		return err
	}

	return nil
}

// seedMasterData は instance_sizes などのマスターテーブルに初期データを挿入する
func seedMasterData() error {
	instanceSizeList := []models.InstanceSize{
		{Size: "small",  CPURequest: "100m",  CPULimit: "500m",  MemoryRequest: "128Mi", MemoryLimit: "512Mi"},  // 小サイズ
		{Size: "medium", CPURequest: "250m",  CPULimit: "1000m", MemoryRequest: "256Mi", MemoryLimit: "1Gi"},    // 中サイズ
		{Size: "large",  CPURequest: "500m",  CPULimit: "2000m", MemoryRequest: "512Mi", MemoryLimit: "2Gi"},    // 大サイズ
	} // 挿入するインスタンスサイズ一覧

	for _, instanceSize := range instanceSizeList {
		result := Database.FirstOrCreate(&instanceSize, models.InstanceSize{Size: instanceSize.Size}) // 存在しない場合のみ挿入する
		if result.Error != nil {
			return fmt.Errorf("instance_sizes シードデータの挿入に失敗しました (size=%s): %w", instanceSize.Size, result.Error) // シードエラーを返す
		}
	}

	logger.Println("マスターデータのシードが完了しました")
	return nil
}

func AutoMigrate() error {
	// ingress_routes テーブルの旧カラムを削除する（存在する場合のみ）
	// GORM AutoMigrate はカラム削除をしないため手動で処理する
	Database.Exec("ALTER TABLE ingress_routes DROP COLUMN IF EXISTS deployment_id")
	Database.Exec("ALTER TABLE ingress_routes DROP COLUMN IF EXISTS path_prefix")
	Database.Exec("ALTER TABLE ingress_routes DROP COLUMN IF EXISTS port")
	Database.Exec("ALTER TABLE ingress_routes DROP COLUMN IF EXISTS pending_path_prefix")
	Database.Exec("ALTER TABLE ingress_routes DROP COLUMN IF EXISTS pending_port")
	Database.Exec("ALTER TABLE ingress_routes DROP COLUMN IF EXISTS tls_enabled")
	Database.Exec("ALTER TABLE ingress_routes DROP COLUMN IF EXISTS pending_tls_enabled")
	Database.Exec("ALTER TABLE ingress_routes DROP COLUMN IF EXISTS certificate_resolver")
	Database.Exec("ALTER TABLE ingress_routes DROP COLUMN IF EXISTS pending_certificate_resolver")

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
		&models.IngressRouteRoute{},
		&models.EnvVar{},
		&models.EnvVarMount{},
		&models.Volume{},
		&models.VolumeMount{},
	)
}
