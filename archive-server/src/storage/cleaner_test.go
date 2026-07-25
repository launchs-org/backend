package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCleaner_Sweep_期限切れファイルを削除する はTTLを超えたファイルが削除されることを確認する
func TestCleaner_Sweep_期限切れファイルを削除する(t *testing.T) {
	tempDir := t.TempDir()
	fileStorage := NewFileStorage(tempDir)
	cleaner := NewCleaner(fileStorage, 1*time.Hour, 1*time.Minute) // TTL1時間のクリーナーを生成する

	expiredPath := filepath.Join(tempDir, "expired-id")
	if err := os.WriteFile(expiredPath, []byte("data"), 0o644); err != nil {
		t.Fatalf("テスト用ファイルの作成に失敗しました: %v", err)
	}
	oldTime := time.Now().Add(-2 * time.Hour) // TTLを超えた古い更新時刻を設定する
	if err := os.Chtimes(expiredPath, oldTime, oldTime); err != nil {
		t.Fatalf("ファイルの更新時刻変更に失敗しました: %v", err)
	}

	cleaner.sweep() // 走査を1回実行する

	if _, err := os.Stat(expiredPath); !os.IsNotExist(err) {
		t.Fatalf("期限切れファイルが削除されていません") // 削除されていない場合はテスト失敗とする
	}
}

// TestCleaner_Sweep_期限内ファイルは削除しない はTTL未経過のファイルが残ることを確認する
func TestCleaner_Sweep_期限内ファイルは削除しない(t *testing.T) {
	tempDir := t.TempDir()
	fileStorage := NewFileStorage(tempDir)
	cleaner := NewCleaner(fileStorage, 1*time.Hour, 1*time.Minute)

	freshPath := filepath.Join(tempDir, "fresh-id")
	if err := os.WriteFile(freshPath, []byte("data"), 0o644); err != nil {
		t.Fatalf("テスト用ファイルの作成に失敗しました: %v", err)
	}

	cleaner.sweep() // 走査を1回実行する

	if _, err := os.Stat(freshPath); err != nil {
		t.Fatalf("期限内ファイルが誤って削除されました") // 削除されている場合はテスト失敗とする
	}
}
