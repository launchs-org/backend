package leader

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// テスト専用のロックキー（本番の advisoryLockKey と衝突しないよう別の値を使う）
// テストはシリアル実行（t.Parallel() 未使用）のためキーは共通で使用する
const testAdvisoryLockKey = int64(99991032)

// setupTestDB はテスト用の DB 接続を準備する
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Tokyo",
		getEnvOrDefault("DB_HOST", "localhost"),
		getEnvOrDefault("DB_USER", "postgres"),
		getEnvOrDefault("DB_PASSWORD", "postgres"),
		getEnvOrDefault("DB_NAME", "postgres"),
		getEnvOrDefault("DB_PORT", "5432"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{}) // DB に接続する
	if err != nil {
		t.Skipf("DB に接続できないためテストをスキップします: %v", err) // DB 未起動時はスキップする
	}

	return db
}

// setupTestConn はテスト用の固定 DB 接続を準備する
func setupTestConn(t *testing.T, db *gorm.DB) *sql.Conn {
	t.Helper()

	sqlDB, err := db.DB() // 生の *sql.DB を取得する
	if err != nil {
		t.Fatalf("*sql.DB の取得に失敗しました: %v", err)
	}

	conn, err := sqlDB.Conn(context.Background()) // 固定接続を確保する
	if err != nil {
		t.Fatalf("DB 接続の確保に失敗しました: %v", err)
	}

	t.Cleanup(func() { // テスト終了後にテスト専用ロックを解放してから接続を返す
		releaseLockWithConn(context.Background(), conn, testAdvisoryLockKey)
		conn.Close()
	})

	return conn
}

// getEnvOrDefault は環境変数を取得し、未設定の場合はデフォルト値を返す
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" { // 環境変数が設定されている場合はその値を返す
		return value
	}
	return defaultValue // 未設定の場合はデフォルト値を返す
}

// tryAcquireTestLock はテスト専用キーで Advisory Lock の取得を試みるヘルパー
func tryAcquireTestLock(ctx context.Context, conn *sql.Conn) (bool, error) {
	return tryAcquireLockWithConn(ctx, conn, testAdvisoryLockKey) // テスト専用キーでロック取得を試みる
}

// TestTryAcquireLockWithConn_ロックを取得できる はロック取得が成功することを確認する
func TestTryAcquireLockWithConn_ロックを取得できる(t *testing.T) {
	db := setupTestDB(t)         // テスト用 DB を準備する
	conn := setupTestConn(t, db) // テスト用固定接続を準備する

	acquired, err := tryAcquireTestLock(context.Background(), conn) // テスト専用キーでロック取得を試みる
	if err != nil {
		t.Fatalf("tryAcquireTestLock がエラーを返しました: %v", err)
	}
	if !acquired { // ロックが取得できることを確認する
		t.Fatal("ロックが取得できませんでした")
	}
}

// TestTryAcquireLockWithConn_別接続はロック取得に失敗する は別接続からのロック取得が失敗することを確認する
func TestTryAcquireLockWithConn_別接続はロック取得に失敗する(t *testing.T) {
	db1 := setupTestDB(t)          // テスト用 DB セッション1 を準備する
	db2 := setupTestDB(t)          // テスト用 DB セッション2 を準備する
	conn1 := setupTestConn(t, db1) // 接続1を確保する
	conn2 := setupTestConn(t, db2) // 接続2を確保する

	acquired1, err := tryAcquireTestLock(context.Background(), conn1) // 接続1でロックを取得する
	if err != nil {
		t.Fatalf("接続1の tryAcquireTestLock がエラーを返しました: %v", err)
	}
	if !acquired1 {
		t.Fatal("接続1のロックが取得できませんでした")
	}

	acquired2, err := tryAcquireTestLock(context.Background(), conn2) // 接続2でロック取得を試みる
	if err != nil {
		t.Fatalf("接続2の tryAcquireTestLock がエラーを返しました: %v", err)
	}
	if acquired2 { // 接続1がロックを保持しているため取得できないはず
		t.Fatal("接続1がロックを保持しているのに接続2がロックを取得できました")
	}
}

// TestReleaseLockWithConn_ロック解放後に別接続が取得できる はロック解放後に別接続がロックを取得できることを確認する
func TestReleaseLockWithConn_ロック解放後に別接続が取得できる(t *testing.T) {
	db1 := setupTestDB(t)          // テスト用 DB セッション1 を準備する
	db2 := setupTestDB(t)          // テスト用 DB セッション2 を準備する
	conn1 := setupTestConn(t, db1) // 接続1を確保する
	conn2 := setupTestConn(t, db2) // 接続2を確保する

	acquired1, err := tryAcquireTestLock(context.Background(), conn1) // 接続1でロックを取得する
	if err != nil || !acquired1 {
		t.Fatalf("接続1のロック取得に失敗しました: %v", err)
	}

	releaseLockWithConn(context.Background(), conn1, testAdvisoryLockKey) // 接続1でロックを明示的に解放する

	acquired2, err := tryAcquireTestLock(context.Background(), conn2) // 接続2でロック取得を試みる
	if err != nil {
		t.Fatalf("接続2の tryAcquireTestLock がエラーを返しました: %v", err)
	}
	if !acquired2 { // ロックが解放されたので接続2が取得できるはず
		t.Fatal("ロック解放後に接続2がロックを取得できませんでした")
	}
}

// TestRunAsLeader_ロック取得後にコールバックが実行される はリーダーになるとコールバックが呼ばれることを確認する
func TestRunAsLeader_ロック取得後にコールバックが実行される(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する

	ctx, cancel := context.WithCancel(context.Background()) // キャンセル可能なコンテキストを生成する

	callbackCalled := make(chan struct{}) // コールバック呼び出しを通知するチャネルを生成する

	go runAsLeaderWithKey(ctx, db, testAdvisoryLockKey, func(innerCtx context.Context) { // テスト専用キーでリーダーエレクションをバックグラウンドで実行する
		close(callbackCalled) // コールバックが呼ばれたことを通知する
		<-innerCtx.Done()     // コンテキストがキャンセルされるまでブロックする
	})

	select {
	case <-callbackCalled: // コールバックが呼ばれたことを確認する
		cancel() // コンテキストをキャンセルする
	case <-time.After(3 * time.Second): // タイムアウト
		cancel()
		t.Fatal("コールバックが呼ばれませんでした")
	}
}

// TestRunAsLeader_別接続がロックを保持している間はコールバックが実行されない は別接続がリーダーの間待機することを確認する
func TestRunAsLeader_別接続がロックを保持している間はコールバックが実行されない(t *testing.T) {
	db1 := setupTestDB(t) // テスト用 DB セッション1（先行リーダー）を準備する
	db2 := setupTestDB(t) // テスト用 DB セッション2（後続 Pod）を準備する

	conn1 := setupTestConn(t, db1)                                            // 接続1を確保する
	acquired, err := tryAcquireTestLock(context.Background(), conn1) // テスト専用キーで接続1がロックを先に取得する
	if err != nil || !acquired {
		t.Fatalf("接続1のロック取得に失敗しました: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background()) // キャンセル可能なコンテキストを生成する
	defer cancel()

	callbackCalled := false   // コールバックが呼ばれたかを記録する
	var callbackMu sync.Mutex // コールバック呼び出しの排他制御用 mutex を生成する

	go runAsLeaderWithKey(ctx, db2, testAdvisoryLockKey, func(innerCtx context.Context) { // テスト専用キーで接続2でリーダーエレクションを実行する
		callbackMu.Lock()
		callbackCalled = true // コールバックが呼ばれたことを記録する
		callbackMu.Unlock()
		<-innerCtx.Done() // コンテキストがキャンセルされるまでブロックする
	})

	time.Sleep(retryInterval + 1*time.Second) // リトライ間隔より長く待機する

	callbackMu.Lock()
	wasCalled := callbackCalled // コールバックが呼ばれていないことを確認する
	callbackMu.Unlock()

	if wasCalled { // 接続1がロックを保持しているためコールバックは呼ばれないはず
		t.Fatal("別接続がロックを保持しているのにコールバックが実行されました")
	}
}
