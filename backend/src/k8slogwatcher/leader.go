package k8slogwatcher

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	leaderLockTTL     = 15 * time.Second // リーダーロックの有効期限
	leaderRenewInterval = 5 * time.Second  // ロック更新間隔（TTLの1/3程度）
	leaderRetryInterval = 3 * time.Second  // リーダー取得失敗時のリトライ間隔
)

// leaderElector はRedisを使ったリーダー選出を担当します。
type leaderElector struct {
	redisClient *redis.Client
	lockKey     string // Redis上のロックキー
	ownerID     string // このプロセスの識別子（Pod名など）
}

// newLeaderElector は新しい leaderElector を生成します。
func newLeaderElector(redisClient *redis.Client, namespace string, deploymentName string) *leaderElector {
	// Pod名を識別子に使用（環境変数 POD_NAME が未設定の場合はホスト名）
	ownerID, err := os.Hostname()
	if err != nil {
		ownerID = "unknown-host"
	}
	if podName := os.Getenv("POD_NAME"); podName != "" {
		ownerID = podName
	}

	return &leaderElector{
		redisClient: redisClient,
		lockKey:     fmt.Sprintf("k8slogwatcher:leader:%s:%s", namespace, deploymentName),
		ownerID:     ownerID,
	}
}

// tryAcquire はリーダーロックの取得を試みます。
// 取得できた場合は true を返します（SET NX による排他制御）。
func (elector *leaderElector) tryAcquire(ctx context.Context) (bool, error) {
	// SET key value NX EX ttl：キーが存在しない場合のみセット
	result, err := elector.redisClient.SetNX(ctx, elector.lockKey, elector.ownerID, leaderLockTTL).Result()
	if err != nil {
		return false, fmt.Errorf("redis SetNX failed: %w", err)
	}
	return result, nil
}

// renew はリーダーロックのTTLを延長します。
// 自分がオーナーの場合のみ更新します。
func (elector *leaderElector) renew(ctx context.Context) error {
	// Luaスクリプトで「自分がオーナーのときだけ更新」をアトミックに実行
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("EXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)
	ttlSeconds := int(leaderLockTTL.Seconds())
	result, err := script.Run(ctx, elector.redisClient, []string{elector.lockKey}, elector.ownerID, ttlSeconds).Int()
	if err != nil {
		return fmt.Errorf("leader renew failed: %w", err)
	}
	if result == 0 {
		return fmt.Errorf("lost leadership: key owned by another process")
	}
	return nil
}

// release はリーダーロックを解放します。
// 自分がオーナーの場合のみ削除します。
func (elector *leaderElector) release(ctx context.Context) {
	// Luaスクリプトで「自分がオーナーのときだけ削除」をアトミックに実行
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`)
	script.Run(ctx, elector.redisClient, []string{elector.lockKey}, elector.ownerID)
}

// runLeaderLoop はリーダー選出ループを実行します。
// リーダーになった場合は onBecomeLeader を呼び出し、
// リーダーを失った場合は onLoseLeader を呼び出します。
func (elector *leaderElector) runLeaderLoop(
	ctx context.Context,
	onBecomeLeader func(ctx context.Context),
	onLoseLeader func(),
) {
	for {
		select {
		case <-ctx.Done():
			// コンテキストキャンセル時はロックを解放して終了
			elector.release(context.Background())
			return
		default:
		}

		// リーダーロック取得を試みる
		acquired, err := elector.tryAcquire(ctx)
		if err != nil || !acquired {
			// 取得失敗時は一定時間待ってリトライ
			select {
			case <-ctx.Done():
				return
			case <-time.After(leaderRetryInterval):
			}
			continue
		}

		// リーダーとして動作するための子コンテキストを生成
		leaderCtx, cancelLeader := context.WithCancel(ctx)

		// リーダー処理をゴルーチンで開始
		go onBecomeLeader(leaderCtx)

		// リーダー中はロックを定期更新し続ける
		elector.maintainLeadership(ctx, leaderCtx, cancelLeader, onLoseLeader)
	}
}

// maintainLeadership はリーダー中のロック更新ループです。
// ロック更新失敗時はリーダー処理をキャンセルして onLoseLeader を呼び出します。
func (elector *leaderElector) maintainLeadership(
	parentCtx context.Context,
	leaderCtx context.Context,
	cancelLeader context.CancelFunc,
	onLoseLeader func(),
) {
	renewTicker := time.NewTicker(leaderRenewInterval)
	defer renewTicker.Stop()
	defer cancelLeader()

	for {
		select {
		case <-parentCtx.Done():
			// 親コンテキストキャンセル時はロックを解放
			elector.release(context.Background())
			return
		case <-leaderCtx.Done():
			return
		case <-renewTicker.C:
			// ロックのTTLを更新
			if err := elector.renew(parentCtx); err != nil {
				// 更新失敗はリーダーシップ喪失
				onLoseLeader()
				return
			}
		}
	}
}
