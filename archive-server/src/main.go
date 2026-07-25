package main

import (
	"archive-server/handler"
	"archive-server/storage"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
)

func main() {
	log.Println("archive-server を起動します") // 起動ログを出力する

	storageDir := os.Getenv("ARCHIVE_STORAGE_DIR") // アーカイブ保存先ディレクトリを環境変数から取得する
	if storageDir == "" {
		storageDir = "/data/archives" // デフォルトの保存先ディレクトリ
	}

	ttl := parseTTL(os.Getenv("ARCHIVE_TTL_MINUTES"), 60*time.Minute) // アーカイブの保持期間を環境変数から取得する（デフォルト1時間）

	archiveStorage := storage.NewFileStorage(storageDir) // ファイルストレージを初期化する

	cleaner := storage.NewCleaner(archiveStorage, ttl, 5*time.Minute) // TTL経過ファイルを定期削除するクリーナーを生成する
	go cleaner.Run()                                                  // クリーナーをバックグラウンドで起動する

	archiveHandler := handler.NewArchiveHandler(archiveStorage) // アーカイブハンドラーを生成する

	echoRouter := echo.New() // Echoルーターを初期化する
	echoRouter.HideBanner = true

	echoRouter.GET("/health", func(echoCtx echo.Context) error { // ヘルスチェックエンドポイント
		return echoCtx.String(http.StatusOK, "ok")
	})
	echoRouter.POST("/archives", archiveHandler.Upload)       // アーカイブ保存エンドポイント（handler からの内部呼び出し用）
	echoRouter.GET("/archives/:id", archiveHandler.Download)  // アーカイブ配信エンドポイント（builder からのダウンロード用）
	echoRouter.DELETE("/archives/:id", archiveHandler.Delete) // アーカイブ即時削除エンドポイント

	serverPort := os.Getenv("SERVER_PORT") // リッスンポートを環境変数から取得する
	if serverPort == "" {
		serverPort = "8080"
	}

	log.Fatal(echoRouter.Start(":" + serverPort)) // サーバーを起動する
}

// parseTTL は環境変数の文字列を分単位のTTLとしてパースし、失敗時はデフォルト値を返す
func parseTTL(raw string, defaultValue time.Duration) time.Duration {
	if raw == "" {
		return defaultValue
	}
	minutes, err := time.ParseDuration(raw + "m")
	if err != nil {
		return defaultValue
	}
	return minutes
}
