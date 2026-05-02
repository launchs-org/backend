package k8slogwatcher

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// JobStatus は Job の現在の状態を表します。
type JobStatus string

const (
	// JobStatusRunning は Job が実行中であることを示します。
	JobStatusRunning JobStatus = "Running"
	// JobStatusSucceeded は Job が成功したことを示します。
	JobStatusSucceeded JobStatus = "Succeeded"
	// JobStatusFailed は Job が失敗したことを示します。
	JobStatusFailed JobStatus = "Failed"
	// JobStatusPending は Job が起動待ちであることを示します。
	JobStatusPending JobStatus = "Pending"
)

// JobLogEntry は Job Pod のログ1行を表します。
type JobLogEntry struct {
	Namespace string    // ログが属するNamespace
	JobName   string    // ログが属するJob名
	PodName   string    // ログが属するPod名
	Container string    // ログが属するContainer名
	Message   string    // ログ本文
	Timestamp time.Time // ログのタイムスタンプ
}

// JobStatusEntry は Job のステータス変化イベントを表します。
type JobStatusEntry struct {
	Namespace string    // NameSpace
	JobName   string    // Job名
	Status    JobStatus // 現在のステータス
	Message   string    // 補足メッセージ（失敗時のエラーなど）
	Timestamp time.Time // 変化した時刻
}

// JobLogCallback は Job ログ受信時に呼び出されるコールバック関数の型です。
type JobLogCallback func(entry JobLogEntry)

// JobStatusCallback は Job ステータス変化時に呼び出されるコールバック関数の型です。
type JobStatusCallback func(entry JobStatusEntry)

// JobWatcher は Kubernetes Job のログとステータスを監視するクラスです。
// Deployment 用の Watcher とは独立した実装です。
type JobWatcher struct {
	k8sClient   *kubernetes.Clientset // Kubernetes APIクライアント
	redisClient *redis.Client         // Redis接続クライアント（ログ配信用 Pub/Sub）
	pool        *jobWatchPool         // アクティブな JobWatch の管理プール
}

// NewJobWatcher は JobWatcher を初期化して返します。
// k8sClient と redisClient を直接受け取ります。
func NewJobWatcher(k8sClient *kubernetes.Clientset, redisClient *redis.Client) (*JobWatcher, error) {
	// k8sClient の必須チェック
	if k8sClient == nil {
		return nil, fmt.Errorf("k8sClient は必須です")
	}
	// redisClient の必須チェック
	if redisClient == nil {
		return nil, fmt.Errorf("redisClient は必須です")
	}

	return &JobWatcher{
		k8sClient:   k8sClient,
		redisClient: redisClient,
		pool:        newJobWatchPool(),
	}, nil
}

// Watch は指定した Job のログとステータスの監視を開始します。
// 同一 namespace/jobName への重複呼び出しは既存の JobWatch にコールバックを追記して返します。
// 完了済みの Job を指定した場合は、既存ログとステータスを送信して即座に終了します。
func (jw *JobWatcher) Watch(
	ctx context.Context,
	namespace string,
	jobName string,
	logCallback JobLogCallback,
	statusCallback JobStatusCallback,
) (*JobWatch, error) {
	// Pool から既存の Watch を検索
	existing := jw.pool.get(namespace, jobName)
	if existing != nil {
		// 既存の Watch にコールバックを追加して返す
		existing.addCallbacks(logCallback, statusCallback)
		return existing, nil
	}

	// 新規 JobWatch を作成
	watch := newJobWatch(ctx, namespace, jobName, jw.k8sClient, jw.redisClient, logCallback, statusCallback)

	// Pool に登録してバックグラウンドで動作開始
	jw.pool.set(namespace, jobName, watch)
	watch.start()

	return watch, nil
}

// Unwatch は指定した Job の監視を停止します。
func (jw *JobWatcher) Unwatch(namespace string, jobName string) {
	watch := jw.pool.get(namespace, jobName)
	if watch != nil {
		watch.stop()
		jw.pool.delete(namespace, jobName)
	}
}

// ---

// JobWatch は1つの Job に対する監視セッションを表します。
type JobWatch struct {
	namespace      string
	jobName        string
	k8sClient      *kubernetes.Clientset
	redisClient    *redis.Client
	cancelFunc     context.CancelFunc
	logCallbacks   []JobLogCallback   // ログコールバック一覧
	statCallbacks  []JobStatusCallback // ステータスコールバック一覧
	callbackMutex  sync.RWMutex       // コールバックリストの保護用ミューテックス
	lastStatus     JobStatus          // 最後に確認したステータス（変化検出用）
}

// newJobWatch は新しい JobWatch を生成します。
func newJobWatch(
	ctx context.Context,
	namespace string,
	jobName string,
	k8sClient *kubernetes.Clientset,
	redisClient *redis.Client,
	logCallback JobLogCallback,
	statusCallback JobStatusCallback,
) *JobWatch {
	_, cancel := context.WithCancel(ctx)

	// 初期コールバックを設定
	logCallbacks := []JobLogCallback{}
	if logCallback != nil {
		logCallbacks = append(logCallbacks, logCallback)
	}
	statCallbacks := []JobStatusCallback{}
	if statusCallback != nil {
		statCallbacks = append(statCallbacks, statusCallback)
	}

	return &JobWatch{
		namespace:     namespace,
		jobName:       jobName,
		k8sClient:     k8sClient,
		redisClient:   redisClient,
		cancelFunc:    cancel,
		logCallbacks:  logCallbacks,
		statCallbacks: statCallbacks,
		lastStatus:    JobStatusPending,
	}
}

// addCallbacks は既存の JobWatch に新しいコールバックを追加します。
func (jw *JobWatch) addCallbacks(logCallback JobLogCallback, statusCallback JobStatusCallback) {
	jw.callbackMutex.Lock()
	defer jw.callbackMutex.Unlock()

	if logCallback != nil {
		jw.logCallbacks = append(jw.logCallbacks, logCallback)
	}
	if statusCallback != nil {
		jw.statCallbacks = append(jw.statCallbacks, statusCallback)
	}
}

// start はバックグラウンドで Job の監視ループを開始します。
func (jw *JobWatch) start() {
	// 新しい子コンテキストで監視を実行
	watchCtx, cancel := context.WithCancel(context.Background())
	jw.cancelFunc = cancel

	go jw.runWatchLoop(watchCtx)
}

// stop は監視を停止します。
func (jw *JobWatch) stop() {
	jw.cancelFunc()
}

// runWatchLoop は Job のステータスとログを定期的にポーリングするメインループです。
func (jw *JobWatch) runWatchLoop(ctx context.Context) {
	// Redis Pub/Sub チャンネル名（ログ配信用）
	redisChannel := buildJobChannelName(jw.namespace, jw.jobName)

	// Redis 購読をバックグラウンドで開始（全プロセスがログを受信する）
	go jw.subscribeRedis(ctx, redisChannel)

	// リーダー選出（複数Podが動いていても1プロセスだけが k8s からログを取得する）
	elector := newLeaderElector(jw.redisClient, jw.namespace, "job-"+jw.jobName)
	go elector.runLeaderLoop(ctx, jw.onBecomeLeader, jw.onLoseLeader)
}

// onBecomeLeader はリーダーになった際に呼び出されます。
// k8s Job の Pod ログと状態のストリーミングを開始します。
func (jw *JobWatch) onBecomeLeader(leaderCtx context.Context) {
	// Job のログをストリーミングして Redis に発行
	redisChannel := buildJobChannelName(jw.namespace, jw.jobName)
	publisher := &jobLogPublisher{
		redisClient: jw.redisClient,
		channel:     redisChannel,
	}

	// ログ取得チャンネル
	logCh := make(chan JobLogEntry, 100)

	// Pod ログのストリーミングをゴルーチンで実行
	go jw.streamJobLogs(leaderCtx, logCh)

	// ステータス監視をゴルーチンで並行実行
	go jw.pollJobStatus(leaderCtx)

	// ログを Redis に発行
	for {
		select {
		case <-leaderCtx.Done():
			return
		case entry, ok := <-logCh:
			if !ok {
				return
			}
			// Redis チャンネルにログを発行
			publisher.publish(leaderCtx, entry)
		}
	}
}

// onLoseLeader はリーダーシップを失った際に呼び出されます。
func (jw *JobWatch) onLoseLeader() {
	// リーダー処理の停止は leaderCtx のキャンセルで行われるため、ここでは何もしない
}

// streamJobLogs は Job に紐づく Pod のログを全て logCh に送信します。
// Job の全コンテナ（InitContainer 含む）のログを順番に取得します。
func (jw *JobWatch) streamJobLogs(ctx context.Context, logCh chan<- JobLogEntry) {
	defer close(logCh)

	// Job に紐づく Pod が起動するまで待機（最大 10 分）
	pod, err := jw.waitForJobPod(ctx, 10*time.Minute)
	if err != nil {
		// タイムアウトまたはキャンセルの場合は終了
		return
	}

	// InitContainer と通常コンテナの全コンテナのログを順番に取得
	allContainers := []string{}
	for _, initContainer := range pod.Spec.InitContainers {
		allContainers = append(allContainers, initContainer.Name)
	}
	for _, container := range pod.Spec.Containers {
		allContainers = append(allContainers, container.Name)
	}

	// コンテナを順番にストリーミング（直列実行）
	for _, containerName := range allContainers {
		// このコンテナが起動するまで待機
		if err := jw.waitForContainerRunning(ctx, pod.Name, containerName); err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}

		// コンテナのログをストリーミング
		jw.streamContainerLogs(ctx, pod.Name, containerName, logCh)
	}
}

// waitForJobPod は Job に紐づく Pod が Running/Succeeded/Failed になるまで待機します。
func (jw *JobWatch) waitForJobPod(ctx context.Context, timeout time.Duration) (*corev1.Pod, error) {
	deadline := time.Now().Add(timeout)

	for {
		// タイムアウトチェック
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("job pod の起動を待機中にタイムアウトしました")
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}

		// Job に紐づく Pod を検索（ラベル job-name=<jobName>）
		pods, err := jw.k8sClient.CoreV1().Pods(jw.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", jw.jobName),
		})
		if err != nil {
			continue
		}

		// Pod が見つかり、かつ Running 以降の状態なら返す
		for idx := range pods.Items {
			pod := &pods.Items[idx]
			phase := pod.Status.Phase
			if phase == corev1.PodRunning || phase == corev1.PodSucceeded || phase == corev1.PodFailed {
				return pod, nil
			}
		}
	}
}

// waitForContainerRunning は指定したコンテナが Running または Terminated になるまで待機します。
func (jw *JobWatch) waitForContainerRunning(ctx context.Context, podName string, containerName string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}

		pod, err := jw.k8sClient.CoreV1().Pods(jw.namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		// InitContainer と通常コンテナ両方のステータスを確認
		allStatuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
		for _, containerStatus := range allStatuses {
			if containerStatus.Name != containerName {
				continue
			}
			// Running または Terminated なら OK
			if containerStatus.State.Running != nil || containerStatus.State.Terminated != nil {
				return nil
			}
		}
	}
}

// streamContainerLogs は単一コンテナのログを全行読み取り logCh へ送信します。
func (jw *JobWatch) streamContainerLogs(ctx context.Context, podName string, containerName string, logCh chan<- JobLogEntry) {
	logOptions := &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     true,  // リアルタイムでストリーミング
		Timestamps: true,  // タイムスタンプ付与
	}

	req := jw.k8sClient.CoreV1().Pods(jw.namespace).GetLogs(podName, logOptions)
	stream, err := req.Stream(ctx)
	if err != nil {
		return
	}
	defer stream.Close()

	// 行ごとにログを読み取って logCh に送信
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// "タイムスタンプ メッセージ" 形式をパース
		line := scanner.Text()
		timestamp, message := parseTimestampedLine(line)

		entry := JobLogEntry{
			Namespace: jw.namespace,
			JobName:   jw.jobName,
			PodName:   podName,
			Container: containerName,
			Message:   message,
			Timestamp: timestamp,
		}

		select {
		case <-ctx.Done():
			return
		case logCh <- entry:
		}
	}
}

// pollJobStatus は Job のステータスを定期的にポーリングしてステータス変化を通知します。
func (jw *JobWatch) pollJobStatus(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Job リソースを取得してステータスを確認
			job, err := jw.k8sClient.BatchV1().Jobs(jw.namespace).Get(ctx, jw.jobName, metav1.GetOptions{})
			if err != nil {
				continue
			}

			// 現在のステータスを判定
			currentStatus := determineJobStatus(job)

			// ステータスが変化した場合のみ通知
			jw.callbackMutex.RLock()
			lastStatus := jw.lastStatus
			jw.callbackMutex.RUnlock()

			if currentStatus != lastStatus {
				// lastStatus を更新
				jw.callbackMutex.Lock()
				jw.lastStatus = currentStatus
				jw.callbackMutex.Unlock()

				// ステータスイベントを全コールバックに送信
				entry := JobStatusEntry{
					Namespace: jw.namespace,
					JobName:   jw.jobName,
					Status:    currentStatus,
					Timestamp: time.Now(),
				}
				jw.dispatchStatusToCallbacks(entry)
			}

			// 完了（成功・失敗）したら監視を終了
			if currentStatus == JobStatusSucceeded || currentStatus == JobStatusFailed {
				return
			}
		}
	}
}

// dispatchLogToCallbacks は全登録ログコールバックに JobLogEntry を渡します。
func (jw *JobWatch) dispatchLogToCallbacks(entry JobLogEntry) {
	jw.callbackMutex.RLock()
	defer jw.callbackMutex.RUnlock()
	for _, cb := range jw.logCallbacks {
		cb(entry)
	}
}

// dispatchStatusToCallbacks は全登録ステータスコールバックに JobStatusEntry を渡します。
func (jw *JobWatch) dispatchStatusToCallbacks(entry JobStatusEntry) {
	jw.callbackMutex.RLock()
	defer jw.callbackMutex.RUnlock()
	for _, cb := range jw.statCallbacks {
		cb(entry)
	}
}

// subscribeRedis は Redis チャンネルからログを受信してコールバックに渡します。
func (jw *JobWatch) subscribeRedis(ctx context.Context, channel string) {
	pubsub := jw.redisClient.Subscribe(ctx, channel)
	defer pubsub.Close()

	msgChan := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case redisMsg, ok := <-msgChan:
			if !ok {
				return
			}

			// JSON をデシリアライズして JobLogEntry に変換
			var entry JobLogEntry
			if err := json.Unmarshal([]byte(redisMsg.Payload), &entry); err != nil {
				continue
			}

			// ログコールバックに配信
			jw.dispatchLogToCallbacks(entry)
		}
	}
}

// ---

// determineJobStatus は k8s Job リソースから現在のステータスを判定します。
func determineJobStatus(job *batchv1.Job) JobStatus {
	// 成功した Pod が 1 つ以上あれば Succeeded
	if job.Status.Succeeded > 0 {
		return JobStatusSucceeded
	}
	// 失敗した Pod が条件を超えていれば Failed
	if job.Status.Failed > 0 {
		// Job の backoffLimit を確認
		backoffLimit := int32(6) // k8s デフォルト
		if job.Spec.BackoffLimit != nil {
			backoffLimit = *job.Spec.BackoffLimit
		}
		if job.Status.Failed > backoffLimit {
			return JobStatusFailed
		}
	}
	// Active な Pod があれば Running
	if job.Status.Active > 0 {
		return JobStatusRunning
	}
	// Conditions を確認
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return JobStatusFailed
		}
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			return JobStatusSucceeded
		}
	}

	return JobStatusPending
}

// buildJobChannelName は Job 用の Redis チャンネル名を生成します。
func buildJobChannelName(namespace string, jobName string) string {
	return fmt.Sprintf("k8sjobwatcher:logs:%s:%s", namespace, jobName)
}

// parseTimestampedLine は "2006-01-02T15:04:05.000000000Z message text" 形式の行をパースします。
func parseTimestampedLine(line string) (time.Time, string) {
	// スペースで分割してタイムスタンプとメッセージを分離
	for idx, ch := range line {
		if ch == ' ' {
			ts, err := time.Parse(time.RFC3339Nano, line[:idx])
			if err == nil {
				return ts, line[idx+1:]
			}
			break
		}
	}
	return time.Now(), line
}

// ---

// jobLogPublisher は Redis チャンネルに Job ログを発行します（リーダーが使用）。
type jobLogPublisher struct {
	redisClient *redis.Client
	channel     string
}

// publish は JobLogEntry を JSON にシリアライズして Redis チャンネルに発行します。
func (publisher *jobLogPublisher) publish(ctx context.Context, entry JobLogEntry) {
	payload, err := json.Marshal(entry)
	if err != nil {
		return
	}
	publisher.redisClient.Publish(ctx, publisher.channel, payload)
}

// ---

// jobWatchPool は Job ごとの JobWatch を管理するプールです。
type jobWatchPool struct {
	entries map[string]*JobWatch // キー: "namespace/jobName"
	mutex   sync.RWMutex         // マップの読み書き保護用ミューテックス
}

// newJobWatchPool は新しい jobWatchPool を生成します。
func newJobWatchPool() *jobWatchPool {
	return &jobWatchPool{
		entries: make(map[string]*JobWatch),
	}
}

// jobPoolKey は namespace と jobName からマップのキーを生成します。
func jobPoolKey(namespace string, jobName string) string {
	return namespace + "/" + jobName
}

// get は指定した Job の JobWatch を返します。存在しない場合は nil を返します。
func (pool *jobWatchPool) get(namespace string, jobName string) *JobWatch {
	pool.mutex.RLock()
	defer pool.mutex.RUnlock()
	return pool.entries[jobPoolKey(namespace, jobName)]
}

// set は指定した Job の JobWatch をプールに登録します。
func (pool *jobWatchPool) set(namespace string, jobName string, watch *JobWatch) {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()
	pool.entries[jobPoolKey(namespace, jobName)] = watch
}

// delete は指定した Job の JobWatch をプールから削除します。
func (pool *jobWatchPool) delete(namespace string, jobName string) {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()
	delete(pool.entries, jobPoolKey(namespace, jobName))
}
