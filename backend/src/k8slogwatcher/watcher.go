// Package k8slogwatcher は Kubernetes Deployment / Job のログをリアルタイムで取得・配信するライブラリです。
// Redis を用いたリーダー選出により、複数Pod間での重複取得を防ぎます。
package k8slogwatcher

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"k8s.io/client-go/kubernetes"
)

// LogEntry は購読者に渡されるログの1行分のデータです。
type LogEntry struct {
	Namespace  string    `json:"namespace"`  // ログが属するNamespace
	Deployment string    `json:"deployment"` // ログが属するDeployment名
	PodName    string    `json:"pod_name"`   // ログが属するPod名
	Container  string    `json:"container"`  // ログが属するContainer名
	Message    string    `json:"message"`    // ログ本文
	Timestamp  time.Time `json:"timestamp"`  // ログのタイムスタンプ（SinceTimeとして利用）
}

// LogCallback はログ受信時に呼び出されるコールバック関数の型です。
type LogCallback func(entry LogEntry)

// Watcher はライブラリのメインエントリーポイントです（Deployment ログ監視用）。
type Watcher struct {
	k8sClient   *kubernetes.Clientset // Kubernetes APIクライアント
	redisClient *redis.Client         // Redis接続クライアント
	pool        *subscriptionPool     // アクティブなSubscriptionの管理プール
}

// NewWatcher は Watcher を初期化して返します。
// k8sClient と redisClient を直接受け取ります。
func NewWatcher(k8sClient *kubernetes.Clientset, redisClient *redis.Client) (*Watcher, error) {
	// k8sClient の必須チェック
	if k8sClient == nil {
		return nil, fmt.Errorf("k8sClient は必須です")
	}
	// redisClient の必須チェック
	if redisClient == nil {
		return nil, fmt.Errorf("redisClient は必須です")
	}

	return &Watcher{
		k8sClient:   k8sClient,
		redisClient: redisClient,
		pool:        newSubscriptionPool(),
	}, nil
}

// Subscribe は指定した Deployment のログ購読を開始します。
// sinceTime 以降のログをリアルタイムで callback に渡します。
// 同一の namespace/deployment への重複呼び出しは既存の購読を返します。
func (watcher *Watcher) Subscribe(
	ctx context.Context,
	namespace string,
	deploymentName string,
	sinceTime time.Time,
	callback LogCallback,
) (*Subscription, error) {
	// Pool から既存の購読を検索
	existing := watcher.pool.get(namespace, deploymentName)
	if existing != nil {
		// 既存購読にコールバックを追加
		existing.addCallback(callback)
		// 新規参加者向けに履歴を取得して送信
		existing.FetchHistory(ctx, sinceTime, callback)
		return existing, nil
	}

	// 新規 Subscription を作成
	sub, err := newSubscription(ctx, subscriptionOptions{
		namespace:      namespace,
		deploymentName: deploymentName,
		sinceTime:      sinceTime,
		callback:       callback,
		k8sClient:      watcher.k8sClient,
		redisClient:    watcher.redisClient,
	})
	if err != nil {
		return nil, fmt.Errorf("subscription creation failed: %w", err)
	}

	// Pool に登録してバックグラウンドで動作開始
	watcher.pool.set(namespace, deploymentName, sub)
	sub.start()

	return sub, nil
}

// Unsubscribe は指定した Deployment の購読を停止してPoolから削除します。
func (watcher *Watcher) Unsubscribe(namespace string, deploymentName string) {
	sub := watcher.pool.get(namespace, deploymentName)
	if sub != nil {
		sub.stop()
		watcher.pool.delete(namespace, deploymentName)
	}
}
