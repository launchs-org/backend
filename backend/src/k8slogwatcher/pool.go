package k8slogwatcher

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"k8s.io/client-go/kubernetes"
)

// subscriptionOptions は Subscription 生成時のオプションをまとめた構造体です。
type subscriptionOptions struct {
	namespace      string
	deploymentName string
	sinceTime      time.Time
	callback       LogCallback
	k8sClient      *kubernetes.Clientset
	redisClient    *redis.Client
}

// Subscription は1つの Deployment に対するログ購読を表します。
// リーダー選出・ログストリーミング・Redis Pub/Sub を統合管理します。
type Subscription struct {
	namespace      string
	deploymentName string
	callbacks      []LogCallback  // 登録されたコールバック一覧
	callbackMutex  sync.RWMutex   // コールバックリストの保護用ミューテックス
	cancelFunc     context.CancelFunc
	elector        *leaderElector
	streamer       *logStreamer
	publisher      *logPublisher
	subscriber     *logSubscriber
}

// newSubscription は新しい Subscription を生成します（start は別途呼び出す）。
func newSubscription(ctx context.Context, opts subscriptionOptions) (*Subscription, error) {
	_, cancel := context.WithCancel(ctx)

	sub := &Subscription{
		namespace:      opts.namespace,
		deploymentName: opts.deploymentName,
		callbacks:      []LogCallback{opts.callback},
		cancelFunc:     cancel,
		elector:        newLeaderElector(opts.redisClient, opts.namespace, opts.deploymentName),
		streamer:       newLogStreamer(opts.k8sClient, opts.namespace, opts.deploymentName),
		publisher:      newLogPublisher(opts.redisClient, opts.namespace, opts.deploymentName),
		subscriber:     newLogSubscriber(opts.redisClient, opts.namespace, opts.deploymentName),
	}

	return sub, nil
}

// start はバックグラウンドでリーダー選出と Redis 購読を開始します。
func (sub *Subscription) start() {
	// 親コンテキストは stop() で cancelFunc により停止
	subCtx, cancel := context.WithCancel(context.Background())
	sub.cancelFunc = cancel

	// Redis チャンネルの購読を開始（全プロセスが受信する）
	go sub.subscriber.listen(subCtx, sub.dispatchToCallbacks)

	// リーダー選出ループを開始（リーダーのみログをストリーミング）
	go sub.elector.runLeaderLoop(subCtx, sub.onBecomeLeader, sub.onLoseLeader)
}

// stop は購読を停止してリソースを解放します。
func (sub *Subscription) stop() {
	sub.cancelFunc()
}

// addCallback は新しいコールバックを Subscription に追加します。
// 同じ Deployment への複数回の Subscribe で利用されます。
func (sub *Subscription) addCallback(callback LogCallback) {
	sub.callbackMutex.Lock()
	defer sub.callbackMutex.Unlock()
	sub.callbacks = append(sub.callbacks, callback)
}

// dispatchToCallbacks は全登録コールバックに LogEntry を渡します。
// Redis 購読受信時に呼び出されます。
func (sub *Subscription) dispatchToCallbacks(entry LogEntry) {
	sub.callbackMutex.RLock()
	defer sub.callbackMutex.RUnlock()
	for _, callback := range sub.callbacks {
		callback(entry)
	}
}

// onBecomeLeader はリーダーになった際に呼び出されます。
// Kubernetes のログをストリーミングして Redis に発行します。
func (sub *Subscription) onBecomeLeader(leaderCtx context.Context) {
	// ログエントリを受け取るバッファ付きチャンネル
	logEntryChan := make(chan LogEntry, 100)

	// Pod ログのストリーミングを開始
	go sub.streamer.streamAll(leaderCtx, time.Now(), logEntryChan)

	// ストリーミングされたログを Redis に発行
	for {
		select {
		case <-leaderCtx.Done():
			return
		case entry, ok := <-logEntryChan:
			if !ok {
				return
			}
			// Redis チャンネルにログを発行（エラーは無視して継続）
			sub.publisher.publish(leaderCtx, entry)
		}
	}
}

// onLoseLeader はリーダーシップを失った際に呼び出されます。
// 現在は特別な処理なし（次のリーダー選出ループでフォロワーに戻る）。
func (sub *Subscription) onLoseLeader() {
	// リーダー処理の停止は leaderCtx のキャンセルで行われるため、ここでは何もしない
}

// subscriptionPool は Deployment ごとの Subscription を管理するプールです。
type subscriptionPool struct {
	entries map[string]*Subscription // キー: "namespace/deploymentName"
	mutex   sync.RWMutex             // マップの読み書き保護用ミューテックス
}

// newSubscriptionPool は新しい subscriptionPool を生成します。
func newSubscriptionPool() *subscriptionPool {
	return &subscriptionPool{
		entries: make(map[string]*Subscription),
	}
}

// poolKey は namespace と deploymentName からマップのキーを生成します。
func poolKey(namespace string, deploymentName string) string {
	return namespace + "/" + deploymentName
}

// get は指定した Deployment の Subscription を返します。存在しない場合は nil を返します。
func (pool *subscriptionPool) get(namespace string, deploymentName string) *Subscription {
	pool.mutex.RLock()
	defer pool.mutex.RUnlock()
	return pool.entries[poolKey(namespace, deploymentName)]
}

// set は指定した Deployment の Subscription をプールに登録します。
func (pool *subscriptionPool) set(namespace string, deploymentName string, sub *Subscription) {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()
	pool.entries[poolKey(namespace, deploymentName)] = sub
}

// delete は指定した Deployment の Subscription をプールから削除します。
func (pool *subscriptionPool) delete(namespace string, deploymentName string) {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()
	delete(pool.entries, poolKey(namespace, deploymentName))
}
