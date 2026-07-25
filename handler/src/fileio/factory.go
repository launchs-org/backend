package fileio

import "os"

// NewUploaderFromEnv は ARCHIVE_STORAGE_MODE 環境変数に応じてアップロードクライアントを生成する。
// "local"（本番想定）の場合はクラスタ内の archive-server を、それ以外（デフォルト）の場合は
// 開発環境向けに外部の litterbox サービスを使う。
func NewUploaderFromEnv() Uploader {
	switch os.Getenv("ARCHIVE_STORAGE_MODE") {
	case "local":
		endpoint := getEnvOrDefault("ARCHIVE_SERVER_UPLOAD_URL", "http://archive-server.launchs-org:8080/archives")
		downloadBase := getEnvOrDefault("ARCHIVE_SERVER_DOWNLOAD_BASE", "http://archive-server.launchs-org:8080/archives")
		return NewArchiveServerClient(endpoint, downloadBase)
	default:
		return NewFileIOClient()
	}
}

// getEnvOrDefault は環境変数を取得し、未設定の場合はデフォルト値を返す
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
