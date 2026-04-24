// Package railpack は Kubernetes 上で BuildKit を使ったコンテナイメージビルドを
// シンプルに実行するためのライブラリです。
//
// 基本的な使い方:
//
//	client, err := railpack.New(clientset, railpack.BuildConfig{
//	    GitRepo:        "https://github.com/org/repo",
//	    ImageName:      "my-app",
//	    ImageTag:        "v1.0.0",
//	    UploadEndpoint: "http://10.10.11.8:8080/upload",
//	    UploadToken:    "secret-token",
//	    Namespace:      "buildkit",
//	})
//
//	jobID, err    := client.Build(ctx)
//	status, err   := client.Status(ctx, jobID)
//	logCh, errCh := client.StreamLogs(ctx, jobID)
//	err           = client.Cancel(ctx, jobID)
package railpack

import (
	"bufio"
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Client はビルドパイプラインの操作インターフェースです。
// New() で作成し、Build / Status / StreamLogs / Cancel を呼び出して使います。
type Client struct {
	clientset *kubernetes.Clientset
	config    BuildConfig
}

// New は Client を生成します。
// config には BuildConfig を渡してください。省略可能なフィールドにはデフォルト値が適用されます。
func New(clientset *kubernetes.Clientset, config BuildConfig) (*Client, error) {
	if clientset == nil {
		return nil, fmt.Errorf("clientset は必須です")
	}
	if config.GitRepo == "" {
		return nil, fmt.Errorf("GitRepo は必須です")
	}
	if config.UploadEndpoint == "" {
		return nil, fmt.Errorf("UploadEndpoint は必須です")
	}
	if config.Namespace == "" {
		return nil, fmt.Errorf("Namespace は必須です")
	}
	if config.ImageName == "" {
		return nil, fmt.Errorf("ImageName は必須です")
	}
	if config.ImageTag == "" {
		return nil, fmt.Errorf("ImageTag は必須です")
	}

	config = applyDefaults(config)

	return &Client{
		clientset: clientset,
		config:    config,
	}, nil
}

// Build はビルドジョブを Kubernetes 上に作成し、jobID を返します。
// jobID を使って Status / StreamLogs / Cancel を呼び出してください。
// この関数はジョブを起動するだけで、完了を待ちません。
func (client *Client) Build(ctx context.Context) (jobID string, err error) {
	jobID, err = createJob(ctx, client.clientset, client.config)
	if err != nil {
		return "", fmt.Errorf("ジョブの作成に失敗しました: %w", err)
	}
	return jobID, nil
}

// Status は指定した jobID の現在の状態を返します。
//
// 戻り値は以下のいずれかです:
//   - StatusInit     — Job作成済み、Pod起動待ち
//   - StatusRunning  — ビルド実行中
//   - StatusComplete — ビルド成功
//   - StatusFailed   — ビルド失敗
func (client *Client) Status(ctx context.Context, jobID string) (BuildStatus, error) {
	status, err := getJobStatus(ctx, client.clientset, client.config.Namespace, jobID)
	if err != nil {
		return StatusFailed, fmt.Errorf("ステータスの取得に失敗しました: %w", err)
	}
	return status, nil
}

// StreamLogs は指定した jobID のビルドログをチャンネルで返します。
// logCh にログの各行が、errCh にエラーまたは nil (正常終了) が送られます。
// どちらのチャンネルもビルド完了時または ctx キャンセル時にクローズされます。
// ログには "[コンテナ名]" のプレフィックスが付きます。
//
// 実行順にログを取得するコンテナ:
//   - git-clone    (InitContainer)
//   - railpack     (InitContainer)
//   - buildctl     (メインコンテナ)
//   - tar-push     (メインコンテナ)
//
// 使用例:
//
//	logCh, errCh := client.StreamLogs(ctx, jobID)
//	for line := range logCh {
//	    log.Println(line)
//	}
//	if err := <-errCh; err != nil {
//	    log.Printf("ストリームエラー: %v", err)
//	}
func (client *Client) StreamLogs(ctx context.Context, jobID string) (<-chan string, <-chan error) {
	logCh := make(chan string, 100)
	errCh := make(chan error, 1)

	// 実行順にログを取得するコンテナ名の一覧
	// InitContainer → メインコンテナの順で直列に実行されるため、この順番で追う
	containerNames := []string{"git-clone", "railpack", "buildctl", "tar-push"}

	go func() {
		defer close(logCh)
		defer close(errCh)

		// Pod が現れるまで待機
		pod, err := waitForPod(ctx, client.clientset, client.config.Namespace, jobID)
		if err != nil {
			errCh <- fmt.Errorf("Pod の起動待ちに失敗しました: %w", err)
			return
		}

		// コンテナを順番にストリーム
		for _, containerName := range containerNames {
			// このコンテナが起動するまで待機
			if err := waitForContainerRunning(ctx, client.clientset, client.config.Namespace, pod.Name, containerName); err != nil {
				if ctx.Err() != nil {
					errCh <- ctx.Err()
				} else {
					errCh <- fmt.Errorf("[%s] コンテナの起動待ちに失敗しました: %w", containerName, err)
				}
				return
			}

			// ログをストリームして logCh へ送信
			if err := streamContainerLogs(ctx, client.clientset, client.config.Namespace, pod.Name, containerName, logCh); err != nil {
				if ctx.Err() != nil {
					errCh <- ctx.Err()
				} else {
					errCh <- fmt.Errorf("[%s] ログストリームに失敗しました: %w", containerName, err)
				}
				return
			}
		}

		errCh <- nil
	}()

	return logCh, errCh
}

// waitForContainerRunning は指定したコンテナが Running または Terminated になるまで待機します。
func waitForContainerRunning(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, containerName string) error {
	for {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// InitContainer と通常コンテナ両方のステータスを確認
		allStatuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
		for _, containerStatus := range allStatuses {
			if containerStatus.Name != containerName {
				continue
			}
			if containerStatus.State.Running != nil || containerStatus.State.Terminated != nil {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// streamContainerLogs は指定したコンテナのログを全行読み取り、logCh へ送信します。
// コンテナのログが終端に達したら return します。
func streamContainerLogs(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, containerName string, logCh chan<- string) error {
	logOptions := &corev1.PodLogOptions{
		Container: containerName,
		Follow:    true,
	}
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, logOptions)
	stream, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		line := fmt.Sprintf("[%s] %s", containerName, scanner.Text())
		select {
		case logCh <- line:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Cancel は指定した jobID のビルドジョブを強制停止します。
func (client *Client) Cancel(ctx context.Context, jobID string) error {
	if err := deleteJob(ctx, client.clientset, client.config.Namespace, jobID); err != nil {
		return fmt.Errorf("ジョブの停止に失敗しました: %w", err)
	}
	return nil
}

// Wait はビルドが完了（Complete または Failed）するまでブロックします。
// ポーリング間隔は 10 秒です。
// タイムアウトは BuildConfig.Timeout で設定します。
func (client *Client) Wait(ctx context.Context, jobID string) (BuildStatus, error) {
	deadline := time.Now().Add(client.config.Timeout)

	for {
		if time.Now().After(deadline) {
			return StatusFailed, fmt.Errorf("タイムアウト: %s 経過しました", client.config.Timeout)
		}

		status, err := client.Status(ctx, jobID)
		if err != nil {
			return StatusFailed, err
		}

		switch status {
		case StatusComplete, StatusFailed:
			return status, nil
		}

		select {
		case <-ctx.Done():
			return StatusFailed, ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}
