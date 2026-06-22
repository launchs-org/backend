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
	// データベース
	Database *gorm.DB = nil

	// FreePlanID は seed で作成した free プランの ID を保持する
	// NewQuotaService の defaultPlanID に渡す用途で使用する
	FreePlanID string = ""
)

func Init() error {
	// ログを出す
	logger.Println("データベースを初期化します")

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

	// データベースを格納
	Database = db

	// ログを出す
	logger.Println("データベースに接続しました")
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

// seedMasterData は instance_sizes・plans などのマスターテーブルに初期データを挿入する
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

	// free プランを挿入する（存在しない場合のみ）
	freePlan := models.Plan{
		Name:                     "free",  // プラン名
		MaxProjects:              3,       // プロジェクト上限
		MaxDeployments:           5,       // デプロイメント上限
		MaxReplicasPerDeployment: 2,       // レプリカ上限
		MaxVolumes:               5,       // ボリューム数上限
		MaxVolumeSizeMB:          10240,   // 1ボリューム最大サイズ（10GB）
		MaxTotalVolumeMB:         51200,   // ボリューム総容量上限（50GB）
	}
	if err := Database.Where(models.Plan{Name: "free"}).FirstOrCreate(&freePlan).Error; err != nil {
		return fmt.Errorf("plans (free) シードデータの挿入に失敗しました: %w", err) // シードエラーを返す
	}
	FreePlanID = freePlan.ID // free プランの ID をパッケージ変数に保持する

	// pro プランを挿入する（存在しない場合のみ）
	proPlan := models.Plan{
		Name:                     "pro",   // プラン名
		MaxProjects:              20,      // プロジェクト上限
		MaxDeployments:           50,      // デプロイメント上限
		MaxReplicasPerDeployment: 5,       // レプリカ上限
		MaxVolumes:               20,      // ボリューム数上限
		MaxVolumeSizeMB:          102400,  // 1ボリューム最大サイズ（100GB）
		MaxTotalVolumeMB:         512000,  // ボリューム総容量上限（500GB）
	}
	if err := Database.Where(models.Plan{Name: "pro"}).FirstOrCreate(&proPlan).Error; err != nil {
		return fmt.Errorf("plans (pro) シードデータの挿入に失敗しました: %w", err) // シードエラーを返す
	}

	// enterprise プランを挿入する（存在しない場合のみ）
	enterprisePlan := models.Plan{
		Name:                     "enterprise", // プラン名
		MaxProjects:              100,           // プロジェクト上限
		MaxDeployments:           500,           // デプロイメント上限
		MaxReplicasPerDeployment: 20,            // レプリカ上限
		MaxVolumes:               100,           // ボリューム数上限
		MaxVolumeSizeMB:          1048576,       // 1ボリューム最大サイズ（1TB）
		MaxTotalVolumeMB:         10485760,      // ボリューム総容量上限（10TB）
	}
	if err := Database.Where(models.Plan{Name: "enterprise"}).FirstOrCreate(&enterprisePlan).Error; err != nil {
		return fmt.Errorf("plans (enterprise) シードデータの挿入に失敗しました: %w", err) // シードエラーを返す
	}

	// free プランのインスタンスサイズ別上限を挿入する
	freeInstanceLimitList := []models.PlanInstanceLimit{
		{PlanID: freePlan.ID, InstanceSize: "small",  MaxCount: 5}, // small: 5台まで
		{PlanID: freePlan.ID, InstanceSize: "medium", MaxCount: 2}, // medium: 2台まで
		{PlanID: freePlan.ID, InstanceSize: "large",  MaxCount: 0}, // large: 使用不可
	}
	for _, limitData := range freeInstanceLimitList {
		if err := Database.Where(models.PlanInstanceLimit{PlanID: limitData.PlanID, InstanceSize: limitData.InstanceSize}).
			FirstOrCreate(&limitData).Error; err != nil {
			return fmt.Errorf("plan_instance_limits (free/%s) シードデータの挿入に失敗しました: %w", limitData.InstanceSize, err) // シードエラーを返す
		}
	}

	// pro プランのインスタンスサイズ別上限を挿入する
	proInstanceLimitList := []models.PlanInstanceLimit{
		{PlanID: proPlan.ID, InstanceSize: "small",  MaxCount: 20}, // small: 20台まで
		{PlanID: proPlan.ID, InstanceSize: "medium", MaxCount: 10}, // medium: 10台まで
		{PlanID: proPlan.ID, InstanceSize: "large",  MaxCount: 3},  // large: 3台まで
	}
	for _, limitData := range proInstanceLimitList {
		if err := Database.Where(models.PlanInstanceLimit{PlanID: limitData.PlanID, InstanceSize: limitData.InstanceSize}).
			FirstOrCreate(&limitData).Error; err != nil {
			return fmt.Errorf("plan_instance_limits (pro/%s) シードデータの挿入に失敗しました: %w", limitData.InstanceSize, err) // シードエラーを返す
		}
	}

	// enterprise プランのインスタンスサイズ別上限を挿入する
	enterpriseInstanceLimitList := []models.PlanInstanceLimit{
		{PlanID: enterprisePlan.ID, InstanceSize: "small",  MaxCount: 100}, // small: 100台まで
		{PlanID: enterprisePlan.ID, InstanceSize: "medium", MaxCount: 50},  // medium: 50台まで
		{PlanID: enterprisePlan.ID, InstanceSize: "large",  MaxCount: 20},  // large: 20台まで
	}
	for _, limitData := range enterpriseInstanceLimitList {
		if err := Database.Where(models.PlanInstanceLimit{PlanID: limitData.PlanID, InstanceSize: limitData.InstanceSize}).
			FirstOrCreate(&limitData).Error; err != nil {
			return fmt.Errorf("plan_instance_limits (enterprise/%s) シードデータの挿入に失敗しました: %w", limitData.InstanceSize, err) // シードエラーを返す
		}
	}

	logger.Println("マスターデータのシードが完了しました")
	return nil
}

func AutoMigrate() error {
	return Database.AutoMigrate(
		&models.InstanceSize{},
		&models.Plan{},             // plans テーブルを追加する
		&models.PlanInstanceLimit{}, // plan_instance_limits テーブルを追加する
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
		&models.PathRule{},
		&models.EnvVar{},
		&models.EnvVarMount{},
		&models.Volume{},
		&models.VolumeMount{},
	)
}
