package config

import (
	"os"
	"strconv"
)

// Config はアプリケーション設定を取得するインターフェース
type Config interface {
	GetDBHost() string           // データベースホストを返す
	GetDBUser() string           // データベースユーザー名を返す
	GetDBPassword() string       // データベースパスワードを返す
	GetDBName() string           // データベース名を返す
	GetDBPort() string           // データベースポートを返す
	GetServerPort() string       // サーバーのリッスンポートを返す
	GetHarborEndpoint() string   // Harbor エンドポイントを返す
	GetHarborRobotName() string  // Harbor 管理用 robot アカウント名を返す
	GetHarborRobotSecret() string  // Harbor 管理用 robot アカウントのシークレットを返す
	GetStorageClassName() string        // PVC に使用する StorageClass 名を返す
	GetHarborStorageLimitBytes() int64  // Harbor プロジェクトのストレージ上限をバイト単位で返す
	GetRegistryHost() string            // ビルドジョブが使う Harbor ホスト名（クラスタ内 DNS 名）を返す
}

// EnvConfig は環境変数から設定を読み込む Config の実装
type EnvConfig struct{}

// NewEnvConfig は EnvConfig を生成するコンストラクタ
func NewEnvConfig() *EnvConfig {
	return &EnvConfig{} // EnvConfig を生成して返す
}

func (envConfig *EnvConfig) GetDBHost() string {
	return getEnv("DB_HOST", "localhost") // DB ホストを環境変数から取得する
}

func (envConfig *EnvConfig) GetDBUser() string {
	return getEnv("DB_USER", "postgres") // DB ユーザーを環境変数から取得する
}

func (envConfig *EnvConfig) GetDBPassword() string {
	return getEnv("DB_PASSWORD", "") // DB パスワードを環境変数から取得する
}

func (envConfig *EnvConfig) GetDBName() string {
	return getEnv("DB_NAME", "postgres") // DB 名を環境変数から取得する
}

func (envConfig *EnvConfig) GetDBPort() string {
	return getEnv("DB_PORT", "5432") // DB ポートを環境変数から取得する
}

func (envConfig *EnvConfig) GetServerPort() string {
	return getEnv("SERVER_PORT", "8080") // サーバーポートを環境変数から取得する
}

func (envConfig *EnvConfig) GetHarborEndpoint() string {
	return getEnv("HARBOR_ENDPOINT", "") // Harbor エンドポイントを環境変数から取得する
}

func (envConfig *EnvConfig) GetHarborRobotName() string {
	return getEnv("HARBOR_ROBOT_NAME", "") // Harbor 管理用 robot アカウント名を環境変数から取得する
}

func (envConfig *EnvConfig) GetHarborRobotSecret() string {
	return getEnv("HARBOR_ROBOT_SECRET", "") // Harbor 管理用 robot アカウントのシークレットを環境変数から取得する
}

func (envConfig *EnvConfig) GetStorageClassName() string {
	return getEnv("STORAGE_CLASS_NAME", "longhorn") // PVC に使用する StorageClass 名を環境変数から取得する
}

func (envConfig *EnvConfig) GetHarborStorageLimitBytes() int64 {
	rawValue := getEnv("HARBOR_STORAGE_LIMIT_BYTES", "10737418240") // Harbor ストレージ上限を環境変数から取得する（デフォルト 10GB）
	parsedValue, parseErr := strconv.ParseInt(rawValue, 10, 64)     // 文字列を int64 に変換する
	if parseErr != nil {
		return 10737418240 // パース失敗時はデフォルト値（10GB）を返す
	}
	return parsedValue // 変換した値を返す
}

func (envConfig *EnvConfig) GetRegistryHost() string {
	return getEnv("REGISTRY_HOST", "harbor.main-harbor") // ビルドジョブが使う Harbor ホスト名を環境変数から取得する（デフォルトはクラスタ内 DNS 名）
}

// getEnv は環境変数を取得し、未設定の場合はデフォルト値を返す
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key) // 環境変数を取得する
	if value == "" {
		return defaultValue // 未設定の場合はデフォルト値を返す
	}
	return value
}
