package storage

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

// Cleaner は保存先ディレクトリを定期的に走査し、TTLを超えたアーカイブを削除する
type Cleaner struct {
	fileStorage *FileStorage
	ttl         time.Duration // アーカイブの保持期間
	interval    time.Duration // 走査の実行間隔
}

// NewCleaner は Cleaner を生成する
func NewCleaner(fileStorage *FileStorage, ttl time.Duration, interval time.Duration) *Cleaner {
	return &Cleaner{
		fileStorage: fileStorage,
		ttl:         ttl,
		interval:    interval,
	}
}

// Run は走査ループを開始する（呼び出し側で goroutine 化すること）
func (cleaner *Cleaner) Run() {
	ticker := time.NewTicker(cleaner.interval)
	defer ticker.Stop()

	cleaner.sweep() // 起動直後に一度実行する
	for range ticker.C {
		cleaner.sweep()
	}
}

// sweep は保存先ディレクトリ内のファイルを1回走査し、TTLを超えたものを削除する
func (cleaner *Cleaner) sweep() {
	entries, err := os.ReadDir(cleaner.fileStorage.BaseDir())
	if err != nil {
		log.Printf("アーカイブディレクトリの走査に失敗しました: %v", err)
		return
	}

	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		if now.Sub(info.ModTime()) < cleaner.ttl { // TTL未経過のファイルはスキップする
			continue
		}
		targetPath := filepath.Join(cleaner.fileStorage.BaseDir(), entry.Name())
		if removeErr := os.Remove(targetPath); removeErr != nil {
			log.Printf("期限切れアーカイブの削除に失敗しました (%s): %v", entry.Name(), removeErr)
			continue
		}
		log.Printf("期限切れアーカイブを削除しました: %s", entry.Name())
	}
}
