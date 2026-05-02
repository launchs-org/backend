package k8slogwatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisLogMessage は Redis Pub/Sub で送受信するメッセージの構造体です。
type redisLogMessage struct {
	Namespace  string    `json:"namespace"`
	Deployment string    `json:"deployment"`
	PodName    string    `json:"pod_name"`
	Container  string    `json:"container"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
}

// logPublisher は Redis チャンネルにログを発行します（リーダーが使用）。
type logPublisher struct {
	redisClient *redis.Client
	channel     string // 発行先 Redis チャンネル名
}

// newLogPublisher は新しい logPublisher を生成します。
func newLogPublisher(redisClient *redis.Client, namespace string, deploymentName string) *logPublisher {
	return &logPublisher{
		redisClient: redisClient,
		channel:     buildChannelName(namespace, deploymentName),
	}
}

// publish は LogEntry を JSON にシリアライズして Redis チャンネルに発行します。
func (publisher *logPublisher) publish(ctx context.Context, entry LogEntry) error {
	msg := redisLogMessage{
		Namespace:  entry.Namespace,
		Deployment: entry.Deployment,
		PodName:    entry.PodName,
		Container:  entry.Container,
		Message:    entry.Message,
		Timestamp:  entry.Timestamp,
	}

	// JSON にシリアライズ
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("json marshal failed: %w", err)
	}

	// Redis チャンネルに発行
	return publisher.redisClient.Publish(ctx, publisher.channel, payload).Err()
}

// logSubscriber は Redis チャンネルからログを受信して callback に渡します。
type logSubscriber struct {
	redisClient *redis.Client
	channel     string // 購読対象 Redis チャンネル名
}

// newLogSubscriber は新しい logSubscriber を生成します。
func newLogSubscriber(redisClient *redis.Client, namespace string, deploymentName string) *logSubscriber {
	return &logSubscriber{
		redisClient: redisClient,
		channel:     buildChannelName(namespace, deploymentName),
	}
}

// listen は Redis チャンネルを購読してメッセージを callback に渡します。
// コンテキストがキャンセルされるまで継続します。
func (subscriber *logSubscriber) listen(ctx context.Context, callback LogCallback) {
	// Redis チャンネルを購読
	pubsub := subscriber.redisClient.Subscribe(ctx, subscriber.channel)
	defer pubsub.Close()

	// メッセージ受信チャンネルを取得
	msgChan := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case redisMsg, ok := <-msgChan:
			if !ok {
				// チャンネルが閉じられた場合は終了
				return
			}

			// JSON をデシリアライズして LogEntry に変換
			entry, err := parseRedisMessage(redisMsg.Payload)
			if err != nil {
				continue
			}

			// コールバックを呼び出す
			callback(entry)
		}
	}
}

// parseRedisMessage は Redis のペイロード文字列を LogEntry に変換します。
func parseRedisMessage(payload string) (LogEntry, error) {
	var msg redisLogMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		return LogEntry{}, fmt.Errorf("json unmarshal failed: %w", err)
	}

	return LogEntry{
		Namespace:  msg.Namespace,
		Deployment: msg.Deployment,
		PodName:    msg.PodName,
		Container:  msg.Container,
		Message:    msg.Message,
		Timestamp:  msg.Timestamp,
	}, nil
}

// buildChannelName は namespace と deploymentName から Redis チャンネル名を生成します。
func buildChannelName(namespace string, deploymentName string) string {
	return fmt.Sprintf("k8slogwatcher:logs:%s:%s", namespace, deploymentName)
}
