package leader

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	leaderKey     = "watcher:leader"
	leaseTTL      = 15 * time.Second
	renewInterval = 10 * time.Second
	retryInterval = 5 * time.Second
)

// RunWithLeaderElection は Redis SETNX でリーダー選出を行い、リーダーになった場合のみ fn を実行します。
// リーダーシップを失った場合は fn のコンテキストをキャンセルし、再選出を試みます。
func RunWithLeaderElection(ctx context.Context, redisClient *redis.Client, fn func(ctx context.Context)) {
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		podName = "watcher-1"
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ok, err := redisClient.SetNX(ctx, leaderKey, podName, leaseTTL).Result()
		if err != nil {
			fmt.Printf("[leader] Redis error: %v, retrying...\n", err)
			sleep(ctx, retryInterval)
			continue
		}

		if !ok {
			// 他のインスタンスがリーダー — スタンバイ
			sleep(ctx, retryInterval)
			continue
		}

		fmt.Printf("[leader] %s became leader\n", podName)

		leaderCtx, cancel := context.WithCancel(ctx)

		// TTL 更新ゴルーチン
		go func() {
			defer cancel()
			ticker := time.NewTicker(renewInterval)
			defer ticker.Stop()
			for {
				select {
				case <-leaderCtx.Done():
					return
				case <-ticker.C:
					val, err := redisClient.Get(leaderCtx, leaderKey).Result()
					if err != nil || val != podName {
						fmt.Printf("[leader] %s lost leadership\n", podName)
						return
					}
					redisClient.Expire(leaderCtx, leaderKey, leaseTTL)
				}
			}
		}()

		fn(leaderCtx)
		cancel()

		fmt.Printf("[leader] %s leader session ended, re-electing...\n", podName)
	}
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
