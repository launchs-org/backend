package database

import (
	"context"
	"os"

	"github.com/redis/go-redis/v9"
)

// RedisClient は Redis クライアントのグローバル変数です
var RedisClient *redis.Client

// InitRedis は Redis 接続を初期化します
func InitRedis() {
	// 環境変数から Redis の URL を取得 (デフォルトは localhost:6379)
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}

	// Redis クライアントを作成
	RedisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// 接続確認
	ctx := context.Background()
	if err := RedisClient.Ping(ctx).Err(); err != nil {
		// 接続失敗時はエラーを出力するがパニックにはしない (任意)
		// 今回は Redis が必須の機能があるためパニックにする
		panic("failed to connect to redis: " + err.Error())
	}
}
