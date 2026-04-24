package batch

import (
	"backend/database"
	"backend/deploy/models"
	"log"
	"os"
	"strconv"
	"time"
)

// StartLogRotation はログローテーションバッチをバックグラウンドで開始します
func StartLogRotation() {
	intervalHoursStr := os.Getenv("LOG_ROTATE_INTERVAL_HOURS")
	intervalHours, err := strconv.Atoi(intervalHoursStr)
	if err != nil {
		intervalHours = 24 // デフォルトは 24 時間
	}

	ticker := time.NewTicker(time.Duration(intervalHours) * time.Hour)
	go func() {
		for range ticker.C {
			rotateLogs()
		}
	}()

	// 初回実行
	go rotateLogs()
}

func rotateLogs() {
	log.Println("[batch] ログローテーションを開始します...")

	buildLogRetentionDaysStr := os.Getenv("BUILD_LOG_RETENTION_DAYS")
	buildLogRetentionDays, err := strconv.Atoi(buildLogRetentionDaysStr)
	if err != nil {
		buildLogRetentionDays = 30
	}

	containerLogRetentionDaysStr := os.Getenv("CONTAINER_LOG_RETENTION_DAYS")
	containerLogRetentionDays, err := strconv.Atoi(containerLogRetentionDaysStr)
	if err != nil {
		containerLogRetentionDays = 7
	}

	// 1. BuildJob.BuildLog の nil クリア (30日以上前)
	buildThreshold := time.Now().AddDate(0, 0, -buildLogRetentionDays)
	buildJobModel := &models.BuildJob{}
	affected, err := buildJobModel.RotateBuildLogs(database.DB, buildThreshold)
	if err != nil {
		log.Printf("[batch] ビルドログのローテーション失敗: %v", err)
	} else {
		log.Printf("[batch] ビルドログを %d 件クリアしました", affected)
	}

	// 2. ContainerLog の物理削除 (7日以上前)
	containerThreshold := time.Now().AddDate(0, 0, -containerLogRetentionDays)
	logModel := &models.ContainerLog{}
	affected, err = logModel.DeleteOldLogs(database.DB, containerThreshold)
	if err != nil {
		log.Printf("[batch] コンテナログの削除失敗: %v", err)
	} else {
		log.Printf("[batch] コンテナログを %d 件削除しました", affected)
	}

	log.Println("[batch] ログローテーションが完了しました")
}
