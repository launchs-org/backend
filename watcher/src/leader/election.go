package leader

import (
	"app/shared/logger"
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
)

const (
	advisoryLockKey = int64(1032)       // WatchDeployments 専用のアドバイザリーロックキー（ISSUE番号に対応）
	retryInterval   = 5 * time.Second   // ロック取得失敗時のリトライ間隔
)

// RunAsLeader は PostgreSQL Advisory Lock を使ってリーダーエレクションを行い、
// ロックを取得できた Pod のみ callback を実行する。
// ロック取得に失敗した場合は retryInterval ごとにリトライし続ける。
// ロックは単一の DB 接続に固定して保持するため、接続が切れると自動解放される。
func RunAsLeader(ctx context.Context, db *gorm.DB, callback func(ctx context.Context)) {
	runAsLeaderWithKey(ctx, db, advisoryLockKey, callback) // 本番用ロックキーでリーダーエレクションを実行する
}

// runAsLeaderWithKey はロックキーを指定してリーダーエレクションを実行する（テストから呼び出し可能）
func runAsLeaderWithKey(ctx context.Context, db *gorm.DB, lockKey int64, callback func(ctx context.Context)) {
	logger.Println("RunAsLeader: リーダーエレクションを開始します") // 起動ログを出力する

	for {
		if ctx.Err() != nil { // コンテキストがキャンセルされた場合は終了する
			logger.Println("RunAsLeader: コンテキストがキャンセルされました。終了します") // 終了ログを出力する
			return
		}

		sqlDB, err := db.DB() // 生の *sql.DB を取得する
		if err != nil {
			logger.PrintErr("RunAsLeader: *sql.DB の取得に失敗しました: " + err.Error()) // エラーをログ出力する
			waitWithContext(ctx, retryInterval)                                         // リトライ間隔を待機する
			continue
		}

		conn, err := sqlDB.Conn(ctx) // 単一の DB 接続を確保する（Advisory Lock はセッション単位のため固定が必要）
		if err != nil {
			logger.PrintErr("RunAsLeader: DB 接続の確保に失敗しました: " + err.Error()) // エラーをログ出力する
			waitWithContext(ctx, retryInterval)                                        // リトライ間隔を待機する
			continue
		}

		logger.Println("RunAsLeader: Advisory Lock の取得を試みます") // ロック試行ログを出力する

		acquired, err := tryAcquireLockWithConn(ctx, conn, lockKey) // 固定接続で Advisory Lock の取得を試みる
		if err != nil {
			logger.PrintErr("RunAsLeader: ロック取得中にエラーが発生しました: " + err.Error()) // エラーをログ出力する
			conn.Close()                                                                    // 接続を解放する
			waitWithContext(ctx, retryInterval)                                             // リトライ間隔を待機する
			continue
		}

		if !acquired { // ロック取得に失敗した場合は別の Pod がリーダーのためリトライする
			logger.Println("RunAsLeader: ロック取得に失敗しました。別の Pod がリーダーです。5秒後にリトライします") // 待機ログを出力する
			conn.Close()                       // 接続を解放する
			waitWithContext(ctx, retryInterval) // リトライ間隔を待機する
			continue
		}

		logger.Println("RunAsLeader: Advisory Lock を取得しました。この Pod がリーダーとして選出されました") // 選出ログを出力する
		logger.Println("RunAsLeader: リーダーとして昇格しました。Watcher を起動します")                   // 昇格ログを出力する
		callback(ctx)                                                                               // コールバックを実行する（Watcher が終了するまでブロックする）

		logger.Println("RunAsLeader: Watcher が終了しました。リーダーを退任します")   // 退任ログを出力する
		releaseLockWithConn(context.Background(), conn, lockKey)                    // ロックを明示的に解放する
		logger.Println("RunAsLeader: Advisory Lock を解放しました。リトライします") // 解放ログを出力する
		conn.Close()                                                                // 接続をプールに返す
	}
}

// tryAcquireLockWithConn は固定接続で pg_try_advisory_lock を実行してロックの取得を試みる
func tryAcquireLockWithConn(ctx context.Context, conn *sql.Conn, lockKey int64) (bool, error) {
	var acquired bool
	err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", lockKey).Scan(&acquired) // Advisory Lock の取得を試みる
	if err != nil {
		return false, err // DB エラーを返す
	}
	return acquired, nil // 取得結果を返す
}

// releaseLockWithConn は固定接続で pg_advisory_unlock を実行してロックを解放する
func releaseLockWithConn(ctx context.Context, conn *sql.Conn, lockKey int64) {
	var released bool
	err := conn.QueryRowContext(ctx, "SELECT pg_advisory_unlock($1)", lockKey).Scan(&released) // Advisory Lock を解放する
	if err != nil {
		logger.PrintErr("RunAsLeader: ロック解放に失敗しました: " + err.Error()) // エラーをログ出力する
	}
}

// waitWithContext はコンテキストがキャンセルされるか duration が経過するまで待機する
func waitWithContext(ctx context.Context, duration time.Duration) {
	select {
	case <-ctx.Done():          // コンテキストがキャンセルされた場合は即座に返る
	case <-time.After(duration): // 待機時間が経過した場合は返る
	}
}
