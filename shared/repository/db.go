package repository

import (
	"app/shared/assets"
	"app/shared/logger"
	"app/shared/models"
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
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
		{Size: "small", CPURequest: "100m", CPULimit: "500m", MemoryRequest: "128Mi", MemoryLimit: "512Mi"}, // 小サイズ
		{Size: "medium", CPURequest: "250m", CPULimit: "1000m", MemoryRequest: "256Mi", MemoryLimit: "1Gi"}, // 中サイズ
		{Size: "large", CPURequest: "500m", CPULimit: "2000m", MemoryRequest: "512Mi", MemoryLimit: "2Gi"},  // 大サイズ
	} // 挿入するインスタンスサイズ一覧

	for _, instanceSize := range instanceSizeList {
		result := Database.FirstOrCreate(&instanceSize, models.InstanceSize{Size: instanceSize.Size}) // 存在しない場合のみ挿入する
		if result.Error != nil {
			return fmt.Errorf("instance_sizes シードデータの挿入に失敗しました (size=%s): %w", instanceSize.Size, result.Error) // シードエラーを返す
		}
	}

	// free プランを挿入または更新する
	freePlanAttrs := models.Plan{
		MaxProjects:              3,     // プロジェクト上限
		MaxDeployments:           5,     // デプロイメント上限
		MaxReplicasPerDeployment: 2,     // レプリカ上限
		MaxVolumes:               5,     // ボリューム数上限
		MaxVolumeSizeMB:          10240, // 1ボリューム最大サイズ（10GB）
		MaxTotalVolumeMB:         10240, // ボリューム総容量上限（10GB）
	}
	freePlan := models.Plan{Name: "free"} // 検索キー
	if err := Database.Where(models.Plan{Name: "free"}).Assign(freePlanAttrs).FirstOrCreate(&freePlan).Error; err != nil {
		return fmt.Errorf("plans (free) シードデータの挿入に失敗しました: %w", err) // シードエラーを返す
	}
	FreePlanID = freePlan.ID // free プランの ID をパッケージ変数に保持する

	// pro プランを挿入する（存在しない場合のみ）
	proPlan := models.Plan{
		Name:                     "pro",  // プラン名
		MaxProjects:              20,     // プロジェクト上限
		MaxDeployments:           50,     // デプロイメント上限
		MaxReplicasPerDeployment: 5,      // レプリカ上限
		MaxVolumes:               20,     // ボリューム数上限
		MaxVolumeSizeMB:          102400, // 1ボリューム最大サイズ（100GB）
		MaxTotalVolumeMB:         512000, // ボリューム総容量上限（500GB）
	}
	if err := Database.Where(models.Plan{Name: "pro"}).FirstOrCreate(&proPlan).Error; err != nil {
		return fmt.Errorf("plans (pro) シードデータの挿入に失敗しました: %w", err) // シードエラーを返す
	}

	// enterprise プランを挿入する（存在しない場合のみ）
	enterprisePlan := models.Plan{
		Name:                     "enterprise", // プラン名
		MaxProjects:              100,          // プロジェクト上限
		MaxDeployments:           500,          // デプロイメント上限
		MaxReplicasPerDeployment: 20,           // レプリカ上限
		MaxVolumes:               100,          // ボリューム数上限
		MaxVolumeSizeMB:          1048576,      // 1ボリューム最大サイズ（1TB）
		MaxTotalVolumeMB:         10485760,     // ボリューム総容量上限（10TB）
	}
	if err := Database.Where(models.Plan{Name: "enterprise"}).FirstOrCreate(&enterprisePlan).Error; err != nil {
		return fmt.Errorf("plans (enterprise) シードデータの挿入に失敗しました: %w", err) // シードエラーを返す
	}

	// free プランのインスタンスサイズ別上限を挿入または更新する
	freeInstanceLimitList := []models.PlanInstanceLimit{
		{PlanID: freePlan.ID, InstanceSize: "small", MaxCount: 20},  // small: 20台まで
		{PlanID: freePlan.ID, InstanceSize: "medium", MaxCount: 10}, // medium: 10台まで
		{PlanID: freePlan.ID, InstanceSize: "large", MaxCount: 5},   // large: 5台まで
	}
	for _, limitData := range freeInstanceLimitList {
		record := models.PlanInstanceLimit{PlanID: limitData.PlanID, InstanceSize: limitData.InstanceSize} // 検索キー
		if err := Database.Where(record).Assign(models.PlanInstanceLimit{MaxCount: limitData.MaxCount}).FirstOrCreate(&record).Error; err != nil {
			return fmt.Errorf("plan_instance_limits (free/%s) シードデータの挿入に失敗しました: %w", limitData.InstanceSize, err) // シードエラーを返す
		}
	}

	// pro プランのインスタンスサイズ別上限を挿入する
	proInstanceLimitList := []models.PlanInstanceLimit{
		{PlanID: proPlan.ID, InstanceSize: "small", MaxCount: 20},  // small: 20台まで
		{PlanID: proPlan.ID, InstanceSize: "medium", MaxCount: 10}, // medium: 10台まで
		{PlanID: proPlan.ID, InstanceSize: "large", MaxCount: 3},   // large: 3台まで
	}
	for _, limitData := range proInstanceLimitList {
		if err := Database.Where(models.PlanInstanceLimit{PlanID: limitData.PlanID, InstanceSize: limitData.InstanceSize}).
			FirstOrCreate(&limitData).Error; err != nil {
			return fmt.Errorf("plan_instance_limits (pro/%s) シードデータの挿入に失敗しました: %w", limitData.InstanceSize, err) // シードエラーを返す
		}
	}

	// enterprise プランのインスタンスサイズ別上限を挿入する
	enterpriseInstanceLimitList := []models.PlanInstanceLimit{
		{PlanID: enterprisePlan.ID, InstanceSize: "small", MaxCount: 100}, // small: 100台まで
		{PlanID: enterprisePlan.ID, InstanceSize: "medium", MaxCount: 50}, // medium: 50台まで
		{PlanID: enterprisePlan.ID, InstanceSize: "large", MaxCount: 20},  // large: 20台まで
	}
	for _, limitData := range enterpriseInstanceLimitList {
		if err := Database.Where(models.PlanInstanceLimit{PlanID: limitData.PlanID, InstanceSize: limitData.InstanceSize}).
			FirstOrCreate(&limitData).Error; err != nil {
			return fmt.Errorf("plan_instance_limits (enterprise/%s) シードデータの挿入に失敗しました: %w", limitData.InstanceSize, err) // シードエラーを返す
		}
	}

	// 埋め込み YAML テンプレートを deployment_templates テーブルへ upsert する
	if err := seedDeploymentTemplates(); err != nil {
		return err // テンプレートシードエラーを返す
	}

	logger.Println("マスターデータのシードが完了しました")
	return nil
}

// seedDeploymentTemplates は assets/templates/*.yaml を読み込んで deployment_templates テーブルへ upsert する
func seedDeploymentTemplates() error {
	entries, err := assets.TemplateFS.ReadDir("templates") // 埋め込みテンプレート一覧を取得する
	if err != nil {
		return fmt.Errorf("テンプレートディレクトリの読み込みに失敗しました: %w", err) // 読み込みエラーを返す
	}
	for _, entry := range entries { // 各 YAML ファイルを処理する
		data, err := assets.TemplateFS.ReadFile("templates/" + entry.Name()) // YAML ファイルを読み込む
		if err != nil {
			return fmt.Errorf("テンプレートファイルの読み込みに失敗しました (%s): %w", entry.Name(), err) // 読み込みエラーを返す
		}
		var yamlData models.TemplateYAML                        // YAML 中間構造体を定義する
		if err := yaml.Unmarshal(data, &yamlData); err != nil { // YAML をパースする
			return fmt.Errorf("テンプレート YAML のパースに失敗しました (%s): %w", entry.Name(), err) // パースエラーを返す
		}

		envVarsJSON, err := json.Marshal(yamlData.EnvVars) // 環境変数を JSON に変換する
		if err != nil {
			return fmt.Errorf("環境変数の JSON 変換に失敗しました (%s): %w", entry.Name(), err) // 変換エラーを返す
		}
		volumesJSON, err := json.Marshal(yamlData.Volumes) // ボリュームを JSON に変換する
		if err != nil {
			return fmt.Errorf("ボリュームの JSON 変換に失敗しました (%s): %w", entry.Name(), err) // 変換エラーを返す
		}

		templateRecord := models.DeploymentTemplate{ // テンプレートレコードを構築する
			Name:         yamlData.Name,                 // テンプレート名を設定する
			Description:  yamlData.Description,          // 説明を設定する
			Type:         models.DeploymentTypeImageURL, // image_url 固定
			ImageURL:     yamlData.ImageURL,             // イメージ URL を設定する
			InstanceSize: yamlData.InstanceSize,         // インスタンスサイズを設定する
			Replicas:     yamlData.Replicas,             // レプリカ数を設定する
			Command:      yamlData.Command,              // コマンドを設定する
			Args:         yamlData.Args,                 // 引数を設定する
			EnvVars:      envVarsJSON,                   // 環境変数 JSON を設定する
			Volumes:      volumesJSON,                   // ボリューム JSON を設定する
			CreatedBy:    "system",                      // システムシードであることを示す
		}
		if yamlData.Service != nil { // サービス設定がある場合は設定する
			templateRecord.ServicePort = yamlData.Service.Port                     // 公開ポートを設定する
			templateRecord.ServiceTargetPort = yamlData.Service.TargetPort         // ターゲットポートを設定する
			templateRecord.ServiceType = models.ServiceType(yamlData.Service.Type) // サービスタイプを設定する
		}
		if templateRecord.InstanceSize == "" {
			templateRecord.InstanceSize = "small" // インスタンスサイズのデフォルトを設定する
		}
		if templateRecord.Replicas == 0 {
			templateRecord.Replicas = 1 // レプリカ数のデフォルトを設定する
		}

		// 既存レコードを名前で検索して上書き upsert する（YAML 変更が即座に反映されるようにする）
		var existingTemplate models.DeploymentTemplate
		if err := Database.Where("name = ?", templateRecord.Name).First(&existingTemplate).Error; err == nil {
			templateRecord.ID = existingTemplate.ID // 既存 ID を引き継いで上書きする
		}
		if err := Database.Save(&templateRecord).Error; err != nil {
			return fmt.Errorf("テンプレートシードの upsert に失敗しました (%s): %w", yamlData.Name, err) // upsert エラーを返す
		}
	}
	return nil // シード完了を返す
}

func AutoMigrate() error {
	return Database.AutoMigrate(
		&models.InstanceSize{},
		&models.Plan{},              // plans テーブルを追加する
		&models.PlanInstanceLimit{}, // plan_instance_limits テーブルを追加する
		&models.UserQuota{},
		&models.Project{},
		&models.HarborCredential{},
		&models.Deployment{},
		&models.DeploymentBuild{},
		&models.Image{},         // ビルド成果物（イメージ）を保存するテーブル
		&models.BuildLogChunk{}, // ビルドログをchunk単位で保存するテーブル
		&models.PodLogChunk{},   // 稼働中Podのログをchunk単位で保存するテーブル
		&models.ApplyHistory{},
		&models.DeploymentApplyProgress{}, // apply進捗ステップ管理テーブルを追加する
		&models.DeploymentWebhook{},
		&models.Service{},
		&models.IngressRoute{},
		&models.PathRule{},
		&models.EnvVar{},
		&models.EnvVarMount{},
		&models.Volume{},
		&models.VolumeMount{},
		&models.DeploymentTemplate{},
		&models.DeploymentMetrics{}, // メトリクス時系列テーブルを追加する
	)
}
